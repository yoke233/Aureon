package api

import (
	"net/http"
	"strings"

	"github.com/yoke233/zhanggui/internal/core"
)

type pendingWorkItemItem struct {
	WorkItem      *core.WorkItem     `json:"work_item"`
	Reason        string             `json:"reason"`
	NextHandler   string             `json:"next_handler"`
	LatestSummary string             `json:"latest_summary,omitempty"`
	PendingAction *core.Action       `json:"pending_action,omitempty"`
	LatestContext *core.ActionSignal `json:"latest_context,omitempty"`
}

func (h *Handler) listPendingWorkItems(w http.ResponseWriter, r *http.Request) {
	profileID := strings.TrimSpace(r.URL.Query().Get("profile_id"))
	archived := false
	workItems, err := h.store.ListWorkItems(r.Context(), core.WorkItemFilter{
		Archived: &archived,
		Limit:    1000,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "STORE_ERROR")
		return
	}

	items := make([]pendingWorkItemItem, 0)
	for _, workItem := range workItems {
		if workItem == nil {
			continue
		}
		reason, nextHandler, ok := classifyPendingWorkItem(workItem)
		if !ok {
			continue
		}
		if profileID != "" && nextHandler != profileID {
			continue
		}

		item := pendingWorkItemItem{
			WorkItem:    workItem,
			Reason:      reason,
			NextHandler: nextHandler,
		}
		item.LatestSummary = h.pendingWorkItemSummary(r, workItem.ID, workItem.FinalDeliverableID)

		pendingActions, actionErr := h.store.ListPendingHumanActions(r.Context(), workItem.ID)
		if actionErr == nil && len(pendingActions) > 0 {
			item.PendingAction = pendingActions[0]
			if latestContext, ctxErr := h.store.GetLatestActionSignal(r.Context(), item.PendingAction.ID, core.SignalContext, core.SignalFeedback); ctxErr == nil {
				item.LatestContext = latestContext
			}
		}
		items = append(items, item)
	}

	writeJSON(w, http.StatusOK, items)
}

func classifyPendingWorkItem(workItem *core.WorkItem) (reason string, nextHandler string, ok bool) {
	if workItem == nil {
		return "", "", false
	}
	switch workItem.Status {
	case core.WorkItemPendingReview:
		return "pending_review", firstPendingHandler(workItem.ReviewerProfileID, "human"), true
	case core.WorkItemNeedsRework:
		return "needs_rework", firstPendingHandler(firstEscalationHandler(workItem.EscalationPath), workItem.ReviewerProfileID, "human"), true
	case core.WorkItemEscalated:
		return "escalated", firstPendingHandler(firstEscalationHandler(workItem.EscalationPath), "human"), true
	default:
		return "", "", false
	}
}

func firstEscalationHandler(path []string) string {
	for _, item := range path {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func firstPendingHandler(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return "human"
}

func (h *Handler) pendingWorkItemSummary(r *http.Request, workItemID int64, finalDeliverableID *int64) string {
	if h == nil || h.store == nil {
		return ""
	}
	if finalDeliverableID != nil {
		deliverable, err := h.store.GetDeliverable(r.Context(), *finalDeliverableID)
		if err == nil {
			return compactPendingText(summarizePendingDeliverable(deliverable), 160)
		}
	}
	actions, err := h.store.ListActionsByWorkItem(r.Context(), workItemID)
	if err != nil {
		return ""
	}
	for _, action := range actions {
		if action == nil {
			continue
		}
		run, runErr := h.store.GetLatestRunWithResult(r.Context(), action.ID)
		if runErr == nil && run != nil && strings.TrimSpace(run.ResultMarkdown) != "" {
			return compactPendingText(run.ResultMarkdown, 160)
		}
	}
	return ""
}

func summarizePendingDeliverable(deliverable *core.Deliverable) string {
	if deliverable == nil {
		return ""
	}
	if summary := strings.TrimSpace(deliverable.Summary); summary != "" {
		return summary
	}
	if title := strings.TrimSpace(deliverable.Title); title != "" {
		return title
	}
	return ""
}

func compactPendingText(raw string, limit int) string {
	if limit <= 0 {
		limit = 160
	}
	text := strings.Join(strings.Fields(strings.TrimSpace(raw)), " ")
	if len(text) <= limit {
		return text
	}
	return strings.TrimSpace(text[:limit]) + "..."
}
