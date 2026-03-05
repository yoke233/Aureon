package teamleader

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/yoke233/ai-workflow/internal/core"
)

// DecomposeSpec describes a single child issue produced by the decomposer.
type DecomposeSpec struct {
	Title    string   `json:"title"`
	Body     string   `json:"body"`
	Template string   `json:"template"`
	Labels   []string `json:"labels"`
	Priority int      `json:"priority"`
}

// DecomposeFunc analyzes a parent issue and returns child issue specs.
// The production implementation calls a decomposer ACP agent.
type DecomposeFunc func(ctx context.Context, parent *core.Issue) ([]DecomposeSpec, error)

// decomposeIssueCreator is the minimal interface DecomposeHandler needs.
type decomposeIssueCreator interface {
	CreateIssue(i *core.Issue) error
}

// decomposeReviewSubmitter allows DecomposeHandler to auto-submit child issues
// for review after decomposition.  Optional — if nil, children stay in draft.
type decomposeReviewSubmitter interface {
	SubmitForReview(ctx context.Context, issueIDs []string) error
}

// DecomposeHandler listens for EventIssueDecomposing and spawns child issues.
type DecomposeHandler struct {
	store     decomposeIssueCreator
	fullStore core.Store
	pub       eventPublisher
	decompose DecomposeFunc
	reviewer  decomposeReviewSubmitter
	log       *slog.Logger
}

// NewDecomposeHandler creates a handler that decomposes epic issues into children.
// reviewer may be nil; if non-nil, child issues are auto-submitted for review.
func NewDecomposeHandler(store core.Store, pub eventPublisher, fn DecomposeFunc) *DecomposeHandler {
	return &DecomposeHandler{
		store:     store,
		fullStore: store,
		pub:       pub,
		decompose: fn,
		log:       slog.Default(),
	}
}

// SetReviewSubmitter sets an optional review submitter for auto-submitting
// child issues after decomposition.
func (h *DecomposeHandler) SetReviewSubmitter(r decomposeReviewSubmitter) {
	h.reviewer = r
}

// Start subscribes to the event bus and processes decompose events.
func (h *DecomposeHandler) Start(ctx context.Context, bus eventSubscriber) {
	ch := bus.Subscribe()
	defer bus.Unsubscribe(ch)
	for {
		select {
		case evt := <-ch:
			h.OnEvent(ctx, evt)
		case <-ctx.Done():
			return
		}
	}
}

// OnEvent handles a single event. Only reacts to EventIssueDecomposing.
func (h *DecomposeHandler) OnEvent(ctx context.Context, evt core.Event) {
	if evt.Type != core.EventIssueDecomposing {
		return
	}
	issueID := strings.TrimSpace(evt.IssueID)
	if issueID == "" {
		return
	}

	parent, err := h.fullStore.GetIssue(issueID)
	if err != nil || parent == nil {
		h.log.Warn("decompose: parent issue not found", "issue_id", issueID, "error", err)
		return
	}
	if parent.Status != core.IssueStatusDecomposing {
		return
	}

	specs, err := h.decompose(ctx, parent)
	if err != nil {
		h.log.Error("decompose: agent failed", "issue_id", issueID, "error", err)
		h.markParentFailed(parent, fmt.Sprintf("decompose failed: %v", err))
		return
	}
	if len(specs) == 0 {
		h.log.Error("decompose: agent returned zero specs", "issue_id", issueID)
		h.markParentFailed(parent, "decompose returned zero child issues")
		return
	}

	for _, spec := range specs {
		child := &core.Issue{
			ID:         core.NewIssueID(),
			ProjectID:  parent.ProjectID,
			SessionID:  parent.SessionID,
			ParentID:   parent.ID,
			Title:      spec.Title,
			Body:       spec.Body,
			Template:   spec.Template,
			Labels:     spec.Labels,
			Priority:   spec.Priority,
			AutoMerge:  parent.AutoMerge,
			FailPolicy: parent.FailPolicy,
			State:      core.IssueStateOpen,
			Status:     core.IssueStatusDraft,
		}
		if child.Template == "" {
			child.Template = "standard"
		}
		if err := h.store.CreateIssue(child); err != nil {
			h.log.Error("decompose: create child failed", "parent_id", issueID, "error", err)
			h.markParentFailed(parent, fmt.Sprintf("create child issue failed: %v", err))
			return
		}
		h.log.Info("decompose: child created", "parent_id", issueID, "child_id", child.ID, "title", child.Title)
	}

	// Mark parent as decomposed.
	if err := transitionIssueStatus(parent, core.IssueStatusDecomposed); err != nil {
		h.log.Error("decompose: invalid parent transition", "issue_id", issueID, "error", err)
		return
	}
	if err := h.fullStore.SaveIssue(parent); err != nil {
		h.log.Error("decompose: save parent failed", "issue_id", issueID, "error", err)
		return
	}
	h.pub.Publish(core.Event{
		Type:      core.EventIssueDecomposed,
		IssueID:   parent.ID,
		ProjectID: parent.ProjectID,
		Data:      map[string]string{"child_count": fmt.Sprintf("%d", len(specs))},
		Timestamp: time.Now(),
	})

	// Auto-submit child issues for review if a reviewer is configured.
	if h.reviewer != nil {
		children, err := h.fullStore.GetChildIssues(parent.ID)
		if err != nil {
			h.log.Error("decompose: get children for review failed", "parent_id", issueID, "error", err)
			return
		}
		childIDs := make([]string, len(children))
		for i, c := range children {
			childIDs[i] = c.ID
		}
		if err := h.reviewer.SubmitForReview(ctx, childIDs); err != nil {
			h.log.Error("decompose: auto-submit children for review failed", "parent_id", issueID, "error", err)
		}
	}
}

func (h *DecomposeHandler) markParentFailed(parent *core.Issue, errMsg string) {
	if err := transitionIssueStatus(parent, core.IssueStatusFailed); err != nil {
		h.log.Error("decompose: invalid parent transition to failed", "issue_id", parent.ID, "error", err)
		return
	}
	if err := h.fullStore.SaveIssue(parent); err != nil {
		h.log.Error("decompose: mark parent failed", "issue_id", parent.ID, "error", err)
	}
	h.pub.Publish(core.Event{
		Type:      core.EventIssueFailed,
		IssueID:   parent.ID,
		ProjectID: parent.ProjectID,
		Error:     errMsg,
		Timestamp: time.Now(),
	})
}
