package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/yoke233/ai-workflow/internal/v2/core"
)

func (s *Store) CreateEvent(ctx context.Context, e *core.Event) (int64, error) {
	data, err := marshalJSON(e.Data)
	if err != nil {
		return 0, fmt.Errorf("marshal event data: %w", err)
	}
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now().UTC()
	}
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO events (type, flow_id, step_id, exec_id, data, timestamp)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		e.Type, nilIfZero(e.FlowID), nilIfZero(e.StepID), nilIfZero(e.ExecID), data, e.Timestamp,
	)
	if err != nil {
		return 0, fmt.Errorf("insert event: %w", err)
	}
	id, _ := res.LastInsertId()
	e.ID = id
	return id, nil
}

func (s *Store) ListEvents(ctx context.Context, filter core.EventFilter) ([]*core.Event, error) {
	query := `SELECT id, type, flow_id, step_id, exec_id, data, timestamp FROM events`
	var conditions []string
	var args []any

	if filter.FlowID != nil {
		conditions = append(conditions, "flow_id = ?")
		args = append(args, *filter.FlowID)
	}
	if filter.StepID != nil {
		conditions = append(conditions, "step_id = ?")
		args = append(args, *filter.StepID)
	}
	if len(filter.Types) > 0 {
		placeholders := make([]string, len(filter.Types))
		for i, t := range filter.Types {
			placeholders[i] = "?"
			args = append(args, t)
		}
		conditions = append(conditions, fmt.Sprintf("type IN (%s)", strings.Join(placeholders, ",")))
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY id"
	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", filter.Limit)
	}
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", filter.Offset)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	defer rows.Close()

	var events []*core.Event
	for rows.Next() {
		e := &core.Event{}
		var flowID, stepID, execID sql.NullInt64
		var data sql.NullString
		if err := rows.Scan(&e.ID, &e.Type, &flowID, &stepID, &execID, &data, &e.Timestamp); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		if flowID.Valid {
			e.FlowID = flowID.Int64
		}
		if stepID.Valid {
			e.StepID = stepID.Int64
		}
		if execID.Valid {
			e.ExecID = execID.Int64
		}
		if data.Valid {
			_ = json.Unmarshal([]byte(data.String), &e.Data)
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

func nilIfZero(v int64) any {
	if v == 0 {
		return nil
	}
	return v
}
