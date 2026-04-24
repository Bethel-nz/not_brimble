package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/nalgeon/redka"

	"not_brimble/internal/caddy"
	"not_brimble/internal/db"
	"not_brimble/internal/docker"
	"not_brimble/internal/events"
	"not_brimble/internal/pipeline"
)

func main() {
	dbPath := getenv("DB_PATH", "/data/app.db")
	caddyAdminURL := getenv("CADDY_ADMIN_URL", "http://caddy:2019")
	dockerNet := getenv("DOCKER_NETWORK", "not_brimble_net")
	buildDir := getenv("BUILD_DIR", "/tmp/builds")
	apiURL := getenv("API_URL", "http://api:8080")

	database, err := db.Open(dbPath)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer database.Close()

	sqlDB, err := sql.Open("sqlite3", fmt.Sprintf(
		"file:%s?_journal_mode=WAL&_busy_timeout=5000", dbPath))
	if err != nil {
		log.Fatalf("redka sql: %v", err)
	}
	rdb, err := redka.OpenDB(sqlDB, sqlDB, nil)
	if err != nil {
		log.Fatalf("redka: %v", err)
	}
	defer rdb.Close()

	dockerClient, err := docker.NewClient(dockerNet)
	if err != nil {
		log.Fatalf("docker: %v", err)
	}

	caddyClient := caddy.NewClient(caddyAdminURL)
	bus := events.NewBus(rdb)

	// Every log write fires this hook, which POSTs to the API's internal
	// notify endpoint. The API's in-process broker then wakes any SSE
	// subscribers. Failures are dropped on the floor — the SSE handler's
	// 30s fallback drain keeps clients live even if every notification
	// is lost, so the notify channel is best-effort by design.
	notifyClient := &http.Client{Timeout: 2 * time.Second}
	database.SetLogHook(func(depID string) {
		req, err := http.NewRequest(http.MethodPost, apiURL+"/internal/notify/logs/"+depID, nil)
		if err != nil {
			return
		}
		resp, err := notifyClient.Do(req)
		if err == nil {
			resp.Body.Close()
		}
	})

	// Sync routes on startup: ensures Caddy is in sync with DB state after a restart
	syncRoutes(database, caddyClient, dockerClient)

	buildH := &pipeline.BuildHandler{DB: database, Bus: bus, BuildDir: buildDir}
	runH := &pipeline.RunHandler{DB: database, Bus: bus, Docker: dockerClient}
	routeH := &pipeline.RouteHandler{DB: database, Bus: bus, Caddy: caddyClient}
	failH := &pipeline.FailureHandler{DB: database, Bus: bus, Docker: dockerClient}
	deleteH := &pipeline.DeleteHandler{DB: database, Bus: bus, Docker: dockerClient, Caddy: caddyClient}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Per-queue concurrency. Each queue is a pool of N competing consumers
	// — rdb.PopFront is atomic on SQLite, so two goroutines can race for
	// the same event without ever both handling it. This is how we get
	// "upload two deploys and watch them build in parallel" without
	// introducing a second process.
	//
	// Sizing reflects cost: builds hit disk and BuildKit hard so we keep
	// them low; routes are ~50ms HTTP round-trips so they can fan out
	// wide without hurting anyone. Override via env for load testing.
	spawn := func(n int, queue string, handle func(context.Context, events.PipelineEvent) error) {
		for i := 0; i < n; i++ {
			go listen(ctx, rdb, queue, handle)
		}
	}
	spawn(getenvInt("BUILD_CONCURRENCY", 2), events.QueueQueued, buildH.Handle)
	spawn(getenvInt("RUN_CONCURRENCY", 4), events.QueueBuilt, runH.Handle)
	spawn(getenvInt("ROUTE_CONCURRENCY", 8), events.QueueRunning, routeH.Handle)
	spawn(getenvInt("FAIL_CONCURRENCY", 2), events.QueueFailed, failH.Handle)
	spawn(getenvInt("DELETE_CONCURRENCY", 4), events.QueueDelete, deleteH.Handle)

	log.Println("worker ready")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	cancel()
}

