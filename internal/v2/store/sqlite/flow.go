package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/yoke233/ai-workflow/internal/v2/core"
)

func (s *Store) CreateFlow(ctx context.Context, f *core.Flow) (int64, error) {
	meta, err := marshalJSON(f.Metadata)
	if err != nil {
		return 0, fmt.Errorf("marshal metadata: %w", err)
	}
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO flows (name, status, parent_step_id, metadata, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		f.Name, f.Status, f.ParentStepID, meta, now, now,
	)
	if err != nil {
		return 0, fmt.Errorf("insert flow: %w", err)
	}
	id, _ := res.LastInsertId()
	f.ID = id
	f.CreatedAt = now
	f.UpdatedAt = now
	return id, nil
}

func (s *Store) GetFlow(ctx context.Context, id int64) (*core.Flow, error) {
	f := &core.Flow{}
	var meta sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, status, parent_step_id, metadata, created_at, updated_at
		 FROM flows WHERE id = ?`, id,
	).Scan(&f.ID, &f.Name, &f.Status, &f.ParentStepID, &meta, &f.CreatedAt, &f.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, core.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get flow %d: %w", id, err)
	}
	if meta.Valid {
		_ = json.Unmarshal([]byte(meta.String), &f.Metadata)
	}
	return f, nil
}

func (s *Store) ListFlows(ctx context.Context, filter core.FlowFilter) ([]*core.Flow, error) {
	query := `SELECT id, name, status, parent_step_id, metadata, created_at, updated_at FROM flows`
	var args []any
	if filter.Status != nil {
		query += ` WHERE status = ?`
		args = append(args, *filter.Status)
	}
	query += ` ORDER BY id DESC`
	if filter.Limit > 0 {
		query += fmt.Sprintf(` LIMIT %d`, filter.Limit)
	}
	if filter.Offset > 0 {
		query += fmt.Sprintf(` OFFSET %d`, filter.Offset)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list flows: %w", err)
	}
	defer rows.Close()

	var flows []*core.Flow
	for rows.Next() {
		f := &core.Flow{}
		var meta sql.NullString
		if err := rows.Scan(&f.ID, &f.Name, &f.Status, &f.ParentStepID, &meta, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan flow: %w", err)
		}
		if meta.Valid {
			_ = json.Unmarshal([]byte(meta.String), &f.Metadata)
		}
		flows = append(flows, f)
	}
	return flows, rows.Err()
}

func (s *Store) UpdateFlowStatus(ctx context.Context, id int64, status core.FlowStatus) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE flows SET status = ?, updated_at = ? WHERE id = ?`,
		status, time.Now().UTC(), id,
	)
	if err != nil {
		return fmt.Errorf("update flow status: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return core.ErrNotFound
	}
	return nil
}
