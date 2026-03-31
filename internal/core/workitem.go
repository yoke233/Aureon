package core

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// WorkItemStatus represents the unified lifecycle state of a WorkItem.
// It covers both planning (open/accepted) and execution (queued/running/done/failed).
type WorkItemStatus string

const (
	WorkItemPendingExecution WorkItemStatus = "pending_execution"
	WorkItemInExecution      WorkItemStatus = "in_execution"
	WorkItemPendingReview    WorkItemStatus = "pending_review"
	WorkItemNeedsRework      WorkItemStatus = "needs_rework"
	WorkItemEscalated        WorkItemStatus = "escalated"
	WorkItemCompleted        WorkItemStatus = "completed"
	WorkItemCancelled        WorkItemStatus = "cancelled"

	// Legacy statuses kept for migration/backfill compatibility.
	WorkItemOpen     WorkItemStatus = "open"
	WorkItemAccepted WorkItemStatus = "accepted"
	WorkItemQueued   WorkItemStatus = "queued"
	WorkItemRunning  WorkItemStatus = WorkItemInExecution
	WorkItemBlocked  WorkItemStatus = WorkItemEscalated
	WorkItemFailed   WorkItemStatus = WorkItemNeedsRework
	WorkItemDone     WorkItemStatus = WorkItemCompleted
	WorkItemClosed   WorkItemStatus = WorkItemCompleted
)

func (s WorkItemStatus) Valid() bool {
	return isLegacyWorkItemStatus(s) || isCanonicalWorkItemStatus(s)
}

func ParseWorkItemStatus(raw string) (WorkItemStatus, error) {
	s := normalizeWorkItemStatusAlias(strings.TrimSpace(raw))
	if !s.Valid() {
		return "", fmt.Errorf("invalid work item status %q", raw)
	}
	return s, nil
}

func normalizeWorkItemStatusAlias(raw string) WorkItemStatus {
	switch WorkItemStatus(raw) {
	case "running":
		return WorkItemRunning
	case "blocked":
		return WorkItemBlocked
	case "failed":
		return WorkItemFailed
	case "done":
		return WorkItemDone
	case "closed":
		return WorkItemClosed
	default:
		return WorkItemStatus(raw)
	}
}

// CanTransitionWorkItemStatus returns true if transitioning from `from` to `to` is allowed.
// Same-status is always permitted (idempotent update).
var workItemTransitions = map[WorkItemStatus][]WorkItemStatus{
	WorkItemPendingExecution: {WorkItemInExecution, WorkItemEscalated, WorkItemCancelled},
	WorkItemInExecution:      {WorkItemPendingReview, WorkItemEscalated, WorkItemNeedsRework, WorkItemCompleted, WorkItemCancelled},
	WorkItemPendingReview:    {WorkItemNeedsRework, WorkItemCompleted, WorkItemEscalated, WorkItemCancelled},
	WorkItemNeedsRework:      {WorkItemPendingExecution, WorkItemInExecution, WorkItemEscalated, WorkItemCancelled},
	WorkItemEscalated:        {WorkItemPendingExecution, WorkItemInExecution, WorkItemPendingReview, WorkItemCancelled},
	WorkItemCompleted:        {},
	WorkItemCancelled:        {},

	// Legacy transitions allowed while the cutover is in progress.
	WorkItemOpen:     {WorkItemAccepted, WorkItemQueued, WorkItemPendingExecution, WorkItemInExecution, WorkItemCancelled, WorkItemClosed},
	WorkItemAccepted: {WorkItemQueued, WorkItemPendingExecution, WorkItemInExecution, WorkItemCancelled, WorkItemClosed},
	WorkItemQueued:   {WorkItemInExecution, WorkItemCancelled},
}

func CanTransitionWorkItemStatus(from, to WorkItemStatus) bool {
	if from == to {
		return true
	}
	for _, allowed := range workItemTransitions[from] {
		if allowed == to {
			return true
		}
	}
	return false
}

func isCanonicalWorkItemStatus(status WorkItemStatus) bool {
	switch status {
	case WorkItemPendingExecution, WorkItemInExecution, WorkItemPendingReview,
		WorkItemNeedsRework, WorkItemEscalated, WorkItemCompleted, WorkItemCancelled:
		return true
	default:
		return false
	}
}

func isLegacyWorkItemStatus(status WorkItemStatus) bool {
	switch status {
	case WorkItemOpen, WorkItemAccepted, WorkItemQueued:
		return true
	default:
		return false
	}
}

// WorkItemPriority represents the urgency of a WorkItem.
type WorkItemPriority string

const (
	PriorityLow    WorkItemPriority = "low"
	PriorityMedium WorkItemPriority = "medium"
	PriorityHigh   WorkItemPriority = "high"
	PriorityUrgent WorkItemPriority = "urgent"
)

// WorkItem is the unified work unit: it combines the planning intent (title, body,
// priority, labels) with the execution context (status lifecycle, actions, workspace).
//
// A WorkItem optionally belongs to a Project and can point at a specific
// ResourceSpace ID for workspace isolation.
type WorkItem struct {
	ID                 int64  `json:"id"`
	ProjectID          *int64 `json:"project_id,omitempty"`
	ResourceSpaceID    *int64 `json:"resource_space_id,omitempty"` // which resource space to work on
	ParentWorkItemID   *int64 `json:"parent_work_item_id,omitempty"`
	RootWorkItemID     *int64 `json:"root_work_item_id,omitempty"`
	FinalDeliverableID *int64 `json:"final_deliverable_id,omitempty"`

	// Planning fields
	Title     string           `json:"title"`
	Body      string           `json:"body"`
	Priority  WorkItemPriority `json:"priority"`
	Labels    []string         `json:"labels,omitempty"`
	DependsOn []int64          `json:"depends_on,omitempty"`

	// Execution fields
	Status             WorkItemStatus `json:"status"`
	ExecutorProfileID  string         `json:"executor_profile_id,omitempty"`
	ReviewerProfileID  string         `json:"reviewer_profile_id,omitempty"`
	ActiveProfileID    string         `json:"active_profile_id,omitempty"`
	SponsorProfileID   string         `json:"sponsor_profile_id,omitempty"`
	CreatedByProfileID string         `json:"created_by_profile_id,omitempty"`
	EscalationPath     []string       `json:"escalation_path,omitempty"`
	Metadata           map[string]any `json:"metadata,omitempty"`

	ArchivedAt *time.Time `json:"archived_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// WorkItemFilter constrains WorkItem queries.
type WorkItemFilter struct {
	ProjectID *int64
	Status    *WorkItemStatus
	Priority  *WorkItemPriority
	Archived  *bool
	Limit     int
	Offset    int
}

// WorkItemStore persists WorkItem aggregates.
type WorkItemStore interface {
	CreateWorkItem(ctx context.Context, w *WorkItem) (int64, error)
	GetWorkItem(ctx context.Context, id int64) (*WorkItem, error)
	ListWorkItems(ctx context.Context, filter WorkItemFilter) ([]*WorkItem, error)
	UpdateWorkItem(ctx context.Context, w *WorkItem) error
	UpdateWorkItemStatus(ctx context.Context, id int64, status WorkItemStatus) error
	UpdateWorkItemMetadata(ctx context.Context, id int64, metadata map[string]any) error
	PrepareWorkItemRun(ctx context.Context, id int64, queuedStatus WorkItemStatus) error
	SetWorkItemArchived(ctx context.Context, id int64, archived bool) error
	DeleteWorkItem(ctx context.Context, id int64) error
}
