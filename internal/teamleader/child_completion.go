package teamleader

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/yoke233/ai-workflow/internal/core"
)

// ChildCompletionHandler listens for EventIssueDone / EventIssueFailed and
// checks whether all siblings of the completed child are finished.  When every
// child is done the parent issue is closed automatically.
type ChildCompletionHandler struct {
	store core.Store
	pub   eventPublisher
	log   *slog.Logger
}

// NewChildCompletionHandler creates a handler that tracks child issue completion.
func NewChildCompletionHandler(store core.Store, pub eventPublisher) *ChildCompletionHandler {
	return &ChildCompletionHandler{
		store: store,
		pub:   pub,
		log:   slog.Default(),
	}
}

// Start subscribes to the event bus and processes completion events.
func (h *ChildCompletionHandler) Start(ctx context.Context, bus eventSubscriber) {
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

// OnEvent handles a single event.  Reacts to EventIssueDone and EventIssueFailed.
func (h *ChildCompletionHandler) OnEvent(ctx context.Context, evt core.Event) {
	if evt.Type != core.EventIssueDone && evt.Type != core.EventIssueFailed {
		return
	}
	issueID := strings.TrimSpace(evt.IssueID)
	if issueID == "" {
		return
	}

	child, err := h.store.GetIssue(issueID)
	if err != nil || child == nil {
		return
	}
	if child.ParentID == "" {
		return // not a child issue
	}

	parent, err := h.store.GetIssue(child.ParentID)
	if err != nil || parent == nil {
		h.log.Warn("child_completion: parent not found", "parent_id", child.ParentID, "error", err)
		return
	}
	if parent.Status != core.IssueStatusDecomposed {
		return // parent not in decomposed state
	}

	siblings, err := h.store.GetChildIssues(parent.ID)
	if err != nil {
		h.log.Error("child_completion: get children failed", "parent_id", parent.ID, "error", err)
		return
	}

	var allDone, anyFailed bool
	allDone = true
	for _, s := range siblings {
		switch s.Status {
		case core.IssueStatusDone:
			// ok
		case core.IssueStatusFailed:
			anyFailed = true
		default:
			allDone = false
		}
	}

	if !allDone {
		return
	}

	if anyFailed {
		h.resolveParentWithFailures(parent)
	} else {
		h.resolveParentSuccess(parent)
	}
}

func (h *ChildCompletionHandler) resolveParentSuccess(parent *core.Issue) {
	now := time.Now()
	parent.Status = core.IssueStatusDone
	parent.State = core.IssueStateClosed
	parent.ClosedAt = &now
	if err := h.store.SaveIssue(parent); err != nil {
		h.log.Error("child_completion: save parent done", "parent_id", parent.ID, "error", err)
		return
	}
	h.pub.Publish(core.Event{
		Type:      core.EventIssueDone,
		IssueID:   parent.ID,
		ProjectID: parent.ProjectID,
		Timestamp: now,
	})
	h.log.Info("child_completion: parent done", "parent_id", parent.ID)
}

func (h *ChildCompletionHandler) resolveParentWithFailures(parent *core.Issue) {
	switch parent.FailPolicy {
	case core.FailSkip:
		// Treat as success if non-failed children are all done.
		h.resolveParentSuccess(parent)
	case core.FailHuman:
		h.pub.Publish(core.Event{
			Type:      core.EventIssueFailed,
			IssueID:   parent.ID,
			ProjectID: parent.ProjectID,
			Error:     "child issues failed, human review required",
			Timestamp: time.Now(),
		})
	default: // FailBlock
		parent.Status = core.IssueStatusFailed
		if err := h.store.SaveIssue(parent); err != nil {
			h.log.Error("child_completion: save parent failed", "parent_id", parent.ID, "error", err)
			return
		}
		h.pub.Publish(core.Event{
			Type:      core.EventIssueFailed,
			IssueID:   parent.ID,
			ProjectID: parent.ProjectID,
			Error:     "one or more child issues failed",
			Timestamp: time.Now(),
		})
		h.log.Info("child_completion: parent failed", "parent_id", parent.ID)
	}
}
