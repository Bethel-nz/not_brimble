package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"
	"github.com/nalgeon/redka"

	"not_brimble/internal/db"
	"not_brimble/internal/events"
	"not_brimble/internal/notify"
)

func main() {
	dbPath := getenv("DB_PATH", "/data/app.db")
	port := getenv("PORT", "8080")

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

	bus := events.NewBus(rdb)
	uploadDir := getenv("UPLOAD_DIR", "/tmp/uploads")
	os.MkdirAll(uploadDir, 0755)

	broker := notify.New()
	// The API writes rollback/redeploy log lines directly; wake subscribers
	// for those too, not just for worker-written lines.
	database.SetLogHook(broker.Notify)

	h := &Handler{db: database, bus: bus, uploadDir: uploadDir, broker: broker}

	r := gin.Default()

	// @title Not Brimble API
	// @version 1.0
	// @description A one-page deployment pipeline API mirroring Brimble PaaS logic.
	// @host localhost:8080
	// @BasePath /

	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})

	r.GET("/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

	// Internal wake endpoint: the worker posts here after every log write so
	// SSE subscribers get fan-out without polling SQLite. Not authenticated —
	// scoped to the private compose network only. A real deployment would
	// either sign these calls or keep the broker out-of-band entirely.
	r.POST("/internal/notify/logs/:id", func(c *gin.Context) {
		broker.Notify(c.Param("id"))
		c.Status(http.StatusNoContent)
	})

	// Scalar API docs. We serve the spec ourselves and load Scalar's
	// standalone bundle from jsDelivr — no Go wrapper to vendor, and the
	// bundle URL is content-addressable so it pins cleanly.
	r.GET("/docs/openapi.json", func(c *gin.Context) {
		c.File("./docs/swagger.json")
	})
	r.GET("/docs", func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(scalarHTML))
	})

	r.POST("/deployments", h.CreateDeployment)
	r.GET("/deployments", h.ListDeployments)
	r.GET("/deployments/:id", h.GetDeployment)
	r.POST("/deployments/:id/redeploy", h.RedeployDeployment)
	r.DELETE("/deployments/:id", h.DeleteDeployment)
	r.GET("/deployments/:id/logs", h.StreamLogs)
	r.GET("/deployments/:id/status", h.StreamStatus)

	srv := &http.Server{Addr: ":" + port, Handler: r}

	go func() {
		log.Printf("api listening on :%s", port)
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

const scalarHTML = `<!doctype html>
<html>
  <head>
    <title>Not Brimble API Reference</title>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
  </head>
  <body>
    <script id="api-reference" data-url="docs/openapi.json"></script>
    <script>
      var configuration = { theme: 'purple' }
      document.getElementById('api-reference').dataset.configuration = JSON.stringify(configuration)
    </script>
    <script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script>
  </body>
</html>`
