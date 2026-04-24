package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"not_brimble/internal/db"
)

// StreamLogs sends log lines for a deployment as an SSE stream.
//
// The handler does one cursor-based replay from SQLite up front (so
// reconnecting clients never lose history), then blocks on a broker wake
// channel for new writes. A 30s ceiling wake runs as a safety net in case
// a notification is ever dropped — in the steady state this is not polling,
// it's event-driven.
//
// The stream closes once the deployment reaches a terminal status and all
// lines up to that point have been delivered.
func (h *Handler) StreamLogs(c *gin.Context) {
	id := c.Param("id")

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.String(http.StatusInternalServerError, "streaming not supported")
		return
	}

	// Subscribe before the initial replay so any lines written between the
	// replay query and the subscribe still trigger a wake we'll observe.
	wake, unsub := h.broker.Subscribe(id)
	defer unsub()

	var cursor int64 = 0
	drain := func() {
		lines, err := h.db.LogLinesAfter(c.Request.Context(), id, cursor)
		if err != nil {
			return
		}
		for _, l := range lines {
			payload, _ := json.Marshal(map[string]any{
				"id":     l.ID,
				"stream": l.Stream,
				"line":   l.Line,
			})
			fmt.Fprintf(c.Writer, "data: %s\n\n", payload)
			cursor = l.ID
		}
		if len(lines) > 0 {
			flusher.Flush()
		}
	}

	drain()

	for {
		dep, err := h.db.GetDeployment(c.Request.Context(), id)
		if err == db.ErrNotFound {
			return
		}
		if err == nil && db.IsTerminal(dep.Status) {
			// One final pass to catch lines written between the previous
			// drain and the status flipping terminal.
			drain()
			return
		}

		select {
		case <-c.Request.Context().Done():
			return
		case <-wake:
			drain()
		case <-time.After(30 * time.Second):
			// Safety net for a dropped notification. The window is
			// deliberately wide — in the steady state this never fires
			// because the broker wakes us promptly.
			drain()
		}
	}
}

// StreamStatus emits an SSE event on every status change, plus the current
// state up front, then closes once terminal.
func (h *Handler) StreamStatus(c *gin.Context) {
	id := c.Param("id")

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.String(http.StatusInternalServerError, "streaming not supported")
		return
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	var lastStatus string

	for {
		dep, err := h.db.GetDeployment(c.Request.Context(), id)
		if err == db.ErrNotFound {
			return
		}
		if err == nil && dep.Status != lastStatus {
			payload, _ := json.Marshal(map[string]string{
				"status":    dep.Status,
				"subdomain": dep.Subdomain,
				"image_tag": dep.ImageTag,
			})
			fmt.Fprintf(c.Writer, "data: %s\n\n", payload)
			flusher.Flush()
			lastStatus = dep.Status
			if db.IsTerminal(dep.Status) {
				return
			}
		}

		select {
		case <-c.Request.Context().Done():
			return
		case <-ticker.C:
		}
	}
}
