package initiativeapp

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/yoke233/zhanggui/internal/adapters/store/sqlite"
	"github.com/yoke233/zhanggui/internal/core"
)

type schedulerStub struct {
	submitted []int64
}

func (s *schedulerStub) Submit(_ context.Context, workItemID int64) error {
	s.submitted = append(s.submitted, workItemID)
	return nil
}

type sqliteInitiativeTx struct {
	base core.TransactionalStore
}

func (t sqliteInitiativeTx) InTx(ctx context.Context, fn func(ctx context.Context, store Store) error) error {
	return t.base.InTx(ctx, func(store core.Store) error {
		txStore, ok := store.(Store)
		if !ok {
			return core.ErrInvalidTransition
		}
		return fn(ctx, txStore)
	})
}

func newInitiativeServiceTestStore(t *testing.T) *sqlite.Store {
	t.Helper()
	store, err := sqlite.New(filepath.Join(t.TempDir(), "initiative-service.db"))
	if err != nil {
		t.Fatalf("sqlite.New() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestServiceApproveInitiativeQueuesRootsAndHoldsDependents(t *testing.T) {
	store := newInitiativeServiceTestStore(t)
	ctx := context.Background()

	projectA, _ := store.CreateProject(ctx, &core.Project{Name: "project-a"})
	projectB, _ := store.CreateProject(ctx, &core.Project{Name: "project-b"})

	rootID, err := store.CreateWorkItem(ctx, &core.WorkItem{ProjectID: &projectA, Title: "root", Status: core.WorkItemOpen})
	if err != nil {
		t.Fatalf("CreateWorkItem(root): %v", err)
	}
	childID, err := store.CreateWorkItem(ctx, &core.WorkItem{ProjectID: &projectB, Title: "child", Status: core.WorkItemOpen, DependsOn: []int64{rootID}})
	if err != nil {
		t.Fatalf("CreateWorkItem(child): %v", err)
	}

	svc := New(Config{Store: store, Tx: sqliteInitiativeTx{base: store}})
	initiative, err := svc.CreateInitiative(ctx, CreateInitiativeInput{Title: "cross-project rollout", CreatedBy: "user-1"})
	if err != nil {
		t.Fatalf("CreateInitiative: %v", err)
	}
	if _, err := svc.AddWorkItem(ctx, AddInitiativeItemInput{InitiativeID: initiative.ID, WorkItemID: rootID, Role: "backend"}); err != nil {
		t.Fatalf("AddWorkItem(root): %v", err)
	}
	if _, err := svc.AddWorkItem(ctx, AddInitiativeItemInput{InitiativeID: initiative.ID, WorkItemID: childID, Role: "frontend"}); err != nil {
		t.Fatalf("AddWorkItem(child): %v", err)
	}
	if _, err := svc.Propose(ctx, initiative.ID); err != nil {
		t.Fatalf("Propose: %v", err)
	}
	if _, err := svc.Approve(ctx, initiative.ID, "reviewer-1"); err != nil {
		t.Fatalf("Approve: %v", err)
	}

	root, _ := store.GetWorkItem(ctx, rootID)
	if root.Status != core.WorkItemQueued {
		t.Fatalf("root status = %s, want queued", root.Status)
	}
	child, _ := store.GetWorkItem(ctx, childID)
	if child.Status != core.WorkItemAccepted {
		t.Fatalf("child status = %s, want accepted", child.Status)
	}
}

func TestServiceApproveInitiativeUsesSchedulerAfterTransaction(t *testing.T) {
	store := newInitiativeServiceTestStore(t)
	ctx := context.Background()

	workItemID, err := store.CreateWorkItem(ctx, &core.WorkItem{Title: "root", Status: core.WorkItemOpen})
	if err != nil {
		t.Fatalf("CreateWorkItem: %v", err)
	}

	stub := &schedulerStub{}
	svc := New(Config{Store: store, Tx: sqliteInitiativeTx{base: store}, Scheduler: stub})
	initiative, err := svc.CreateInitiative(ctx, CreateInitiativeInput{Title: "approve-with-scheduler", CreatedBy: "user-1"})
	if err != nil {
		t.Fatalf("CreateInitiative: %v", err)
	}
	if _, err := svc.AddWorkItem(ctx, AddInitiativeItemInput{InitiativeID: initiative.ID, WorkItemID: workItemID}); err != nil {
		t.Fatalf("AddWorkItem: %v", err)
	}
	if _, err := svc.Propose(ctx, initiative.ID); err != nil {
		t.Fatalf("Propose: %v", err)
	}
	if _, err := svc.Approve(ctx, initiative.ID, "reviewer-1"); err != nil {
		t.Fatalf("Approve: %v", err)
	}

	if len(stub.submitted) != 1 || stub.submitted[0] != workItemID {
		t.Fatalf("scheduler submitted = %+v, want [%d]", stub.submitted, workItemID)
	}
	workItem, err := store.GetWorkItem(ctx, workItemID)
	if err != nil {
		t.Fatalf("GetWorkItem: %v", err)
	}
	if workItem.Status != core.WorkItemOpen {
		t.Fatalf("work item status = %s, want open before scheduler transition", workItem.Status)
	}
}

func TestServiceApproveInitiativeDoesNotRequeueCompletedItems(t *testing.T) {
	store := newInitiativeServiceTestStore(t)
	ctx := context.Background()

	openID, err := store.CreateWorkItem(ctx, &core.WorkItem{Title: "open-root", Status: core.WorkItemOpen})
	if err != nil {
		t.Fatalf("CreateWorkItem(open): %v", err)
	}
	doneID, err := store.CreateWorkItem(ctx, &core.WorkItem{Title: "done-root", Status: core.WorkItemDone})
	if err != nil {
		t.Fatalf("CreateWorkItem(done): %v", err)
	}

	stub := &schedulerStub{}
	svc := New(Config{Store: store, Tx: sqliteInitiativeTx{base: store}, Scheduler: stub})
	initiative, err := svc.CreateInitiative(ctx, CreateInitiativeInput{Title: "preserve-terminal", CreatedBy: "user-1"})
	if err != nil {
		t.Fatalf("CreateInitiative: %v", err)
	}
	if _, err := svc.AddWorkItem(ctx, AddInitiativeItemInput{InitiativeID: initiative.ID, WorkItemID: openID}); err != nil {
		t.Fatalf("AddWorkItem(open): %v", err)
	}
	if _, err := svc.AddWorkItem(ctx, AddInitiativeItemInput{InitiativeID: initiative.ID, WorkItemID: doneID}); err != nil {
		t.Fatalf("AddWorkItem(done): %v", err)
	}
	if _, err := svc.Propose(ctx, initiative.ID); err != nil {
		t.Fatalf("Propose: %v", err)
	}
	if _, err := svc.Approve(ctx, initiative.ID, "reviewer-1"); err != nil {
		t.Fatalf("Approve: %v", err)
	}

	if len(stub.submitted) != 1 || stub.submitted[0] != openID {
		t.Fatalf("scheduler submitted = %+v, want [%d]", stub.submitted, openID)
	}
	doneItem, err := store.GetWorkItem(ctx, doneID)
	if err != nil {
		t.Fatalf("GetWorkItem(done): %v", err)
	}
	if doneItem.Status != core.WorkItemDone {
		t.Fatalf("done work item status = %s, want done", doneItem.Status)
	}
}

func TestServiceGetInitiativeDetailIncludesProgressAndThreads(t *testing.T) {
	store := newInitiativeServiceTestStore(t)
	ctx := context.Background()

	workItemID, err := store.CreateWorkItem(ctx, &core.WorkItem{Title: "tracked-item", Status: core.WorkItemOpen})
	if err != nil {
		t.Fatalf("CreateWorkItem: %v", err)
	}
	threadID, err := store.CreateThread(ctx, &core.Thread{Title: "source", Status: core.ThreadActive, OwnerID: "user-1"})
	if err != nil {
		t.Fatalf("CreateThread: %v", err)
	}

	svc := New(Config{Store: store})
	initiative, err := svc.CreateInitiative(ctx, CreateInitiativeInput{Title: "detail", CreatedBy: "user-1"})
	if err != nil {
		t.Fatalf("CreateInitiative: %v", err)
	}
	if _, err := svc.AddWorkItem(ctx, AddInitiativeItemInput{InitiativeID: initiative.ID, WorkItemID: workItemID}); err != nil {
		t.Fatalf("AddWorkItem: %v", err)
	}
	if _, err := svc.LinkThread(ctx, LinkThreadInput{InitiativeID: initiative.ID, ThreadID: threadID, RelationType: "source"}); err != nil {
		t.Fatalf("LinkThread: %v", err)
	}
	if _, err := svc.Propose(ctx, initiative.ID); err != nil {
		t.Fatalf("Propose: %v", err)
	}
	if _, err := svc.Approve(ctx, initiative.ID, "reviewer-1"); err != nil {
		t.Fatalf("Approve: %v", err)
	}
	if err := store.UpdateWorkItemStatus(ctx, workItemID, core.WorkItemDone); err != nil {
		t.Fatalf("UpdateWorkItemStatus(done): %v", err)
	}

	detail, err := svc.GetInitiativeDetail(ctx, initiative.ID)
	if err != nil {
		t.Fatalf("GetInitiativeDetail: %v", err)
	}
	if detail.Progress.Done != 1 || detail.Progress.Total != 1 {
		t.Fatalf("unexpected progress: %+v", detail.Progress)
	}
	if len(detail.Threads) != 1 || detail.Threads[0].ThreadID != threadID {
		t.Fatalf("unexpected threads: %+v", detail.Threads)
	}
	if detail.Initiative.Status != core.InitiativeDone {
		t.Fatalf("initiative status = %s, want done", detail.Initiative.Status)
	}
}

func TestServiceGetInitiativeDetailTransitionsBlockedToDone(t *testing.T) {
	store := newInitiativeServiceTestStore(t)
	ctx := context.Background()

	workItemID, err := store.CreateWorkItem(ctx, &core.WorkItem{Title: "blocked-then-done", Status: core.WorkItemOpen})
	if err != nil {
		t.Fatalf("CreateWorkItem: %v", err)
	}

	svc := New(Config{Store: store})
	initiative, err := svc.CreateInitiative(ctx, CreateInitiativeInput{Title: "blocked-transition", CreatedBy: "user-1"})
	if err != nil {
		t.Fatalf("CreateInitiative: %v", err)
	}
	if _, err := svc.AddWorkItem(ctx, AddInitiativeItemInput{InitiativeID: initiative.ID, WorkItemID: workItemID}); err != nil {
		t.Fatalf("AddWorkItem: %v", err)
	}
	if _, err := svc.Propose(ctx, initiative.ID); err != nil {
		t.Fatalf("Propose: %v", err)
	}
	if _, err := svc.Approve(ctx, initiative.ID, "reviewer-1"); err != nil {
		t.Fatalf("Approve: %v", err)
	}
	if err := store.UpdateWorkItemStatus(ctx, workItemID, core.WorkItemBlocked); err != nil {
		t.Fatalf("UpdateWorkItemStatus(blocked): %v", err)
	}

	detail, err := svc.GetInitiativeDetail(ctx, initiative.ID)
	if err != nil {
		t.Fatalf("GetInitiativeDetail(blocked): %v", err)
	}
	if detail.Initiative.Status != core.InitiativeBlocked {
		t.Fatalf("initiative status = %s, want blocked", detail.Initiative.Status)
	}

	if err := store.UpdateWorkItemStatus(ctx, workItemID, core.WorkItemDone); err != nil {
		t.Fatalf("UpdateWorkItemStatus(done): %v", err)
	}
	detail, err = svc.GetInitiativeDetail(ctx, initiative.ID)
	if err != nil {
		t.Fatalf("GetInitiativeDetail(done): %v", err)
	}
	if detail.Initiative.Status != core.InitiativeDone {
		t.Fatalf("initiative status = %s, want done", detail.Initiative.Status)
	}
}
