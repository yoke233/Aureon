package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/yoke233/zhanggui/internal/core"
)

func newInitiativeTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := New(filepath.Join(t.TempDir(), "initiative-test.db"))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestStoreInitiativeCRUDAndLinks(t *testing.T) {
	store := newInitiativeTestStore(t)
	ctx := context.Background()

	initiativeID, err := store.CreateInitiative(ctx, &core.Initiative{
		Title:       "cross-project rollout",
		Description: "coordinate backend and frontend work",
		Status:      core.InitiativeDraft,
		CreatedBy:   "user-1",
	})
	if err != nil {
		t.Fatalf("CreateInitiative() error = %v", err)
	}

	workItemID, err := store.CreateWorkItem(ctx, &core.WorkItem{
		Title:  "backend",
		Status: core.WorkItemOpen,
	})
	if err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
	}
	if _, err := store.CreateInitiativeItem(ctx, &core.InitiativeItem{
		InitiativeID: initiativeID,
		WorkItemID:   workItemID,
		Role:         "lead",
	}); err != nil {
		t.Fatalf("CreateInitiativeItem() error = %v", err)
	}

	threadID, err := store.CreateThread(ctx, &core.Thread{Title: "proposal", Status: core.ThreadActive, OwnerID: "user-1"})
	if err != nil {
		t.Fatalf("CreateThread() error = %v", err)
	}
	if _, err := store.CreateThreadInitiativeLink(ctx, &core.ThreadInitiativeLink{
		ThreadID:     threadID,
		InitiativeID: initiativeID,
		RelationType: "source",
	}); err != nil {
		t.Fatalf("CreateThreadInitiativeLink() error = %v", err)
	}

	items, err := store.ListInitiativeItems(ctx, initiativeID)
	if err != nil {
		t.Fatalf("ListInitiativeItems() error = %v", err)
	}
	if len(items) != 1 || items[0].WorkItemID != workItemID {
		t.Fatalf("ListInitiativeItems() = %+v", items)
	}

	threads, err := store.ListThreadsByInitiative(ctx, initiativeID)
	if err != nil {
		t.Fatalf("ListThreadsByInitiative() error = %v", err)
	}
	if len(threads) != 1 || threads[0].ThreadID != threadID {
		t.Fatalf("ListThreadsByInitiative() = %+v", threads)
	}
}

func TestStoreListDependentWorkItems(t *testing.T) {
	store := newInitiativeTestStore(t)
	ctx := context.Background()

	parentID, err := store.CreateWorkItem(ctx, &core.WorkItem{Title: "parent", Status: core.WorkItemDone})
	if err != nil {
		t.Fatalf("CreateWorkItem(parent): %v", err)
	}
	childID, err := store.CreateWorkItem(ctx, &core.WorkItem{
		Title:     "child",
		Status:    core.WorkItemAccepted,
		DependsOn: []int64{parentID},
	})
	if err != nil {
		t.Fatalf("CreateWorkItem(child): %v", err)
	}

	items, err := store.ListDependentWorkItems(ctx, parentID)
	if err != nil {
		t.Fatalf("ListDependentWorkItems() error = %v", err)
	}
	if len(items) != 1 || items[0].ID != childID {
		t.Fatalf("ListDependentWorkItems() = %+v", items)
	}
}
