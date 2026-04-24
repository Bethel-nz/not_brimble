CREATE TABLE IF NOT EXISTS deployments (
    id            TEXT PRIMARY KEY,
    name          TEXT NOT NULL DEFAULT '',
    source_type   TEXT NOT NULL,
    source_url    TEXT NOT NULL DEFAULT '',
    image_tag     TEXT NOT NULL DEFAULT '',
    container_id  TEXT NOT NULL DEFAULT '',
    subdomain     TEXT NOT NULL DEFAULT '',
    caddy_route_id TEXT NOT NULL DEFAULT '',
    status        TEXT NOT NULL DEFAULT 'pending',
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS log_lines (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    deployment_id TEXT NOT NULL,
    stream        TEXT NOT NULL DEFAULT 'stdout',
    line          TEXT NOT NULL,
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (deployment_id) REFERENCES deployments(id)
);

CREATE INDEX IF NOT EXISTS idx_log_lines_deployment ON log_lines(deployment_id, id);
