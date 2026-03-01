package core

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

type WaitReason string

const (
	WaitNone          WaitReason = ""
	WaitFinalApproval WaitReason = "final_approval"
	WaitFeedbackReq   WaitReason = "feedback_required"
)

type TaskPlanStatus string

const (
	PlanDraft        TaskPlanStatus = "draft"
	PlanReviewing    TaskPlanStatus = "reviewing"
	PlanApproved     TaskPlanStatus = "approved"
	PlanWaitingHuman TaskPlanStatus = "waiting_human"
	PlanExecuting    TaskPlanStatus = "executing"
	PlanPartial      TaskPlanStatus = "partially_done"
	PlanDone         TaskPlanStatus = "done"
	PlanFailed       TaskPlanStatus = "failed"
	PlanAbandoned    TaskPlanStatus = "abandoned"
)

type TaskItemStatus string

const (
	ItemPending          TaskItemStatus = "pending"
	ItemReady            TaskItemStatus = "ready"
	ItemRunning          TaskItemStatus = "running"
	ItemDone             TaskItemStatus = "done"
	ItemFailed           TaskItemStatus = "failed"
	ItemSkipped          TaskItemStatus = "skipped"
	ItemBlockedByFailure TaskItemStatus = "blocked_by_failure"
)

type FailurePolicy string

const (
	FailBlock FailurePolicy = "block"
	FailSkip  FailurePolicy = "skip"
	FailHuman FailurePolicy = "human"
)

type TaskPlan struct {
	ID          string         `json:"id"`
	ProjectID   string         `json:"project_id"`
	SessionID   string         `json:"session_id"`
	Name        string         `json:"name"`
	Status      TaskPlanStatus `json:"status"`
	WaitReason  WaitReason     `json:"wait_reason"`
	Tasks       []TaskItem     `json:"tasks"`
	FailPolicy  FailurePolicy  `json:"fail_policy"`
	ReviewRound int            `json:"review_round"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

type TaskItem struct {
	ID          string         `json:"id"`
	PlanID      string         `json:"plan_id"`
	Title       string         `json:"title"`
	Description string         `json:"description"` // required
	Labels      []string       `json:"labels"`
	DependsOn   []string       `json:"depends_on"`
	Template    string         `json:"template"`
	PipelineID  string         `json:"pipeline_id"`
	ExternalID  string         `json:"external_id"`
	Status      TaskItemStatus `json:"status"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// Validate checks required TaskItem fields at the domain-model layer.
func (t TaskItem) Validate() error {
	if strings.TrimSpace(t.Description) == "" {
		return errors.New("task item description is required")
	}
	return nil
}

// NewTaskPlanID generates an ID in format: plan-YYYYMMDD-xxxxxxxx.
func NewTaskPlanID() string {
	return fmt.Sprintf("plan-%s-%s", time.Now().Format("20060102"), randomHex(4))
}

// NewTaskItemID generates an ID in format: task-{planShortID}-{sequence}.
func NewTaskItemID(planID string, sequence int) string {
	return fmt.Sprintf("task-%s-%d", planShortID(planID), sequence)
}

func planShortID(planID string) string {
	parts := strings.Split(planID, "-")
	if len(parts) >= 3 && parts[0] == "plan" {
		return parts[len(parts)-1]
	}
	return planID
}
