package db_test

import (
	"context"
	"testing"
	"time"

	"not_brimble/internal/db"
)

func openTestDB(t *testing.T) *db.DB {
	t.Helper()
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func TestDeploymentCRUD(t *testing.T) {
	ctx := context.Background()
	d := openTestDB(t)

	dep := db.Deployment{
		ID:         "01HX1234",
		Name:       "test-dep",
		SourceType: "git",
		SourceURL:  "https://github.com/example/repo",
		Status:     "pending",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	if err := d.CreateDeployment(ctx, dep); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := d.GetDeployment(ctx, dep.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != "pending" {
		t.Errorf("got status %q, want %q", got.Status, "pending")
	}

	dep.Status = "building"
	dep.ImageTag = "01HX1234:latest"
	if err := d.UpdateDeployment(ctx, dep); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, _ = d.GetDeployment(ctx, dep.ID)
	if got.Status != "building" {
		t.Errorf("got status %q after update, want %q", got.Status, "building")
	}

	all, err := d.ListDeployments(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(all) != 1 {
		t.Errorf("got %d deployments, want 1", len(all))
	}
}

func TestGetDeploymentNotFound(t *testing.T) {
	d := openTestDB(t)
	_, err := d.GetDeployment(context.Background(), "nonexistent")
	if err != db.ErrNotFound {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestLogLines(t *testing.T) {
	ctx := context.Background()
	d := openTestDB(t)

	dep := db.Deployment{
		ID: "dep1", Name: "x", SourceType: "git",
		SourceURL: "u", Status: "building",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	d.CreateDeployment(ctx, dep)

	for _, line := range []string{"step 1", "step 2", "step 3"} {
		if err := d.AppendLogLine(ctx, "dep1", "stdout", line); err != nil {
			t.Fatalf("append log line: %v", err)
		}
	}

	lines, err := d.LogLinesAfter(ctx, "dep1", 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(lines) != 3 {
		t.Errorf("got %d lines, want 3", len(lines))
	}
	if lines[0].Line != "step 1" {
		t.Errorf("first line: got %q, want %q", lines[0].Line, "step 1")
	}

	// Cursor: passing the last ID should yield no new lines.
	after, err := d.LogLinesAfter(ctx, "dep1", lines[2].ID)
	if err != nil {
		t.Fatalf("list after: %v", err)
	}
	if len(after) != 0 {
		t.Errorf("expected 0 lines after cursor, got %d", len(after))
	}
}
