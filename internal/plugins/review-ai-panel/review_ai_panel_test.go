package reviewaipanel

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/user/ai-workflow/internal/core"
	storesqlite "github.com/user/ai-workflow/internal/plugins/store-sqlite"
	"github.com/user/ai-workflow/internal/secretary"
)

func TestAIReviewGate_UnknownReview(t *testing.T) {
	store, _ := newTestStoreWithPlan(t)
	gate := New(store, fakePanel{})

	if err := gate.Init(context.Background()); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	if _, err := gate.Check(context.Background(), "plan-unknown"); err == nil {
		t.Fatalf("expected Check(unknown reviewID) to fail")
	}
	if err := gate.Cancel(context.Background(), "plan-unknown"); err == nil {
		t.Fatalf("expected Cancel(unknown reviewID) to fail")
	}
}

func TestAIReviewGate_CancelWinsOverAsyncError(t *testing.T) {
	store, plan := newTestStoreWithPlan(t)
	started := make(chan struct{})

	panel := fakePanel{
		run: func(ctx context.Context, _ *core.TaskPlan, _ secretary.ReviewInput) (*secretary.ReviewResult, error) {
			close(started)
			<-ctx.Done()
			return nil, errors.New("runner returned non-context error after cancellation")
		},
	}
	gate := New(store, panel)
	if err := gate.Init(context.Background()); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	if _, err := gate.Submit(context.Background(), plan); err != nil {
		t.Fatalf("Submit() error = %v", err)
	}
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("runner did not start")
	}

	if err := gate.Cancel(context.Background(), plan.ID); err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		got, err := gate.Check(context.Background(), plan.ID)
		if err == nil && got.Status == "cancelled" && got.Decision == "cancelled" {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("expected cancelled status to win after cancel+async-error race")
}

func TestAIReviewGate_SubmitPendingDuplicateAndCancelIdempotent(t *testing.T) {
	store, plan := newTestStoreWithPlan(t)
	started := make(chan struct{})
	panel := fakePanel{
		run: func(ctx context.Context, _ *core.TaskPlan, _ secretary.ReviewInput) (*secretary.ReviewResult, error) {
			close(started)
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}
	gate := New(store, panel)

	reviewID, err := gate.Submit(context.Background(), plan)
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}
	if reviewID != plan.ID {
		t.Fatalf("Submit() reviewID = %q, want %q", reviewID, plan.ID)
	}

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatalf("runner did not start")
	}

	if _, err := gate.Submit(context.Background(), plan); err == nil {
		t.Fatalf("expected duplicate Submit() to fail while review is running")
	}

	pending, err := gate.Check(context.Background(), reviewID)
	if err != nil {
		t.Fatalf("Check() while running error = %v", err)
	}
	if pending.Status != "pending" {
		t.Fatalf("Check().Status = %q, want pending", pending.Status)
	}
	if pending.Decision != "pending" {
		t.Fatalf("Check().Decision = %q, want pending", pending.Decision)
	}

	if err := gate.Cancel(context.Background(), reviewID); err != nil {
		t.Fatalf("first Cancel() error = %v", err)
	}
	if err := gate.Cancel(context.Background(), reviewID); err != nil {
		t.Fatalf("second Cancel() should be idempotent, got error = %v", err)
	}

	cancelled, err := gate.Check(context.Background(), reviewID)
	if err != nil {
		t.Fatalf("Check() after cancel error = %v", err)
	}
	if cancelled.Status != "cancelled" {
		t.Fatalf("cancelled status = %q, want cancelled", cancelled.Status)
	}
	if cancelled.Decision != "cancelled" {
		t.Fatalf("cancelled decision = %q, want cancelled", cancelled.Decision)
	}
}

