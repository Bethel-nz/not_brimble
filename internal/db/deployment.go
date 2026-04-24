package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type Deployment struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	SourceType   string    `json:"source_type"`
	SourceURL    string    `json:"source_url"`
	ImageTag     string    `json:"image_tag"`
	ContainerID  string    `json:"container_id"`
	Subdomain    string    `json:"subdomain"`
	CaddyRouteID string    `json:"caddy_route_id"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

var ErrNotFound = errors.New("not found")

// Deployment status vocabulary. The linear happy path is:
//   pending → building → built → deploying → running
// Any stage can transition to failed; DELETE transitions to stopped.
const (
	StatusPending   = "pending"
	StatusBuilding  = "building"
	StatusBuilt     = "built"
	StatusDeploying = "deploying"
	StatusRunning   = "running"
	StatusFailed    = "failed"
	StatusStopped   = "stopped"
)

// IsTerminal reports whether a deployment status will not change again on its
// own. SSE streams close when they see a terminal status.
func IsTerminal(status string) bool {
	return status == StatusRunning || status == StatusFailed || status == StatusStopped
}

func (d *DB) CreateDeployment(ctx context.Context, dep Deployment) error {
	_, err := d.sql.ExecContext(ctx, `
		INSERT INTO deployments (id, name, source_type, source_url, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		dep.ID, dep.Name, dep.SourceType, dep.SourceURL, dep.Status,
		dep.CreatedAt.UTC(), dep.UpdatedAt.UTC(),
	)
	return err
}

func (d *DB) GetDeployment(ctx context.Context, id string) (Deployment, error) {
	row := d.sql.QueryRowContext(ctx, `
		SELECT id, name, source_type, source_url, image_tag, container_id,
		       subdomain, caddy_route_id, status, created_at, updated_at
		FROM deployments WHERE id = ?`, id)
	return scanDeployment(row)
}

func (d *DB) ListDeployments(ctx context.Context) ([]Deployment, error) {
	rows, err := d.sql.QueryContext(ctx, `
		SELECT id, name, source_type, source_url, image_tag, container_id,
		       subdomain, caddy_route_id, status, created_at, updated_at
		FROM deployments ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deps []Deployment
	for rows.Next() {
		dep, err := scanDeployment(rows)
		if err != nil {
			return nil, err
		}
		deps = append(deps, dep)
	}
	return deps, rows.Err()
}

// ListRunningBySourceURL returns every deployment with status=running that
// shares the given source_url. Used at the moment a new deployment goes live
// so the RouteHandler can retire its siblings and keep exactly one container
// per service.
func (d *DB) ListRunningBySourceURL(ctx context.Context, sourceURL string) ([]Deployment, error) {
	rows, err := d.sql.QueryContext(ctx, `
		SELECT id, name, source_type, source_url, image_tag, container_id,
		       subdomain, caddy_route_id, status, created_at, updated_at
		FROM deployments WHERE status = ? AND source_url = ?`,
		StatusRunning, sourceURL)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deps []Deployment
	for rows.Next() {
		dep, err := scanDeployment(rows)
		if err != nil {
			return nil, err
		}
		deps = append(deps, dep)
	}
	return deps, rows.Err()
}

func (d *DB) UpdateDeployment(ctx context.Context, dep Deployment) error {
	_, err := d.sql.ExecContext(ctx, `
		UPDATE deployments SET
			image_tag = ?, container_id = ?, subdomain = ?,
			caddy_route_id = ?, status = ?, updated_at = ?
		WHERE id = ?`,
		dep.ImageTag, dep.ContainerID, dep.Subdomain,
		dep.CaddyRouteID, dep.Status, time.Now().UTC(),
		dep.ID,
	)
	return err
}

func parseTime(s string) time.Time {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05", "2006-01-02T15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

type scanner interface {
	Scan(dest ...any) error
}

func scanDeployment(s scanner) (Deployment, error) {
	var dep Deployment
	var createdAt, updatedAt string
	err := s.Scan(
		&dep.ID, &dep.Name, &dep.SourceType, &dep.SourceURL,
		&dep.ImageTag, &dep.ContainerID, &dep.Subdomain,
		&dep.CaddyRouteID, &dep.Status, &createdAt, &updatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return dep, ErrNotFound
	}
	if err != nil {
		return dep, fmt.Errorf("scan deployment: %w", err)
	}
	dep.CreatedAt = parseTime(createdAt)
	dep.UpdatedAt = parseTime(updatedAt)
	return dep, nil
}
