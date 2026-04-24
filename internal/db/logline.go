package db

import (
	"context"
	"time"
)

type LogLine struct {
	ID           int64     `json:"id"`
	DeploymentID string    `json:"deployment_id"`
	Stream       string    `json:"stream"`
	Line         string    `json:"line"`
	CreatedAt    time.Time `json:"created_at"`
}

// AppendLogLine writes a single log line for a deployment. Ordering is by
// autoincrement id — callers don't need to manage a sequence number. After
// a successful insert the registered log hook (if any) is fired in a
// goroutine so slow subscribers can't back-pressure the writer.
func (d *DB) AppendLogLine(ctx context.Context, deploymentID, stream, line string) error {
	_, err := d.sql.ExecContext(ctx, `
		INSERT INTO log_lines (deployment_id, stream, line, created_at)
		VALUES (?, ?, ?, ?)`,
		deploymentID, stream, line, time.Now().UTC(),
	)
	if err == nil && d.logHook != nil {
		go d.logHook(deploymentID)
	}
	return err
}

// LogLinesAfter returns log lines for a deployment with id > afterID, in order.
// Used by the SSE handler to poll incrementally per-subscriber.
func (d *DB) LogLinesAfter(ctx context.Context, deploymentID string, afterID int64) ([]LogLine, error) {
	rows, err := d.sql.QueryContext(ctx, `
		SELECT id, deployment_id, stream, line, created_at
		FROM log_lines
		WHERE deployment_id = ? AND id > ?
		ORDER BY id ASC`, deploymentID, afterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lines []LogLine
	for rows.Next() {
		var l LogLine
		var createdAt string
		if err := rows.Scan(&l.ID, &l.DeploymentID, &l.Stream, &l.Line, &createdAt); err != nil {
			return nil, err
		}
		l.CreatedAt = parseTime(createdAt)
		lines = append(lines, l)
	}
	return lines, rows.Err()
}
