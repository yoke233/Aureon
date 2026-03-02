package reviewlocal

import (
	"context"
	"testing"

	"github.com/yoke233/ai-workflow/internal/core"
	storesqlite "github.com/yoke233/ai-workflow/internal/plugins/store-sqlite"
)

func TestLocalReviewGate_NameInitClose(t *testing.T) {
	store, _ := newTestStoreWithPlan(t)
	gate := New(store)

	if got := gate.Name(); got != "local" {
		t.Fatalf("Name() = %q, want %q", got, "local")
	}
	if err := gate.Init(context.Background()); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	if err := gate.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestLocalReviewGate_SubmitCheckCancelFlow(t *testing.T) {
	store, plan := newTestStoreWithPlan(t)
	gate := New(store)

	reviewID, err := gate.Submit(context.Background(), plan)
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}
	if reviewID != plan.ID {
		t.Fatalf("Submit() reviewID = %q, want %q", reviewID, plan.ID)
	}

	pending, err := gate.Check(context.Background(), reviewID)
	if err != nil {
		t.Fatalf("Check() after submit error = %v", err)
	}
	if pending.Status != "pending" {
		t.Fatalf("pending status = %q, want %q", pending.Status, "pending")
	}
	if pending.Decision != "pending" {
		t.Fatalf("pending decision = %q, want %q", pending.Decision, "pending")
	}
	if len(pending.Verdicts) != 1 || pending.Verdicts[0].Status != "pending" {
		t.Fatalf("pending verdicts = %#v, want one pending verdict", pending.Verdicts)
	}

	records, err := store.GetReviewRecords(plan.ID)
	if err != nil {
		t.Fatalf("GetReviewRecords() after submit error = %v", err)
	}
	if len(records) != 1 || records[0].Verdict != "pending" {
		t.Fatalf("review records after submit = %#v, want one pending record", records)
	}

	if err := gate.Cancel(context.Background(), reviewID); err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}

	cancelled, err := gate.Check(context.Background(), reviewID)
	if err != nil {
		t.Fatalf("Check() after cancel error = %v", err)
	}
	if cancelled.Status != "cancelled" {
		t.Fatalf("cancelled status = %q, want %q", cancelled.Status, "cancelled")
	}
	if cancelled.Decision != "cancelled" {
		t.Fatalf("cancelled decision = %q, want %q", cancelled.Decision, "cancelled")
	}
}

func TestLocalReviewGate_CheckReadsLatestVerdict(t *testing.T) {
	store, plan := newTestStoreWithPlan(t)
	gate := New(store)

	reviewID, err := gate.Submit(context.Background(), plan)
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	if err := store.SaveReviewRecord(&core.ReviewRecord{
		PlanID:   plan.ID,
		Round:    1,
		Reviewer: "local_human",
		Verdict:  "approved",
	}); err != nil {
		t.Fatalf("SaveReviewRecord(approved) error = %v", err)
	}

	result, err := gate.Check(context.Background(), reviewID)
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if result.Status != "approved" {
		t.Fatalf("status = %q, want %q", result.Status, "approved")
	}
	if result.Decision != "approve" {
		t.Fatalf("decision = %q, want %q", result.Decision, "approve")
	}
}

func TestLocalReviewGate_Boundaries(t *testing.T) {
	store, plan := newTestStoreWithPlan(t)
	gate := New(store)

	if _, err := gate.Submit(context.Background(), nil); err == nil {
		t.Fatalf("expected Submit(nil) to fail")
	}
	if _, err := gate.Submit(context.Background(), &core.TaskPlan{}); err == nil {
		t.Fatalf("expected Submit(empty plan) to fail")
	}
	if _, err := gate.Check(context.Background(), ""); err == nil {
		t.Fatalf("expected Check(empty reviewID) to fail")
	}
	if _, err := gate.Check(context.Background(), "plan-unknown"); err == nil {
		t.Fatalf("expected Check(unknown reviewID) to fail")
	}
	if err := gate.Cancel(context.Background(), ""); err == nil {
		t.Fatalf("expected Cancel(empty reviewID) to fail")
	}
	if err := gate.Cancel(context.Background(), "plan-unknown"); err == nil {
		t.Fatalf("expected Cancel(unknown reviewID) to fail")
	}
	if err := gate.Cancel(context.Background(), plan.ID); err == nil {
		t.Fatalf("expected Cancel(review without submit) to fail")
	}
}

func newTestStoreWithPlan(t *testing.T) (core.Store, *core.TaskPlan) {
	t.Helper()

	store, err := storesqlite.New(":memory:")
	if err != nil {
		t.Fatalf("storesqlite.New(:memory:) error = %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})

	project := &core.Project{
		ID:       "proj-review-local",
		Name:     "review-local",
		RepoPath: t.TempDir(),
	}
	if err := store.CreateProject(project); err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}

	plan := &core.TaskPlan{
		ID:         "plan-20260301-reviewlocal",
		ProjectID:  project.ID,
		Name:       "local-review",
		Status:     core.PlanDraft,
		WaitReason: core.WaitNone,
		FailPolicy: core.FailBlock,
	}
	if err := store.CreateTaskPlan(plan); err != nil {
		t.Fatalf("CreateTaskPlan() error = %v", err)
	}

	return store, plan
}
