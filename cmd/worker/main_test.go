package main

import (
	"context"
	"database/sql"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/nalgeon/redka"

	"not_brimble/internal/events"
)

func TestWorkerListenFailoverAndRetries(t *testing.T) {
	sqlDB, err := sql.Open("sqlite3", ":memory:?_journal_mode=WAL")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	rdb, err := redka.OpenDB(sqlDB, sqlDB, nil)
	if err != nil {
		t.Fatalf("open redka: %v", err)
	}
	defer rdb.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	queue := "test.queue"
	var callCount atomic.Int32

	handleFunc := func(c context.Context, evt events.PipelineEvent) error {
		n := callCount.Add(1)
		if n == 1 {
			panic("simulated panic")
		}
		if n <= 3 {
			return errors.New("simulated error")
		}
		return nil
	}

	// Start listener
	go listen(ctx, rdb, queue, handleFunc)

	// Publish an event
	evt := events.PipelineEvent{DeploymentID: "dep123"}
	_, err = rdb.List().PushBack(queue, evt.Encode())
	if err != nil {
		t.Fatalf("push: %v", err)
	}

	// Wait long enough for all retries including backoff (500+1000+1500ms = 3s + margin)
	time.Sleep(5 * time.Second)

	if n := int(callCount.Load()); n != 4 {
		t.Errorf("expected handle to be called 4 times due to panic and retries, got %d", n)
	}
}

func TestWorkerListenSpinUpFast(t *testing.T) {
	sqlDB, err := sql.Open("sqlite3", ":memory:?_journal_mode=WAL")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	rdb, err := redka.OpenDB(sqlDB, sqlDB, nil)
	if err != nil {
		t.Fatalf("open redka: %v", err)
	}
	defer rdb.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	queue := "test.queue.fast"
	processed := make(chan bool, 1)

	handleFunc := func(c context.Context, evt events.PipelineEvent) error {
		processed <- true
		return nil
	}

	// Push event before listener starts
	evt := events.PipelineEvent{DeploymentID: "dep_fast"}
	rdb.List().PushBack(queue, evt.Encode())

	// Start listener
	start := time.Now()
	go listen(ctx, rdb, queue, handleFunc)

	select {
	case <-processed:
		elapsed := time.Since(start)
		if elapsed > 200*time.Millisecond {
			t.Errorf("listener took too long to spin up and process: %v", elapsed)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("listener did not process event in time")
	}
}
