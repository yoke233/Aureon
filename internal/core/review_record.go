package core

import "time"

// ReviewRecord stores one persisted reviewer/aggregator output for audit trail.
type ReviewRecord struct {
	ID        int64         `json:"id"`
	PlanID    string        `json:"plan_id"`
	Round     int           `json:"round"`
	Reviewer  string        `json:"reviewer"`
	Verdict   string        `json:"verdict"`
	Issues    []ReviewIssue `json:"issues"`
	Fixes     []ProposedFix `json:"fixes"`
	Score     *int          `json:"score,omitempty"`
	CreatedAt time.Time     `json:"created_at"`
}

// ProposedFix is a normalized shape for review-driven plan/task adjustments.
type ProposedFix struct {
	TaskID      string `json:"task_id,omitempty"`
	Description string `json:"description"`
	Suggestion  string `json:"suggestion,omitempty"`
}
