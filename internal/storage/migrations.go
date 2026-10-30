package storage

import (
	"database/sql"
	"fmt"
)

const schemaDDL = `
CREATE TABLE IF NOT EXISTS projects (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    slug TEXT NOT NULL UNIQUE,
    public_key TEXT NOT NULL UNIQUE,
    secret_key TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS issues (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    fingerprint TEXT NOT NULL,
    title TEXT NOT NULL,
    culprit TEXT NOT NULL,
    type TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'unresolved',
    first_seen_at DATETIME NOT NULL,
    last_seen_at DATETIME NOT NULL,
    event_count INTEGER NOT NULL DEFAULT 1,
    user_count INTEGER NOT NULL DEFAULT 1,
    last_release TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(project_id, fingerprint)
);

CREATE TABLE IF NOT EXISTS events (
    id TEXT PRIMARY KEY,
    issue_id INTEGER NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    timestamp DATETIME NOT NULL,
    environment TEXT,
    release TEXT,
    platform TEXT,
    message TEXT,
    user_json TEXT,
    tags_json TEXT,
    extra_json TEXT,
    breadcrumbs_json TEXT,
    stacktrace_json TEXT,
    raw_payload TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS issue_users (
    issue_id INTEGER NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    user_identifier TEXT NOT NULL,
    PRIMARY KEY (issue_id, user_identifier)
);

CREATE TABLE IF NOT EXISTS hourly_stats (
    project_id INTEGER NOT NULL,
    issue_id INTEGER NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    hour_bucket TEXT NOT NULL,
    count INTEGER NOT NULL DEFAULT 1,
    PRIMARY KEY (project_id, issue_id, hour_bucket)
);

CREATE TABLE IF NOT EXISTS release_artifacts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    release TEXT NOT NULL,
    filename TEXT NOT NULL,
    content_type TEXT NOT NULL,
    data BLOB NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(project_id, release, filename)
);

CREATE TABLE IF NOT EXISTS alert_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    channel TEXT NOT NULL,
    target_url TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT 1,
    notify_on_new BOOLEAN NOT NULL DEFAULT 1,
    notify_on_regression BOOLEAN NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS alert_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER NOT NULL,
    issue_id INTEGER NOT NULL,
    channel TEXT NOT NULL,
    sent_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    status TEXT NOT NULL,
    response TEXT
);

CREATE INDEX IF NOT EXISTS idx_issues_proj_status ON issues(project_id, status, last_seen_at DESC);
CREATE INDEX IF NOT EXISTS idx_issues_fingerprint ON issues(project_id, fingerprint);
CREATE INDEX IF NOT EXISTS idx_events_issue_time ON events(issue_id, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_events_proj_time ON events(project_id, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_hourly_lookup ON hourly_stats(project_id, issue_id, hour_bucket);
CREATE INDEX IF NOT EXISTS idx_artifacts_lookup ON release_artifacts(project_id, release, filename);
`

func migrateSchema(db *sql.DB) error {
	if _, err := db.Exec(schemaDDL); err != nil {
		return fmt.Errorf("failed executing schema migrations: %w", err)
	}
	return nil
}
