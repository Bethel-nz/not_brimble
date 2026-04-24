package db

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed migrations/001_init.sql
var initSQL string

type DB struct {
	sql     *sql.DB
	logHook func(deploymentID string)
}

// SetLogHook registers a callback invoked (in a goroutine) after every log
// line write. The API uses this to wake SSE subscribers; the worker wires
// it to an HTTP POST to the API. Nil is fine — hooks are optional.
func (d *DB) SetLogHook(f func(string)) {
	d.logHook = f
}

func Open(path string) (*DB, error) {
	dsn := fmt.Sprintf("file:%s?_journal_mode=WAL&_foreign_keys=on&_busy_timeout=5000", path)
	sqlDB, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if _, err := sqlDB.ExecContext(context.Background(), initSQL); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return &DB{sql: sqlDB}, nil
}

func (d *DB) Close() error {
	return d.sql.Close()
}

func (d *DB) SQL() *sql.DB {
	return d.sql
}