func TestAIReviewGate_CheckCompletedStatusMapping(t *testing.T) {
	tests := []struct {
		name         string
		waitReason   core.WaitReason
		verdict      string
		planStatus   core.TaskPlanStatus
		wantStatus   string
		wantDecision string
	}{
		{
			name:         "approved requires final approval",
			waitReason:   core.WaitFinalApproval,
			verdict:      "approve",
			planStatus:   core.PlanWaitingHuman,
			wantStatus:   "approved",
			wantDecision: "approve",
		},
		{
			name:         "rejected requires feedback required",
			waitReason:   core.WaitFeedbackReq,
			verdict:      "escalate",
			planStatus:   core.PlanWaitingHuman,
			wantStatus:   "rejected",
			wantDecision: "escalate",
		},
		{
			name:         "feedback required overrides stale fix verdict",
			waitReason:   core.WaitFeedbackReq,
			verdict:      "fix",
			planStatus:   core.PlanWaitingHuman,
			wantStatus:   "rejected",
			wantDecision: "escalate",
		},
		{
			name:         "changes requested from fix verdict",
			waitReason:   core.WaitNone,
			verdict:      "fix",
			planStatus:   core.PlanReviewing,
			wantStatus:   "changes_requested",
			wantDecision: "fix",
		},
		{
			name:         "cancelled",
			waitReason:   core.WaitNone,
			verdict:      "cancelled",
			planStatus:   core.PlanReviewing,
			wantStatus:   "cancelled",
			wantDecision: "cancelled",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			store, plan := newTestStoreWithPlan(t)
			done := make(chan struct{})
			panel := fakePanel{
				run: func(_ context.Context, _ *core.TaskPlan, _ secretary.ReviewInput) (*secretary.ReviewResult, error) {
					updated := *plan
					updated.Status = tc.planStatus
					updated.WaitReason = tc.waitReason
					if err := store.SaveTaskPlan(&updated); err != nil {
						t.Fatalf("SaveTaskPlan() error = %v", err)
					}
					if tc.verdict != "" {
						if err := store.SaveReviewRecord(&core.ReviewRecord{
							PlanID:   plan.ID,
							Round:    1,
							Reviewer: "aggregator",
							Verdict:  tc.verdict,
						}); err != nil {
							t.Fatalf("SaveReviewRecord() error = %v", err)
						}
					}
					close(done)
					return &secretary.ReviewResult{Plan: &updated}, nil
				},
			}
			gate := New(store, panel)

			if _, err := gate.Submit(context.Background(), plan); err != nil {
				t.Fatalf("Submit() error = %v", err)
			}

			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatalf("review run did not complete")
			}

			got, err := gate.Check(context.Background(), plan.ID)
			if err != nil {
				t.Fatalf("Check() error = %v", err)
			}
			if got.Status != tc.wantStatus {
				t.Fatalf("Check().Status = %q, want %q", got.Status, tc.wantStatus)
			}
			if got.Decision != tc.wantDecision {
				t.Fatalf("Check().Decision = %q, want %q", got.Decision, tc.wantDecision)
			}
			for _, verdict := range got.Verdicts {
				if verdict.Reviewer == gateReviewer {
					t.Fatalf("Check().Verdicts should not include gate reviewer marker %q", gateReviewer)
				}
			}
		})
	}
}

type fakePanel struct {
	run func(ctx context.Context, plan *core.TaskPlan, input secretary.ReviewInput) (*secretary.ReviewResult, error)
}

func (f fakePanel) Run(ctx context.Context, plan *core.TaskPlan, input secretary.ReviewInput) (*secretary.ReviewResult, error) {
	if f.run == nil {
		return &secretary.ReviewResult{Plan: plan, Decision: secretary.DecisionApprove, Round: 1}, nil
	}
	return f.run(ctx, plan, input)
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
		ID:       "proj-review-ai-panel",
		Name:     "review-ai-panel",
		RepoPath: t.TempDir(),
	}
	if err := store.CreateProject(project); err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}

	plan := &core.TaskPlan{
		ID:         "plan-20260301-reviewaipanel",
		ProjectID:  project.ID,
		Name:       "ai-panel-review",
		Status:     core.PlanDraft,
		WaitReason: core.WaitNone,
		FailPolicy: core.FailBlock,
	}
	if err := store.CreateTaskPlan(plan); err != nil {
		t.Fatalf("CreateTaskPlan() error = %v", err)
	}

	return store, plan
}
