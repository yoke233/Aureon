package storesqlite

import (
	"database/sql"
	"fmt"
)

const schema = `
PRAGMA journal_mode=WAL;
PRAGMA busy_timeout=5000;
PRAGMA foreign_keys=ON;

CREATE TABLE IF NOT EXISTS projects (
    id           TEXT PRIMARY KEY,
    name         TEXT NOT NULL,
    repo_path    TEXT NOT NULL UNIQUE,
    github_owner TEXT,
    github_repo  TEXT,
    config_json  TEXT,
    created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at   DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS pipelines (
    id                TEXT PRIMARY KEY,
    project_id        TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name              TEXT NOT NULL,
    description       TEXT,
    template          TEXT NOT NULL,
    status            TEXT NOT NULL DEFAULT 'created',
    current_stage     TEXT,
    stages_json       TEXT NOT NULL,
    artifacts_json    TEXT DEFAULT '{}',
    config_json       TEXT DEFAULT '{}',
    issue_number      INTEGER,
    pr_number         INTEGER,
    branch_name       TEXT,
    worktree_path     TEXT,
    error_message     TEXT,
    max_total_retries INTEGER DEFAULT 5,
    total_retries     INTEGER DEFAULT 0,
    run_count         INTEGER DEFAULT 0,
    last_error_type   TEXT,
    queued_at         DATETIME,
    last_heartbeat_at DATETIME,
    started_at        DATETIME,
    finished_at       DATETIME,
    created_at        DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at        DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_pipelines_project ON pipelines(project_id);
CREATE INDEX IF NOT EXISTS idx_pipelines_status ON pipelines(status);
CREATE INDEX IF NOT EXISTS idx_pipelines_status_queued_at ON pipelines(status, queued_at, created_at);
CREATE INDEX IF NOT EXISTS idx_pipelines_project_status ON pipelines(project_id, status);

CREATE TABLE IF NOT EXISTS checkpoints (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    pipeline_id    TEXT NOT NULL REFERENCES pipelines(id) ON DELETE CASCADE,
    stage          TEXT NOT NULL,
    status         TEXT NOT NULL,
    agent_used     TEXT,
    artifacts_json TEXT DEFAULT '{}',
    tokens_used    INTEGER DEFAULT 0,
    retry_count    INTEGER DEFAULT 0,
    error_message  TEXT,
    started_at     DATETIME NOT NULL,
    finished_at    DATETIME,
    created_at     DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_checkpoints_pipeline ON checkpoints(pipeline_id);

CREATE TABLE IF NOT EXISTS logs (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    pipeline_id TEXT NOT NULL REFERENCES pipelines(id) ON DELETE CASCADE,
    stage       TEXT NOT NULL,
    type        TEXT NOT NULL,
    agent       TEXT,
    content     TEXT NOT NULL,
    timestamp   DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_logs_pipeline_stage ON logs(pipeline_id, stage);
CREATE INDEX IF NOT EXISTS idx_logs_id ON logs(id);

CREATE TABLE IF NOT EXISTS human_actions (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    pipeline_id TEXT NOT NULL REFERENCES pipelines(id) ON DELETE CASCADE,
    stage       TEXT NOT NULL,
    action      TEXT NOT NULL,
    message     TEXT,
    source      TEXT NOT NULL,
    user_id     TEXT,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_human_actions_pipeline ON human_actions(pipeline_id);
`

func applyMigrations(db *sql.DB) error {
	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("exec schema: %w", err)
	}

	// Keep older local sqlite files backward-compatible when new columns are introduced.
	columns := map[string]string{
		"run_count":         "run_count INTEGER DEFAULT 0",
		"last_error_type":   "last_error_type TEXT",
		"queued_at":         "queued_at DATETIME",
		"last_heartbeat_at": "last_heartbeat_at DATETIME",
	}
	for column, ddl := range columns {
		exists, err := hasColumn(db, "pipelines", column)
		if err != nil {
			return err
		}
		if exists {
			continue
		}
		if _, err := db.Exec("ALTER TABLE pipelines ADD COLUMN " + ddl); err != nil {
			return fmt.Errorf("add pipelines.%s: %w", column, err)
		}
	}
	return nil
}

func hasColumn(db *sql.DB, table, column string) (bool, error) {
	query := fmt.Sprintf("PRAGMA table_info(%s)", table)
	rows, err := db.Query(query)
	if err != nil {
		return false, fmt.Errorf("pragma table_info(%s): %w", table, err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid       int
			name      string
			colType   string
			notnull   int
			dfltValue sql.NullString
			pk        int
		)
		if err := rows.Scan(&cid, &name, &colType, &notnull, &dfltValue, &pk); err != nil {
			return false, fmt.Errorf("scan table_info(%s): %w", table, err)
		}
		if name == column {
			return true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("iterate table_info(%s): %w", table, err)
	}
	return false, nil
}
