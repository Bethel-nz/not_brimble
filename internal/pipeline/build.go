package pipeline

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"not_brimble/internal/db"
	"not_brimble/internal/events"
)

type BuildHandler struct {
	DB       *db.DB
	Bus      *events.Bus
	BuildDir string // base dir for build workspaces, e.g. /tmp/builds
}

func (h *BuildHandler) Handle(ctx context.Context, evt events.PipelineEvent) error {
	id := evt.DeploymentID

	dep, err := h.DB.GetDeployment(ctx, id)
	if err != nil {
		return fmt.Errorf("get deployment: %w", err)
	}

	dep.Status = db.StatusBuilding
	h.DB.UpdateDeployment(ctx, dep)

	writeLog := func(stream, line string) {
		h.DB.AppendLogLine(ctx, id, stream, line)
	}

	workDir := filepath.Join(h.BuildDir, id)
	if err := os.MkdirAll(workDir, 0755); err != nil {
		h.failDeployment(ctx, dep, fmt.Sprintf("mkdir: %v", err))
		return nil
	}
	defer os.RemoveAll(workDir)

	writeLog("stdout", fmt.Sprintf("[build] preparing source for %s", id))

	switch dep.SourceType {
	case "git":
		if err := cloneRepo(ctx, dep.SourceURL, workDir, writeLog); err != nil {
			h.failDeployment(ctx, dep, fmt.Sprintf("git clone: %v", err))
			return nil
		}
	case "upload":
		if err := extractTarGz(dep.SourceURL, workDir); err != nil {
			h.failDeployment(ctx, dep, fmt.Sprintf("extract: %v", err))
			return nil
		}
	default:
		h.failDeployment(ctx, dep, "unknown source_type: "+dep.SourceType)
		return nil
	}

	// dep.Name is already the DNS/Docker-safe slug (e.g. "my-app-a1b2c3").
	// Use the deployment ID as the tag so each build is a uniquely addressable image.
	imageTag := dep.Name + ":" + dep.ID
	writeLog("stdout", fmt.Sprintf("[build] running railpack build → %s", imageTag))

	if err := runRailpack(ctx, workDir, imageTag, writeLog); err != nil {
		h.failDeployment(ctx, dep, fmt.Sprintf("railpack: %v", err))
		return nil
	}

	dep.ImageTag = imageTag
	dep.Status = db.StatusBuilt
	h.DB.UpdateDeployment(ctx, dep)

	writeLog("stdout", fmt.Sprintf("[build] image ready: %s", imageTag))

	return h.Bus.Publish(ctx, events.QueueBuilt, events.PipelineEvent{
		DeploymentID: id,
		Stage:        "built",
		ImageTag:     imageTag,
	})
}

func (h *BuildHandler) failDeployment(ctx context.Context, dep db.Deployment, msg string) {
	dep.Status = db.StatusFailed
	h.DB.UpdateDeployment(ctx, dep)
	h.Bus.Publish(ctx, events.QueueFailed, events.PipelineEvent{
		DeploymentID: dep.ID,
		Stage:        "failed",
		ErrorMsg:     msg,
	})
}

func cloneRepo(ctx context.Context, url, dst string, writeLog func(string, string)) error {
	// Ensure the URL has a protocol; git clone requires it for HTTPS/SSH
	// targets. Users often paste "github.com/foo/bar".
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "git@") && !filepath.IsAbs(url) {
		url = "https://" + url
	}

	var err error
	for attempt := 1; attempt <= 3; attempt++ {
		if attempt > 1 {
			os.RemoveAll(dst)
		}

		// Per-attempt timeout so an auth hang eventually gives up and retries
		// rather than blocking the pipeline behind a stuck git process.
		attemptCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)

		cmd := exec.CommandContext(attemptCtx, "git", "clone", "--depth=1", "--progress", url, dst)
		// GitHub returns 401 for both private and non-existent repos; without
		// these vars git interprets 401 as "prompt the user" and produces the
		// confusing "No such device or address" error on a headless box.
		// Forcing non-interactive mode turns that into a clean, retryable
		// failure with a readable log line.
		cmd.Env = append(os.Environ(),
			"GIT_TERMINAL_PROMPT=0",
			"GIT_ASKPASS=/bin/true",
			"GCM_INTERACTIVE=never",
		)

		stdout, _ := cmd.StdoutPipe()
		stderr, _ := cmd.StderrPipe()

		if startErr := cmd.Start(); startErr != nil {
			cancel()
			err = startErr
			writeLog("stderr", fmt.Sprintf("git clone start (attempt %d/3): %v", attempt, startErr))
			continue
		}

		// Drain both pipes concurrently so users see progress in real time
		// instead of a wall of text at the end (or nothing, on a hang).
		go drainByCR(stdout, "stdout", writeLog)
		go drainByCR(stderr, "stderr", writeLog)

		waitErr := cmd.Wait()
		cancel()
		if waitErr == nil {
			return nil
		}
		err = waitErr
		writeLog("stderr", fmt.Sprintf("git clone failed (attempt %d/3): %v", attempt, waitErr))

		// Auth failures won't succeed on retry — bail immediately with a
		// message users can act on. Transient network errors still get the
		// full 3-try backoff.
		if isAuthErr(waitErr) {
			writeLog("stderr", "hint: repo is private, renamed, or does not exist — GitHub returns 401 for all three")
			return fmt.Errorf("git clone: authentication required (repo private or not found): %s", url)
		}
	}
	return err
}

