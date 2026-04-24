package pipeline

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCloneRepoFailsFastOnFatal(t *testing.T) {
	ctx := context.Background()
	logs := []string{}
	writeLog := func(stream, line string) {
		logs = append(logs, line)
	}

	// Non-existent local path — git exits 128 (its generic "fatal" exit,
	// shared by auth / bad URL / missing repo). cloneRepo's isAuthErr
	// treats 128 as terminal and bails after the first attempt with a
	// user-actionable error, rather than burning 3×2min on something
	// a retry can't fix. We verify exactly that shape.
	err := cloneRepo(ctx, "/path/does/not/exist/ever", filepath.Join(t.TempDir(), "dst"), writeLog)
	if err == nil {
		t.Fatal("expected clone to fail")
	}

	logStr := strings.Join(logs, "\n")
	if !strings.Contains(logStr, "attempt 1/3") {
		t.Errorf("missing attempt 1 log, got: %s", logStr)
	}
	if strings.Contains(logStr, "attempt 2/3") || strings.Contains(logStr, "attempt 3/3") {
		t.Errorf("expected fast-fail on exit 128, got extra attempts: %s", logStr)
	}
	if !strings.Contains(err.Error(), "authentication required") {
		t.Errorf("expected auth/bad-URL error, got: %v", err)
	}
}

func TestExtractTarGzBroken(t *testing.T) {
	// Create a dummy broken tar.gz file
	tmpDir := t.TempDir()
	brokenFile := filepath.Join(tmpDir, "broken.tar.gz")
	if err := os.WriteFile(brokenFile, []byte("not a gzip file at all"), 0644); err != nil {
		t.Fatal(err)
	}

	dstDir := filepath.Join(tmpDir, "dst")
	err := extractTarGz(brokenFile, dstDir)
	if err == nil {
		t.Fatal("expected extraction of broken file to fail")
	}
	if !strings.Contains(err.Error(), "gzip") {
		t.Errorf("expected gzip error, got: %v", err)
	}
}

func TestCloneRepoSuccess(t *testing.T) {
	ctx := context.Background()

	// Point HOME at a throwaway dir and disable system config so any
	// insteadOf / url rewrites in the tester's gitconfig don't mangle
	// the local path URL we're about to clone from. Without this, a
	// user-level rule like `url.https://.insteadOf = /` would turn our
	// plain path into `https:///path/…` and the clone would 502 at us.
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")

	// Create a dummy local git repository to clone from
	srcDir := t.TempDir()

	cmd := exec.Command("git", "init")
	cmd.Dir = srcDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}

	if err := os.WriteFile(filepath.Join(srcDir, "README.md"), []byte("hello world"), 0644); err != nil {
		t.Fatal(err)
	}

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = srcDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git add failed: %v", err)
	}

	cmd = exec.Command("git", "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "initial commit")
	cmd.Dir = srcDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}

	dstDir := filepath.Join(t.TempDir(), "dst")

	logs := []string{}
	writeLog := func(stream, line string) {
		logs = append(logs, line)
	}

	// Clone the local repository. We pass a plain path rather than a
	// `file://` URL — git recognises absolute paths natively and this
	// sidesteps any insteadOf URL rewrites in the host's gitconfig that
	// could redirect file:// through a different protocol.
	err := cloneRepo(ctx, srcDir, dstDir, writeLog)
	if err != nil {
		t.Fatalf("expected clone to succeed, got error: %v", err)
	}

	// Verify that the file was cloned
	content, err := os.ReadFile(filepath.Join(dstDir, "README.md"))
	if err != nil {
		t.Fatalf("failed to read cloned file: %v", err)
	}
	if string(content) != "hello world" {
		t.Errorf("expected file content 'hello world', got %q", string(content))
	}
}
