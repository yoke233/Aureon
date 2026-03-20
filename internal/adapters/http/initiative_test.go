package api

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/yoke233/zhanggui/internal/core"
)

func TestAPI_InitiativeLifecycle(t *testing.T) {
	h, ts := setupAPI(t)
	ctx := context.Background()

	workItemID, err := h.store.CreateWorkItem(ctx, &core.WorkItem{
		Title:  "backend rollout",
		Status: core.WorkItemOpen,
	})
	if err != nil {
		t.Fatalf("CreateWorkItem: %v", err)
	}
	threadID, err := h.store.CreateThread(ctx, &core.Thread{
		Title:   "proposal thread",
		Status:  core.ThreadActive,
		OwnerID: "user-1",
	})
	if err != nil {
		t.Fatalf("CreateThread: %v", err)
	}

	resp, err := post(ts, "/initiatives", map[string]any{
		"title":       "cross-project rollout",
		"description": "coordinate rollout tasks",
		"created_by":  "user-1",
	})
	if err != nil {
		t.Fatalf("create initiative: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create initiative status = %d, want 201", resp.StatusCode)
	}
	var initiative core.Initiative
	if err := decodeJSON(resp, &initiative); err != nil {
		t.Fatalf("decode initiative: %v", err)
	}

	resp, err = post(ts, "/initiatives/"+itoa64(initiative.ID)+"/items", map[string]any{
		"work_item_id": workItemID,
		"role":         "backend",
	})
	if err != nil {
		t.Fatalf("add initiative item: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("add initiative item status = %d, want 201", resp.StatusCode)
	}

	resp, err = post(ts, "/initiatives/"+itoa64(initiative.ID)+"/threads", map[string]any{
		"thread_id":     threadID,
		"relation_type": "source",
	})
	if err != nil {
		t.Fatalf("link initiative thread: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("link initiative thread status = %d, want 201", resp.StatusCode)
	}

	resp, err = post(ts, "/initiatives/"+itoa64(initiative.ID)+"/propose", map[string]any{})
	if err != nil {
		t.Fatalf("propose initiative: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("propose initiative status = %d, want 200", resp.StatusCode)
	}

	resp, err = post(ts, "/initiatives/"+itoa64(initiative.ID)+"/approve", map[string]any{
		"approved_by": "reviewer-1",
	})
	if err != nil {
		t.Fatalf("approve initiative: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("approve initiative status = %d, want 200", resp.StatusCode)
	}

	resp, err = get(ts, "/initiatives/"+itoa64(initiative.ID))
	if err != nil {
		t.Fatalf("get initiative detail: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get initiative detail status = %d, want 200", resp.StatusCode)
	}
	var detail struct {
		Initiative core.Initiative             `json:"initiative"`
		Threads    []core.ThreadInitiativeLink `json:"threads"`
		Progress   core.InitiativeProgress     `json:"progress"`
	}
	if err := decodeJSON(resp, &detail); err != nil {
		t.Fatalf("decode initiative detail: %v", err)
	}
	if detail.Initiative.Status != core.InitiativeExecuting {
		t.Fatalf("initiative status = %s, want executing", detail.Initiative.Status)
	}
	if len(detail.Threads) != 1 || detail.Threads[0].ThreadID != threadID {
		t.Fatalf("initiative threads = %+v", detail.Threads)
	}

	workItem, err := h.store.GetWorkItem(ctx, workItemID)
	if err != nil {
		t.Fatalf("GetWorkItem: %v", err)
	}
	if workItem.Status != core.WorkItemQueued {
		t.Fatalf("work item status = %s, want queued", workItem.Status)
	}
}

func itoa64(v int64) string {
	return fmt.Sprintf("%d", v)
}
