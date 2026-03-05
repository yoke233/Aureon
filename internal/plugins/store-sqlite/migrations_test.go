package storesqlite

import (
	"database/sql"
	"testing"
)

func TestMigration_V2Baseline_CreatesIssueRunSchema(t *testing.T) {
	db := openSQLite(t)
	defer db.Close()

	if err := applyMigrations(db); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	tables := []string{
		"projects",
		"runs",
		"checkpoints",
		"human_actions",
		"chat_sessions",
		"chat_run_events",
		"issues",
		"issue_attachments",
		"issue_changes",
		"review_records",
	}
	for _, table := range tables {
		assertTableExists(t, db, table)
	}

	assertTableNotExists(t, db, "task_plans")
	assertTableNotExists(t, db, "task_items")
	assertTableNotExists(t, db, "migration_flags")

	assertColumnExists(t, db, "runs", "issue_id")
	assertColumnExists(t, db, "runs", "run_count")
	assertColumnExists(t, db, "runs", "queued_at")
	assertColumnExists(t, db, "runs", "last_heartbeat_at")
	assertColumnNotExists(t, db, "runs", "task_item_id")

	assertColumnExists(t, db, "issues", "auto_merge")
	assertColumnExists(t, db, "issues", "merge_retries")
	assertColumnExists(t, db, "review_records", "issue_id")
	assertColumnExists(t, db, "review_records", "summary")
	assertColumnExists(t, db, "review_records", "raw_output")
	assertColumnNotExists(t, db, "review_records", "plan_id")
}

func TestMigration_V2Baseline_PersistsIssueIDLinks(t *testing.T) {
	db := openSQLite(t)
	defer db.Close()

	if err := applyMigrations(db); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	if _, err := db.Exec(`INSERT INTO projects (id, name, repo_path) VALUES ('proj-v2-1', 'proj', '/tmp/proj-v2-1')`); err != nil {
		t.Fatalf("insert project: %v", err)
	}
	if _, err := db.Exec(`
INSERT INTO runs (id, project_id, name, template, stages_json, issue_id)
VALUES ('pipe-v2-1', 'proj-v2-1', 'pipe', 'standard', '[]', 'issue-v2-1')
`); err != nil {
		t.Fatalf("insert Run: %v", err)
	}
	if _, err := db.Exec(`
INSERT INTO review_records (issue_id, round, reviewer, verdict, summary, raw_output, issues, fixes)
VALUES ('issue-v2-1', 1, 'team-leader', 'approved', 'ok', '{}', '[]', '[]')
`); err != nil {
		t.Fatalf("insert review_record: %v", err)
	}

	var runIssueID string
	if err := db.QueryRow(`SELECT COALESCE(issue_id, '') FROM runs WHERE id='pipe-v2-1'`).Scan(&runIssueID); err != nil {
		t.Fatalf("query runs.issue_id: %v", err)
	}
	if runIssueID != "issue-v2-1" {
		t.Fatalf("expected runs.issue_id=issue-v2-1, got %q", runIssueID)
	}

	var reviewIssueID string
	if err := db.QueryRow(`SELECT issue_id FROM review_records WHERE reviewer='team-leader'`).Scan(&reviewIssueID); err != nil {
		t.Fatalf("query review_records.issue_id: %v", err)
	}
	if reviewIssueID != "issue-v2-1" {
		t.Fatalf("expected review_records.issue_id=issue-v2-1, got %q", reviewIssueID)
	}
}

func TestMigration_V2Baseline_CreatesIndexes(t *testing.T) {
	db := openSQLite(t)
	defer db.Close()

	if err := applyMigrations(db); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	indexes := []string{
		"idx_runs_project",
		"idx_runs_status",
		"idx_runs_status_queued_at",
		"idx_runs_project_status",
		"idx_issues_project",
		"idx_issues_project_status",
		"idx_issues_session",
		"idx_issues_run",
		"idx_issue_attachments_issue",
		"idx_issue_changes_issue",
		"idx_review_records_issue",
	}
	for _, index := range indexes {
		assertIndexExists(t, db, index)
	}
}

func TestMigration_AddsIssueMergeRetriesFromV3(t *testing.T) {
	db := openSQLite(t)
	defer db.Close()

	if _, err := db.Exec(`
CREATE TABLE IF NOT EXISTS issues (
	id                TEXT PRIMARY KEY,
	project_id        TEXT NOT NULL,
	session_id        TEXT,
	title             TEXT NOT NULL,
	body              TEXT NOT NULL DEFAULT '',
	labels            TEXT NOT NULL DEFAULT '[]',
	milestone_id      TEXT NOT NULL DEFAULT '',
	attachments       TEXT NOT NULL DEFAULT '[]',
	depends_on        TEXT NOT NULL DEFAULT '[]',
	blocks            TEXT NOT NULL DEFAULT '[]',
	priority          INTEGER NOT NULL DEFAULT 0,
	template          TEXT NOT NULL DEFAULT 'standard',
	auto_merge        INTEGER NOT NULL DEFAULT 1,
	state             TEXT NOT NULL DEFAULT 'open',
	status            TEXT NOT NULL DEFAULT 'draft',
	run_id            TEXT,
	version           INTEGER NOT NULL DEFAULT 1,
	superseded_by     TEXT NOT NULL DEFAULT '',
	external_id       TEXT,
	fail_policy       TEXT NOT NULL DEFAULT 'block',
	parent_id         TEXT NOT NULL DEFAULT '',
	created_at        DATETIME DEFAULT CURRENT_TIMESTAMP,
	updated_at        DATETIME DEFAULT CURRENT_TIMESTAMP,
	closed_at         DATETIME
)`); err != nil {
		t.Fatalf("create legacy issues table: %v", err)
	}
	if _, err := db.Exec(`PRAGMA user_version = 3`); err != nil {
		t.Fatalf("set user_version=3: %v", err)
	}

	if err := applyMigrations(db); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	assertColumnExists(t, db, "issues", "merge_retries")

	var version int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		t.Fatalf("read user_version: %v", err)
	}
	if version != schemaVersion {
		t.Fatalf("user_version=%d, want %d", version, schemaVersion)
	}
}

func assertColumnExists(t *testing.T, db *sql.DB, table, column string) {
	t.Helper()

	ok, err := hasColumn(db, table, column)
	if err != nil {
		t.Fatalf("check column %s.%s: %v", table, column, err)
	}
	if !ok {
		t.Fatalf("expected column %s.%s to exist", table, column)
	}
}

func assertColumnNotExists(t *testing.T, db *sql.DB, table, column string) {
	t.Helper()

	ok, err := hasColumn(db, table, column)
	if err != nil {
		t.Fatalf("check column %s.%s: %v", table, column, err)
	}
	if ok {
		t.Fatalf("expected column %s.%s to be absent", table, column)
	}
}
