package api

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/yoke233/zhanggui/internal/core"
)

func TestListPendingWorkItemsIncludesFinalDeliverableAndPendingAction(t *testing.T) {
	h, ts := setupAPI(t)
	ctx := context.Background()

	workItemID, err := h.store.CreateWorkItem(ctx, &core.WorkItem{
		Title:             "review inbox item",
		Status:            core.WorkItemPendingReview,
		Priority:          core.PriorityMedium,
		ReviewerProfileID: "ceo",
	})
	if err != nil {
		t.Fatalf("create work item: %v", err)
	}

	actionID, err := h.store.CreateAction(ctx, &core.Action{
		WorkItemID: workItemID,
		Name:       "gate review",
		Type:       core.ActionGate,
		Status:     core.ActionReady,
		Position:   1,
	})
	if err != nil {
		t.Fatalf("create action: %v", err)
	}

	if _, err := h.store.CreateActionSignal(ctx, &core.ActionSignal{
		ActionID:   actionID,
		WorkItemID: workItemID,
		Type:       core.SignalFeedback,
		Source:     core.SignalSourceSystem,
		Content:    "please verify the final summary",
		CreatedAt:  time.Now().UTC(),
	}); err != nil {
		t.Fatalf("create action signal: %v", err)
	}

	deliverableID, err := h.store.CreateDeliverable(ctx, &core.Deliverable{
		WorkItemID:   &workItemID,
		Kind:         core.DeliverableDecision,
		Title:        "Review Decision",
		Summary:      "accepted after final verification",
		ProducerType: core.DeliverableProducerWorkItem,
		ProducerID:   workItemID,
		Status:       core.DeliverableFinal,
	})
	if err != nil {
		t.Fatalf("create deliverable: %v", err)
	}

	workItem, err := h.store.GetWorkItem(ctx, workItemID)
	if err != nil {
		t.Fatalf("get work item: %v", err)
	}
	workItem.FinalDeliverableID = &deliverableID
	if err := h.store.UpdateWorkItem(ctx, workItem); err != nil {
		t.Fatalf("update work item: %v", err)
	}

	resp, err := get(ts, "/work-items/pending?profile_id=ceo")
	if err != nil {
		t.Fatalf("get pending work items: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /work-items/pending status = %d", resp.StatusCode)
	}

	var items []pendingWorkItemItem
	if err := decodeJSON(resp, &items); err != nil {
		t.Fatalf("decode pending work items: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	if items[0].WorkItem == nil || items[0].WorkItem.ID != workItemID {
		t.Fatalf("unexpected work item payload: %+v", items[0].WorkItem)
	}
	if items[0].Reason != "pending_review" {
		t.Fatalf("Reason = %q, want pending_review", items[0].Reason)
	}
	if items[0].NextHandler != "ceo" {
		t.Fatalf("NextHandler = %q, want ceo", items[0].NextHandler)
	}
	if items[0].LatestSummary != "accepted after final verification" {
		t.Fatalf("LatestSummary = %q, want final deliverable summary", items[0].LatestSummary)
	}
	if items[0].PendingAction == nil || items[0].PendingAction.ID != actionID {
		t.Fatalf("PendingAction = %+v, want action %d", items[0].PendingAction, actionID)
	}
	if items[0].LatestContext == nil || items[0].LatestContext.Content != "please verify the final summary" {
		t.Fatalf("LatestContext = %+v", items[0].LatestContext)
	}

	resp, err = get(ts, "/work-items/pending?profile_id=human")
	if err != nil {
		t.Fatalf("get human pending work items: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /work-items/pending?profile_id=human status = %d", resp.StatusCode)
	}
	if err := decodeJSON(resp, &items); err != nil {
		t.Fatalf("decode human pending work items: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected no human pending items, got %+v", items)
	}

	resp, err = get(ts, fmt.Sprintf("/work-items/%d", workItemID))
	if err != nil {
		t.Fatalf("get work item detail: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /work-items/{id} status = %d", resp.StatusCode)
	}
}
