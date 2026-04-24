package events_test

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/nalgeon/redka"

	"not_brimble/internal/events"
)

func openTestBus(t *testing.T) *events.Bus {
	t.Helper()
	sqlDB, err := sql.Open("sqlite3", ":memory:?_journal_mode=WAL")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	rdb, err := redka.OpenDB(sqlDB, sqlDB, nil)
	if err != nil {
		t.Fatalf("open redka: %v", err)
	}
	t.Cleanup(func() { rdb.Close() })
	return events.NewBus(rdb)
}

func TestPublishEncodesToQueue(t *testing.T) {
	ctx := context.Background()
	bus := openTestBus(t)

	evt := events.PipelineEvent{DeploymentID: "dep1", Stage: "queued"}
	if err := bus.Publish(ctx, events.QueueQueued, evt); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if evt.Encode() == "" {
		t.Error("encode returned empty string")
	}
}