// isAuthErr reports whether the git exit code matches the "authentication
// required" path. Git exits 128 on most fatal errors including auth and
// bad URLs — we treat 128 as terminal here because a transient network
// issue is rare enough that the cost of three failed retries on a bad
// repo URL (six minutes) outweighs the occasional missed retry.
func isAuthErr(err error) bool {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode() == 128
	}
	return false
}

func runRailpack(ctx context.Context, dir, imageName string, writeLog func(string, string)) error {
	cmd := exec.CommandContext(ctx, "railpack", "build", "--name", imageName, dir)

	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start railpack: %w", err)
	}

	go drainByCR(stdout, "stdout", writeLog)
	go drainByCR(stderr, "stderr", writeLog)

	return cmd.Wait()
}

// drainByCR scans a reader into log lines, splitting on \r, \n, or \r\n so
// that tools which use \r to rewrite the current terminal line (git clone
// progress, docker pulls, railpack's spinner) don't land as one giant
// concatenated blob in the log store.
//
// Progress streams get coalesced: if a new token arrives within the
// progressWindow of the previous write, we hold it as pending instead of
// writing it. The next token that either (a) arrives after the window
// elapses or (b) is the final token at EOF gets flushed. This keeps the
// stream feeling live (≈3 updates/sec visible) while turning what would
// be 70+ "Counting objects: N%" rows into a handful. Non-progress log
// lines aren't affected because they don't burst at that rate.
func drainByCR(r io.Reader, stream string, writeLog func(string, string)) {
	sc := bufio.NewScanner(r)
	// Progress-heavy tools can spit very long "single lines" between a pair
	// of \r chars; bump the scanner buffer well past the 64KB default so
	// we don't trip bufio.ErrTooLong on them.
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	sc.Split(splitCRLF)

	const progressWindow = 300 * time.Millisecond
	var lastFlush time.Time
	var pending string

	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			continue
		}
		if time.Since(lastFlush) >= progressWindow {
			writeLog(stream, line)
			lastFlush = time.Now()
			pending = ""
		} else {
			pending = line
		}
	}
	// Always flush a trailing pending line — usually the final "done" /
	// "100%" message that would otherwise be eaten by the window.
	if pending != "" {
		writeLog(stream, pending)
	}
}

// splitCRLF treats \n, \r, and \r\n as line terminators. This lets us turn
// terminal "overwrite this line" updates (\r) into independent log entries.
func splitCRLF(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	for i := 0; i < len(data); i++ {
		switch data[i] {
		case '\n':
			return i + 1, data[:i], nil
		case '\r':
			if i+1 < len(data) && data[i+1] == '\n' {
				return i + 2, data[:i], nil
			}
			if i+1 < len(data) {
				return i + 1, data[:i], nil
			}
			// Trailing \r — need more bytes to decide if \n follows, unless
			// the stream is done.
			if !atEOF {
				return 0, nil, nil
			}
			return i + 1, data[:i], nil
		}
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}

func extractTarGz(src, dst string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		// Strip leading path component (e.g. "project/src" → "src")
		parts := strings.SplitN(filepath.ToSlash(hdr.Name), "/", 2)
		name := hdr.Name
		if len(parts) == 2 {
			name = parts[1]
		}
		if name == "" {
			continue
		}
		target := filepath.Join(dst, filepath.FromSlash(name))
		// Zip slip protection: reject paths that escape the destination directory.
		if !strings.HasPrefix(filepath.Clean(target)+string(os.PathSeparator), filepath.Clean(dst)+string(os.PathSeparator)) {
			return fmt.Errorf("zip slip: illegal path in archive: %s", hdr.Name)
		}
		if hdr.Typeflag == tar.TypeDir {
			os.MkdirAll(target, 0755)
		} else {
			os.MkdirAll(filepath.Dir(target), 0755)
			out, err := os.Create(target)
			if err != nil {
				return err
			}
			const maxFileSize = 500 << 20 // 500 MB per file
			if _, err := io.Copy(out, io.LimitReader(tr, maxFileSize)); err != nil {
				out.Close()
				return err
			}
			out.Close()
		}
	}
	return nil
}