// listen runs a single consumer loop per queue. Handlers are invoked
// serially; any panic is recovered and the event is requeued with
// exponential backoff up to 3 attempts, then routed to pipeline.failed.
func listen(
	ctx context.Context,
	rdb *redka.DB,
	queue string,
	handle func(context.Context, events.PipelineEvent) error,
) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		val, err := rdb.List().PopFront(queue)
		if err != nil || val.IsZero() {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
			continue
		}

		var evt events.PipelineEvent
		if err := json.Unmarshal([]byte(val.String()), &evt); err != nil {
			log.Printf("[%s] decode error: %v", queue, err)
			continue
		}

		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[%s] panic recovered for %s: %v", queue, evt.DeploymentID, r)
					requeueOrFail(rdb, queue, evt, fmt.Sprintf("panic: %v", r))
				}
			}()

			if err := handle(ctx, evt); err != nil {
				log.Printf("[%s] handler error for %s: %v", queue, evt.DeploymentID, err)
				requeueOrFail(rdb, queue, evt, err.Error())
			}
		}()
	}
}

func requeueOrFail(rdb *redka.DB, queue string, evt events.PipelineEvent, errMsg string) {
	if evt.Retries < 3 {
		evt.Retries++
		time.Sleep(time.Duration(evt.Retries) * 500 * time.Millisecond)
		rdb.List().PushBack(queue, evt.Encode())
		return
	}
	if queue != events.QueueFailed && queue != events.QueueDelete {
		failed := events.PipelineEvent{
			DeploymentID: evt.DeploymentID,
			Stage:        "failed",
			ErrorMsg:     fmt.Sprintf("max retries on %s: %s", queue, errMsg),
		}
		rdb.List().PushBack(events.QueueFailed, failed.Encode())
	}
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getenvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

// syncRoutes reconciles Caddy's in-memory route table with the deployments
// table on startup. For every deployment marked `running`, we verify the
// container is actually up (a Docker daemon restart may have left an orphan
// row) and then re-register its Caddy route idempotently.
//
// Errors are logged but non-fatal: a temporarily unreachable Caddy admin API
// shouldn't keep the worker from coming up and draining the queue.
func syncRoutes(database *db.DB, caddyClient *caddy.Client, dockerClient *docker.Client) {
	ctx := context.Background()

	// Compose's `depends_on` only waits for the Caddy *container* to exist,
	// not for its admin API to bind :2019. Probe a few times so a cold-start
	// race doesn't leave us with zero restored routes.
	if !waitForCaddy(ctx, caddyClient) {
		log.Println("sync: caddy admin API unreachable after 5s — skipping route restoration")
		return
	}

	deps, err := database.ListDeployments(ctx)
	if err != nil {
		log.Printf("sync: failed to list deployments: %v", err)
		return
	}

	restored, skipped := 0, 0
	for _, d := range deps {
		if d.Status != db.StatusRunning || d.Subdomain == "" {
			continue
		}

		// Don't re-register a route for a container that isn't actually up.
		// The route would just 502 until someone notices. If this happens,
		// the deployment stays marked `running` in the DB — we treat that
		// as "needs attention" rather than silently mutating state, which
		// a future health-check pass can surface.
		running, err := dockerClient.IsRunning(ctx, pipeline.ContainerName(d.Name))
		if err != nil {
			log.Printf("sync: inspect %s: %v", d.Name, err)
			skipped++
			continue
		}
		if !running {
			log.Printf("sync: container for %s not running — skipping route", d.Name)
			skipped++
			continue
		}

		// Delete-then-add is how we keep this idempotent: if Caddy already
		// has the route (partial restart, someone manually added it) we'd
		// otherwise 409 on the second add.
		caddyClient.DeleteRoute(ctx, d.Name)
		if err := caddyClient.AddRoute(ctx, caddy.Route{
			DeploymentID: d.Name,
			Host:         d.Subdomain,
			Upstream:     pipeline.Upstream(d.Name),
		}); err != nil {
			log.Printf("sync: restore route %s: %v", d.Name, err)
			continue
		}
		restored++
	}
	log.Printf("sync: complete — restored=%d skipped=%d total_running=%d", restored, skipped, restored+skipped)
}

// waitForCaddy probes the admin API until it responds or a budget expires.
// We lean on DeleteRoute for a cheap, idempotent "is it up?" check — a 404 on
// a made-up ID still means the admin API is serving requests.
func waitForCaddy(ctx context.Context, caddyClient *caddy.Client) bool {
	deadline := time.Now().Add(5 * time.Second)
	for attempt := 0; time.Now().Before(deadline); attempt++ {
		if err := caddyClient.DeleteRoute(ctx, "__sync_probe__"); err == nil {
			return true
		}
		time.Sleep(250 * time.Millisecond)
	}
	return false
}
