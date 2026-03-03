package secretary

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yoke233/ai-workflow/internal/core"
	storesqlite "github.com/yoke233/ai-workflow/internal/plugins/store-sqlite"
)

func TestScheduler_StartPlanAndProgression(t *testing.T) {
	store := newSchedulerTestStore(t)
	defer store.Close()

	project := mustCreateSchedulerProject(t, store, "proj-scheduler-normal")
	issues := mustCreateIssueSessionWithItems(t, store, project.ID, "session-normal", core.FailBlock, []core.Issue{
		newIssue("task-a", "A", nil),
		newIssue("task-b", "B", []string{"task-a"}),
	})

	runner := &schedulerRunner{}
	s := NewDepScheduler(store, nil, runner.Run, nil, 2)

	if err := s.StartPlan(context.Background(), issues); err != nil {
		t.Fatalf("StartPlan() error = %v", err)
	}

	issueA := waitIssueStatus(t, store, "task-a", core.IssueStatusExecuting, 2*time.Second)
	if issueA.PipelineID == "" {
		t.Fatalf("expected task-a pipeline id assigned")
	}

	if err := s.OnEvent(context.Background(), core.Event{Type: core.EventPipelineDone, PipelineID: issueA.PipelineID, Timestamp: time.Now()}); err != nil {
		t.Fatalf("OnEvent(done A) error = %v", err)
	}

	issueB := waitIssueStatus(t, store, "task-b", core.IssueStatusExecuting, 2*time.Second)
	if issueB.PipelineID == "" {
		t.Fatalf("expected task-b pipeline id assigned")
	}

	if err := s.OnEvent(context.Background(), core.Event{Type: core.EventPipelineDone, PipelineID: issueB.PipelineID, Timestamp: time.Now()}); err != nil {
		t.Fatalf("OnEvent(done B) error = %v", err)
	}

	waitIssueStatus(t, store, "task-a", core.IssueStatusDone, 2*time.Second)
	waitIssueStatus(t, store, "task-b", core.IssueStatusDone, 2*time.Second)
	waitRunnerCalls(t, runner, 2, 2*time.Second)
}

func TestDepScheduler_StartPlan_IdempotentForManagedPlan(t *testing.T) {
	store := newSchedulerTestStore(t)
	defer store.Close()

	project := mustCreateSchedulerProject(t, store, "proj-scheduler-idempotent")
	issues := mustCreateIssueSessionWithItems(t, store, project.ID, "session-idempotent", core.FailBlock, []core.Issue{
		newIssue("task-a", "A", nil),
	})

	runner := &schedulerRunner{}
	s := NewDepScheduler(store, nil, runner.Run, nil, 1)

	if err := s.StartPlan(context.Background(), issues); err != nil {
		t.Fatalf("StartPlan(first) error = %v", err)
	}
	issueA := waitIssueStatus(t, store, "task-a", core.IssueStatusExecuting, 2*time.Second)
	firstPipelineID := issueA.PipelineID
	if firstPipelineID == "" {
		t.Fatalf("expected task-a pipeline id assigned")
	}

	if err := s.StartPlan(context.Background(), issues); err != nil {
		t.Fatalf("StartPlan(second) error = %v", err)
	}

	issueAAfter := waitIssueStatus(t, store, "task-a", core.IssueStatusExecuting, 2*time.Second)
	if issueAAfter.PipelineID != firstPipelineID {
		t.Fatalf("pipeline id changed on idempotent start: got %q want %q", issueAAfter.PipelineID, firstPipelineID)
	}
	waitRunnerCalls(t, runner, 1, 2*time.Second)
}

func TestDepScheduler_TrackerWarning_DoesNotBlockMainFlow(t *testing.T) {
	store := newSchedulerTestStore(t)
	defer store.Close()

	project := mustCreateSchedulerProject(t, store, "proj-scheduler-tracker-warning")
	issues := mustCreateIssueSessionWithItems(t, store, project.ID, "session-tracker-warning", core.FailBlock, []core.Issue{
		newIssue("task-a", "A", nil),
	})

	runner := &schedulerRunner{}
	s := NewDepScheduler(store, nil, runner.Run, &warningTracker{}, 1)
	if err := s.StartPlan(context.Background(), issues); err != nil {
		t.Fatalf("StartPlan() error = %v", err)
	}

	issueA := waitIssueStatus(t, store, "task-a", core.IssueStatusExecuting, 2*time.Second)
	if issueA.ExternalID != "" {
		t.Fatalf("tracker warning path should not assign external id, got %q", issueA.ExternalID)
	}

	if err := s.OnEvent(context.Background(), core.Event{
		Type:       core.EventPipelineDone,
		PipelineID: issueA.PipelineID,
		Timestamp:  time.Now(),
	}); err != nil {
		t.Fatalf("OnEvent(done A) error = %v", err)
	}

	waitIssueStatus(t, store, "task-a", core.IssueStatusDone, 2*time.Second)
}

func TestScheduler_FailPolicyBlock(t *testing.T) {
	store := newSchedulerTestStore(t)
	defer store.Close()

	project := mustCreateSchedulerProject(t, store, "proj-scheduler-block")
	issues := mustCreateIssueSessionWithItems(t, store, project.ID, "session-block", core.FailBlock, []core.Issue{
		newIssue("task-a", "A", nil),
		newIssue("task-b", "B", []string{"task-a"}),
		newIssue("task-c", "C", []string{"task-b"}),
	})

	s := NewDepScheduler(store, nil, (&schedulerRunner{}).Run, nil, 1)
	if err := s.StartPlan(context.Background(), issues); err != nil {
		t.Fatalf("StartPlan() error = %v", err)
	}

	issueA := waitIssueStatus(t, store, "task-a", core.IssueStatusExecuting, 2*time.Second)
	if err := s.OnEvent(context.Background(), core.Event{Type: core.EventPipelineFailed, PipelineID: issueA.PipelineID, Timestamp: time.Now(), Error: "boom"}); err != nil {
		t.Fatalf("OnEvent(failed A) error = %v", err)
	}

	waitIssueStatus(t, store, "task-a", core.IssueStatusFailed, 2*time.Second)
	issueB := waitIssueStatus(t, store, "task-b", core.IssueStatusFailed, 2*time.Second)
	issueC := waitIssueStatus(t, store, "task-c", core.IssueStatusFailed, 2*time.Second)
	if issueB.PipelineID != "" || issueC.PipelineID != "" {
		t.Fatalf("blocked downstream should not be dispatched, got task-b=%q task-c=%q", issueB.PipelineID, issueC.PipelineID)
	}
}

func TestScheduler_FailPolicySkip(t *testing.T) {
	store := newSchedulerTestStore(t)
	defer store.Close()

	project := mustCreateSchedulerProject(t, store, "proj-scheduler-skip")
	issues := mustCreateIssueSessionWithItems(t, store, project.ID, "session-skip", core.FailSkip, []core.Issue{
		newIssue("task-a", "A", nil),
		newIssue("task-x", "X", nil),
		newIssue("task-b", "B", []string{"task-a"}),
		newIssue("task-c", "C", []string{"task-a", "task-x"}),
	})

	s := NewDepScheduler(store, nil, (&schedulerRunner{}).Run, nil, 3)
	if err := s.StartPlan(context.Background(), issues); err != nil {
		t.Fatalf("StartPlan() error = %v", err)
	}

	issueA := waitIssueStatus(t, store, "task-a", core.IssueStatusExecuting, 2*time.Second)
	issueX := waitIssueStatus(t, store, "task-x", core.IssueStatusExecuting, 2*time.Second)

	if err := s.OnEvent(context.Background(), core.Event{Type: core.EventPipelineDone, PipelineID: issueX.PipelineID, Timestamp: time.Now()}); err != nil {
		t.Fatalf("OnEvent(done X) error = %v", err)
	}
	if err := s.OnEvent(context.Background(), core.Event{Type: core.EventPipelineFailed, PipelineID: issueA.PipelineID, Timestamp: time.Now(), Error: "boom"}); err != nil {
		t.Fatalf("OnEvent(failed A) error = %v", err)
	}

	waitIssueStatus(t, store, "task-a", core.IssueStatusFailed, 2*time.Second)
	issueB := waitIssueStatus(t, store, "task-b", core.IssueStatusExecuting, 2*time.Second)
	issueC := waitIssueStatus(t, store, "task-c", core.IssueStatusExecuting, 2*time.Second)
	if issueB.PipelineID == "" || issueC.PipelineID == "" {
		t.Fatalf("downstream issues should be dispatched under skip policy, got task-b=%q task-c=%q", issueB.PipelineID, issueC.PipelineID)
	}

	if err := s.OnEvent(context.Background(), core.Event{Type: core.EventPipelineDone, PipelineID: issueB.PipelineID, Timestamp: time.Now()}); err != nil {
		t.Fatalf("OnEvent(done B) error = %v", err)
	}
	if err := s.OnEvent(context.Background(), core.Event{Type: core.EventPipelineDone, PipelineID: issueC.PipelineID, Timestamp: time.Now()}); err != nil {
		t.Fatalf("OnEvent(done C) error = %v", err)
	}
	waitIssueStatus(t, store, "task-b", core.IssueStatusDone, 2*time.Second)
	waitIssueStatus(t, store, "task-c", core.IssueStatusDone, 2*time.Second)
}

func TestDepScheduler_FailPolicySkip_HardByDefault(t *testing.T) {
	store := newSchedulerTestStore(t)
	defer store.Close()

	project := mustCreateSchedulerProject(t, store, "proj-scheduler-skip-hard-default")
	issues := mustCreateIssueSessionWithItems(t, store, project.ID, "session-skip-hard-default", core.FailSkip, []core.Issue{
		newIssue("task-a", "A", nil),
		newIssue("task-x", "X", nil),
		newIssue("task-c", "C", []string{"task-a", "task-x"}),
	})

	s := NewDepScheduler(store, nil, (&schedulerRunner{}).Run, nil, 3)
	if err := s.StartPlan(context.Background(), issues); err != nil {
		t.Fatalf("StartPlan() error = %v", err)
	}

	issueA := waitIssueStatus(t, store, "task-a", core.IssueStatusExecuting, 2*time.Second)
	issueX := waitIssueStatus(t, store, "task-x", core.IssueStatusExecuting, 2*time.Second)

	if err := s.OnEvent(context.Background(), core.Event{Type: core.EventPipelineDone, PipelineID: issueX.PipelineID, Timestamp: time.Now()}); err != nil {
		t.Fatalf("OnEvent(done X) error = %v", err)
	}
	if err := s.OnEvent(context.Background(), core.Event{Type: core.EventPipelineFailed, PipelineID: issueA.PipelineID, Timestamp: time.Now(), Error: "boom"}); err != nil {
		t.Fatalf("OnEvent(failed A) error = %v", err)
	}

	waitIssueStatus(t, store, "task-a", core.IssueStatusFailed, 2*time.Second)
	issueC := waitIssueStatus(t, store, "task-c", core.IssueStatusExecuting, 2*time.Second)
	if issueC.PipelineID == "" {
		t.Fatalf("task-c should be dispatched under current skip semantics")
	}
}

func TestDepScheduler_FailPolicySkip_WeakEdgeRequiresOtherUnfailedParent(t *testing.T) {
	store := newSchedulerTestStore(t)
	defer store.Close()

	project := mustCreateSchedulerProject(t, store, "proj-scheduler-skip-weak-guard")
	issues := mustCreateIssueSessionWithItems(t, store, project.ID, "session-skip-weak-guard", core.FailSkip, []core.Issue{
		newIssue("task-a", "A", nil),
		newIssue("task-b", "B", []string{"task-a"}),
	})

	s := NewDepScheduler(store, nil, (&schedulerRunner{}).Run, nil, 2)
	if err := s.StartPlan(context.Background(), issues); err != nil {
		t.Fatalf("StartPlan() error = %v", err)
	}

	issueA := waitIssueStatus(t, store, "task-a", core.IssueStatusExecuting, 2*time.Second)
	if err := s.OnEvent(context.Background(), core.Event{Type: core.EventPipelineFailed, PipelineID: issueA.PipelineID, Timestamp: time.Now(), Error: "boom"}); err != nil {
		t.Fatalf("OnEvent(failed A) error = %v", err)
	}

	waitIssueStatus(t, store, "task-a", core.IssueStatusFailed, 2*time.Second)
	issueB := waitIssueStatus(t, store, "task-b", core.IssueStatusExecuting, 2*time.Second)
	if issueB.PipelineID == "" {
		t.Fatalf("task-b should be dispatched under current skip semantics")
	}
}

func TestScheduler_FailPolicyHuman(t *testing.T) {
	store := newSchedulerTestStore(t)
	defer store.Close()

	project := mustCreateSchedulerProject(t, store, "proj-scheduler-human")
	sessionID := "session-human"
	issues := mustCreateIssueSessionWithItems(t, store, project.ID, sessionID, core.FailHuman, []core.Issue{
		newIssue("task-a", "A", nil),
		newIssue("task-b", "B", []string{"task-a"}),
	})

	s := NewDepScheduler(store, nil, (&schedulerRunner{}).Run, nil, 2)
	if err := s.StartPlan(context.Background(), issues); err != nil {
		t.Fatalf("StartPlan() error = %v", err)
	}

	issueA := waitIssueStatus(t, store, "task-a", core.IssueStatusExecuting, 2*time.Second)
	if err := s.OnEvent(context.Background(), core.Event{Type: core.EventPipelineFailed, PipelineID: issueA.PipelineID, Timestamp: time.Now(), Error: "need human"}); err != nil {
		t.Fatalf("OnEvent(failed A) error = %v", err)
	}

	waitIssueStatus(t, store, "task-a", core.IssueStatusFailed, 2*time.Second)
	issueB := waitIssueStatus(t, store, "task-b", core.IssueStatusQueued, 2*time.Second)
	if issueB.PipelineID != "" {
		t.Fatalf("task-b should not be dispatched under human policy, got pipeline=%q", issueB.PipelineID)
	}
	waitSessionHalted(t, s, makeSessionID(project.ID, sessionID), 2*time.Second)
}

func TestScheduler_RecoverExecutingPlans_DispatchesReadyTasks(t *testing.T) {
	store := newSchedulerTestStore(t)
	defer store.Close()

	project := mustCreateSchedulerProject(t, store, "proj-scheduler-recovery")
	mustCreateIssueSessionWithItems(t, store, project.ID, "session-recovery", core.FailBlock, []core.Issue{
		{
			ID:       "task-b",
			Title:    "B",
			Body:     "B",
			Status:   core.IssueStatusReady,
			Template: "standard",
		},
	})

	runner := &schedulerRunner{}
	s := NewDepScheduler(store, nil, runner.Run, nil, 2)
	if err := s.RecoverExecutingPlans(context.Background()); err != nil {
		t.Fatalf("RecoverExecutingPlans() error = %v", err)
	}

	issueB := waitIssueStatus(t, store, "task-b", core.IssueStatusExecuting, 2*time.Second)
	if issueB.PipelineID == "" {
		t.Fatalf("expected task-b dispatched after recovery")
	}
	waitRunnerCalls(t, runner, 1, 2*time.Second)
}

func TestDepScheduler_RecoverExecutingPlans_ReplaysPipelineDoneFromStore(t *testing.T) {
	store := newSchedulerTestStore(t)
	defer store.Close()

	project := mustCreateSchedulerProject(t, store, "proj-scheduler-recover-done")
	mustCreateIssueSessionWithItems(t, store, project.ID, "session-recover-done", core.FailBlock, []core.Issue{
		{
			ID:         "task-a",
			Title:      "A",
			Body:       "A",
			Status:     core.IssueStatusExecuting,
			PipelineID: "pipeline-recover-done",
			Template:   "standard",
		},
		{
			ID:        "task-b",
			Title:     "B",
			Body:      "B",
			DependsOn: []string{"task-a"},
			Status:    core.IssueStatusQueued,
			Template:  "standard",
		},
	})

	if err := store.SavePipeline(&core.Pipeline{
		ID:        "pipeline-recover-done",
		ProjectID: project.ID,
		Name:      "pipeline-recover-done",
		Status:    core.StatusDone,
		IssueID:   "task-a",
	}); err != nil {
		t.Fatalf("SavePipeline(done) error = %v", err)
	}

	runner := &schedulerRunner{}
	s := NewDepScheduler(store, nil, runner.Run, nil, 2)
	if err := s.RecoverExecutingPlans(context.Background()); err != nil {
		t.Fatalf("RecoverExecutingPlans() error = %v", err)
	}

	waitIssueStatus(t, store, "task-a", core.IssueStatusDone, 2*time.Second)
	issueB := waitIssueStatus(t, store, "task-b", core.IssueStatusExecuting, 2*time.Second)
	if issueB.PipelineID == "" {
		t.Fatalf("expected task-b dispatched after replaying pipeline done")
	}
}

func TestDepScheduler_RecoverExecutingPlans_ReplaysPipelineFailedFromStore(t *testing.T) {
	store := newSchedulerTestStore(t)
	defer store.Close()

	project := mustCreateSchedulerProject(t, store, "proj-scheduler-recover-failed")
	mustCreateIssueSessionWithItems(t, store, project.ID, "session-recover-failed", core.FailBlock, []core.Issue{
		{
			ID:         "task-a",
			Title:      "A",
			Body:       "A",
			Status:     core.IssueStatusExecuting,
			PipelineID: "pipeline-recover-failed",
			Template:   "standard",
		},
		{
			ID:        "task-b",
			Title:     "B",
			Body:      "B",
			DependsOn: []string{"task-a"},
			Status:    core.IssueStatusQueued,
			Template:  "standard",
		},
	})

	if err := store.SavePipeline(&core.Pipeline{
		ID:        "pipeline-recover-failed",
		ProjectID: project.ID,
		Name:      "pipeline-recover-failed",
		Status:    core.StatusFailed,
		IssueID:   "task-a",
	}); err != nil {
		t.Fatalf("SavePipeline(failed) error = %v", err)
	}

	s := NewDepScheduler(store, nil, (&schedulerRunner{}).Run, nil, 2)
	if err := s.RecoverExecutingPlans(context.Background()); err != nil {
		t.Fatalf("RecoverExecutingPlans() error = %v", err)
	}

	waitIssueStatus(t, store, "task-a", core.IssueStatusFailed, 2*time.Second)
	waitIssueStatus(t, store, "task-b", core.IssueStatusFailed, 2*time.Second)
}

func TestDepScheduler_GlobalReadyDispatch_AvoidsCrossPlanStarvation(t *testing.T) {
	store := newSchedulerTestStore(t)
	defer store.Close()

	project := mustCreateSchedulerProject(t, store, "proj-scheduler-cross-plan")
	issuesA := mustCreateIssueSessionWithItems(t, store, project.ID, "session-a", core.FailBlock, []core.Issue{
		newIssue("task-a-1", "A1", nil),
		newIssue("task-a-2", "A2", []string{"task-a-1"}),
	})
	issuesB := mustCreateIssueSessionWithItems(t, store, project.ID, "session-b", core.FailBlock, []core.Issue{
		newIssue("task-b-1", "B1", nil),
	})

	runner := &schedulerRunner{}
	s := NewDepScheduler(store, nil, runner.Run, nil, 1)
	if err := s.StartPlan(context.Background(), issuesA); err != nil {
		t.Fatalf("StartPlan(sessionA) error = %v", err)
	}
	if err := s.StartPlan(context.Background(), issuesB); err != nil {
		t.Fatalf("StartPlan(sessionB) error = %v", err)
	}

	issueA1 := waitIssueStatus(t, store, "task-a-1", core.IssueStatusExecuting, 2*time.Second)
	waitIssueStatus(t, store, "task-b-1", core.IssueStatusReady, 2*time.Second)

	if err := s.OnEvent(context.Background(), core.Event{Type: core.EventPipelineDone, PipelineID: issueA1.PipelineID, Timestamp: time.Now()}); err != nil {
		t.Fatalf("OnEvent(done A1) error = %v", err)
	}

	issueB1 := waitIssueStatus(t, store, "task-b-1", core.IssueStatusExecuting, 2*time.Second)
	if issueB1.PipelineID == "" {
		t.Fatalf("expected task-b-1 dispatched after slot release")
	}
	issueA2 := waitIssueStatus(t, store, "task-a-2", core.IssueStatusReady, 2*time.Second)
	if issueA2.PipelineID != "" {
		t.Fatalf("task-a-2 should wait while task-b-1 is running, got pipeline=%q", issueA2.PipelineID)
	}

	if err := s.OnEvent(context.Background(), core.Event{Type: core.EventPipelineDone, PipelineID: issueB1.PipelineID, Timestamp: time.Now()}); err != nil {
		t.Fatalf("OnEvent(done B1) error = %v", err)
	}
	waitIssueStatus(t, store, "task-a-2", core.IssueStatusExecuting, 2*time.Second)
}

func TestDepScheduler_OnEvent_PersistenceFailureRetainsTerminalEventForRetry(t *testing.T) {
	baseStore := newSchedulerTestStore(t)
	defer baseStore.Close()
	store := &flakyIssueSaveStore{
		Store:       baseStore,
		failIssueID: "task-a",
		failStatus:  core.IssueStatusDone,
	}

	project := mustCreateSchedulerProject(t, store, "proj-scheduler-event-retry")
	issues := mustCreateIssueSessionWithItems(t, store, project.ID, "session-event-retry", core.FailBlock, []core.Issue{
		newIssue("task-a", "A", nil),
	})

	s := NewDepScheduler(store, nil, (&schedulerRunner{}).Run, nil, 1)
	if err := s.StartPlan(context.Background(), issues); err != nil {
		t.Fatalf("StartPlan() error = %v", err)
	}

	issueA := waitIssueStatus(t, store, "task-a", core.IssueStatusExecuting, 2*time.Second)
	err := s.OnEvent(context.Background(), core.Event{Type: core.EventPipelineDone, PipelineID: issueA.PipelineID, Timestamp: time.Now()})
	if err == nil {
		t.Fatalf("OnEvent(first done) should fail due to injected SaveIssue error")
	}
	if !errors.Is(err, errInjectedIssueSave) {
		t.Fatalf("OnEvent(first done) error = %v, want %v", err, errInjectedIssueSave)
	}

	if _, ok := s.pipelineIndex[issueA.PipelineID]; !ok {
		t.Fatalf("pipeline index removed before persistence succeeded")
	}
	if got := len(s.sem); got != 1 {
		t.Fatalf("slot should remain occupied after failed persistence, got len(sem)=%d", got)
	}

	if err := s.OnEvent(context.Background(), core.Event{Type: core.EventPipelineDone, PipelineID: issueA.PipelineID, Timestamp: time.Now()}); err != nil {
		t.Fatalf("OnEvent(second done) error = %v", err)
	}
	waitIssueStatus(t, store, "task-a", core.IssueStatusDone, 2*time.Second)
	if _, ok := s.pipelineIndex[issueA.PipelineID]; ok {
		t.Fatalf("pipeline index should be removed after successful retry")
	}
	if got := len(s.sem); got != 0 {
		t.Fatalf("slot should be released after successful retry, got len(sem)=%d", got)
	}
}

func TestDepScheduler_EmitsPlanScopedLifecycleEvents(t *testing.T) {
	store := newSchedulerTestStore(t)
	defer store.Close()

	project := mustCreateSchedulerProject(t, store, "proj-scheduler-events-done")
	issues := mustCreateIssueSessionWithItems(t, store, project.ID, "session-events-done", core.FailBlock, []core.Issue{
		newIssue("task-a", "A", nil),
	})

	bus := &recordingSchedulerBus{}
	runner := &schedulerRunner{}
	s := NewDepScheduler(store, bus, runner.Run, nil, 1)

	if err := s.StartPlan(context.Background(), issues); err != nil {
		t.Fatalf("StartPlan() error = %v", err)
	}
	issueA := waitIssueStatus(t, store, "task-a", core.IssueStatusExecuting, 2*time.Second)
	if issueA.PipelineID == "" {
		t.Fatalf("expected running issue pipeline id")
	}

	if !bus.HasEvent(core.EventIssueQueued, "task-a") {
		t.Fatalf("expected %q event with issue_id=%s", core.EventIssueQueued, "task-a")
	}
	if !bus.HasEvent(core.EventIssueReady, "task-a") {
		t.Fatalf("expected %q event with issue_id=%s", core.EventIssueReady, "task-a")
	}
	if !bus.HasEvent(core.EventIssueExecuting, "task-a") {
		t.Fatalf("expected %q event with issue_id=%s", core.EventIssueExecuting, "task-a")
	}

	if err := s.OnEvent(context.Background(), core.Event{
		Type:       core.EventPipelineDone,
		PipelineID: issueA.PipelineID,
		Timestamp:  time.Now(),
	}); err != nil {
		t.Fatalf("OnEvent(done A) error = %v", err)
	}
	waitIssueStatus(t, store, "task-a", core.IssueStatusDone, 2*time.Second)

	if !bus.HasEvent(core.EventIssueDone, "task-a") {
		t.Fatalf("expected %q event with issue_id=%s", core.EventIssueDone, "task-a")
	}

	issueRunningEvt, ok := bus.FirstEvent(core.EventIssueExecuting, "task-a")
	if !ok {
		t.Fatalf("missing %q event with issue_id=%s", core.EventIssueExecuting, "task-a")
	}
	if issueRunningEvt.PipelineID == "" {
		t.Fatalf("%q should include pipeline_id", core.EventIssueExecuting)
	}
	if issueRunningEvt.Data["issue_status"] != string(core.IssueStatusExecuting) {
		t.Fatalf("%q should include issue_status=%s, got %+v", core.EventIssueExecuting, core.IssueStatusExecuting, issueRunningEvt.Data)
	}

	issueDoneEvt, ok := bus.FirstEvent(core.EventIssueDone, "task-a")
	if !ok {
		t.Fatalf("missing %q event with issue_id=%s", core.EventIssueDone, "task-a")
	}
	if issueDoneEvt.Data["issue_status"] != string(core.IssueStatusDone) {
		t.Fatalf("%q should include issue_status=%s, got %+v", core.EventIssueDone, core.IssueStatusDone, issueDoneEvt.Data)
	}

	for _, evt := range bus.Events() {
		if !core.IsIssueScopedEvent(evt.Type) {
			continue
		}
		if evt.IssueID == "" {
			t.Fatalf("event %q should carry issue_id, got %+v", evt.Type, evt)
		}
	}
}

func TestDepScheduler_EmitsPlanWaitingHumanAndTaskFailedEvents(t *testing.T) {
	store := newSchedulerTestStore(t)
	defer store.Close()

	project := mustCreateSchedulerProject(t, store, "proj-scheduler-events-human")
	issues := mustCreateIssueSessionWithItems(t, store, project.ID, "session-events-human", core.FailHuman, []core.Issue{
		newIssue("task-a", "A", nil),
		newIssue("task-b", "B", []string{"task-a"}),
	})

	bus := &recordingSchedulerBus{}
	s := NewDepScheduler(store, bus, (&schedulerRunner{}).Run, nil, 1)

	if err := s.StartPlan(context.Background(), issues); err != nil {
		t.Fatalf("StartPlan() error = %v", err)
	}
	issueA := waitIssueStatus(t, store, "task-a", core.IssueStatusExecuting, 2*time.Second)
	if err := s.OnEvent(context.Background(), core.Event{
		Type:       core.EventPipelineFailed,
		PipelineID: issueA.PipelineID,
		Timestamp:  time.Now(),
		Error:      "need human",
	}); err != nil {
		t.Fatalf("OnEvent(failed A) error = %v", err)
	}
	waitIssueStatus(t, store, "task-a", core.IssueStatusFailed, 2*time.Second)
	waitIssueStatus(t, store, "task-b", core.IssueStatusQueued, 2*time.Second)

	if !bus.HasEvent(core.EventIssueFailed, "task-a") {
		t.Fatalf("expected %q event with issue_id=%s", core.EventIssueFailed, "task-a")
	}
	if bus.HasEvent(core.EventIssueExecuting, "task-b") {
		t.Fatalf("task-b should not emit %q under fail-human halt", core.EventIssueExecuting)
	}

	taskFailedEvt, ok := bus.FirstEvent(core.EventIssueFailed, "task-a")
	if !ok {
		t.Fatalf("missing %q event with issue_id=%s", core.EventIssueFailed, "task-a")
	}
	if taskFailedEvt.Error != "need human" {
		t.Fatalf("%q should include error message, got %+v", core.EventIssueFailed, taskFailedEvt)
	}
}

func TestDepScheduler_StartPlanRejectsInvalidState(t *testing.T) {
	store := newSchedulerTestStore(t)
	defer store.Close()

	s := NewDepScheduler(store, nil, (&schedulerRunner{}).Run, nil, 1)
	err := s.StartPlan(context.Background(), map[string]string{"legacy": "unsupported"})
	if err == nil {
		t.Fatalf("StartPlan() expected error for unsupported payload")
	}
	if !strings.Contains(err.Error(), "unsupported legacy start payload type") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSchedulerDefaultStageConfig_DefaultAgentAndE2E(t *testing.T) {
	for _, stageID := range []core.StageID{
		core.StageRequirements,
		core.StageCodeReview,
	} {
		cfg := schedulerDefaultStageConfig(stageID)
		if cfg.Agent != "codex" {
			t.Fatalf("stage %s should default to codex, got %q", stageID, cfg.Agent)
		}
	}

	for _, stageID := range []core.StageID{
		core.StageImplement,
		core.StageFixup,
		core.StageE2ETest,
	} {
		cfg := schedulerDefaultStageConfig(stageID)
		if cfg.Agent != "codex" {
			t.Fatalf("stage %s should default to codex, got %q", stageID, cfg.Agent)
		}
	}

	cfg := schedulerDefaultStageConfig(core.StageE2ETest)
	if cfg.Timeout != 15*time.Minute {
		t.Fatalf("e2e_test timeout mismatch, got %s want %s", cfg.Timeout, 15*time.Minute)
	}
}

type schedulerRunner struct {
	mu    sync.Mutex
	calls []string
}

type recordingSchedulerBus struct {
	mu     sync.Mutex
	events []core.Event
}

func (b *recordingSchedulerBus) Subscribe() chan core.Event {
	return make(chan core.Event, 1)
}

func (b *recordingSchedulerBus) Unsubscribe(ch chan core.Event) {
	close(ch)
}

func (b *recordingSchedulerBus) Publish(evt core.Event) {
	b.mu.Lock()
	defer b.mu.Unlock()

	clone := evt
	if len(evt.Data) > 0 {
		clone.Data = make(map[string]string, len(evt.Data))
		for k, v := range evt.Data {
			clone.Data[k] = v
		}
	}
	b.events = append(b.events, clone)
}

func (b *recordingSchedulerBus) HasEvent(eventType core.EventType, issueID string) bool {
	for _, evt := range b.Events() {
		if evt.Type == eventType && evt.IssueID == issueID {
			return true
		}
	}
	return false
}

func (b *recordingSchedulerBus) FirstEvent(eventType core.EventType, issueID string) (core.Event, bool) {
	for _, evt := range b.Events() {
		if evt.Type == eventType && evt.IssueID == issueID {
			return evt, true
		}
	}
	return core.Event{}, false
}

func (b *recordingSchedulerBus) Events() []core.Event {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]core.Event, len(b.events))
	copy(out, b.events)
	return out
}

func (r *schedulerRunner) Run(_ context.Context, pipelineID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, pipelineID)
	return nil
}

func (r *schedulerRunner) CallCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.calls)
}

func newSchedulerTestStore(t *testing.T) core.Store {
	t.Helper()
	s, err := storesqlite.New(":memory:")
	if err != nil {
		t.Fatalf("storesqlite.New() error = %v", err)
	}
	return s
}

func mustCreateSchedulerProject(t *testing.T, store core.Store, id string) *core.Project {
	t.Helper()
	p := &core.Project{
		ID:       id,
		Name:     id,
		RepoPath: t.TempDir(),
	}
	if err := store.CreateProject(p); err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	return p
}

func mustCreateIssueSessionWithItems(
	t *testing.T,
	store core.Store,
	projectID string,
	sessionID string,
	failPolicy core.FailurePolicy,
	items []core.Issue,
) []*core.Issue {
	t.Helper()

	trimmedSessionID := strings.TrimSpace(sessionID)
	if trimmedSessionID != "" {
		err := store.CreateChatSession(&core.ChatSession{
			ID:        trimmedSessionID,
			ProjectID: projectID,
			Messages:  []core.ChatMessage{},
		})
		if err != nil && !strings.Contains(strings.ToLower(err.Error()), "unique") {
			t.Fatalf("CreateChatSession(%s) error = %v", trimmedSessionID, err)
		}
	}

	issues := make([]*core.Issue, 0, len(items))
	for _, item := range items {
		issue := item
		issue.ProjectID = projectID
		issue.SessionID = trimmedSessionID
		if issue.Template == "" {
			issue.Template = "standard"
		}
		if issue.State == "" {
			issue.State = core.IssueStateOpen
		}
		if issue.Status == "" {
			issue.Status = core.IssueStatusQueued
		}
		if issue.FailPolicy == "" {
			issue.FailPolicy = failPolicy
		}
		if err := store.CreateIssue(&issue); err != nil {
			t.Fatalf("CreateIssue(%s) error = %v", issue.ID, err)
		}
		issues = append(issues, &issue)
	}
	return issues
}

func newIssue(id, title string, dependsOn []string) core.Issue {
	return core.Issue{
		ID:        id,
		Title:     title,
		Body:      title,
		DependsOn: dependsOn,
		Status:    core.IssueStatusQueued,
		State:     core.IssueStateOpen,
		Template:  "standard",
	}
}

func waitIssueStatus(t *testing.T, store core.Store, issueID string, want core.IssueStatus, timeout time.Duration) *core.Issue {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		issue, err := store.GetIssue(issueID)
		if err != nil {
			t.Fatalf("GetIssue(%s) error = %v", issueID, err)
		}
		if issue.Status == want {
			return issue
		}
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting issue %s status %q, got %q", issueID, want, issue.Status)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func waitSessionHalted(t *testing.T, s *DepScheduler, sessionID string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		s.mu.Lock()
		rs := s.sessions[sessionID]
		halted := rs != nil && rs.HaltNew
		s.mu.Unlock()
		if halted {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting session %s halt flag", sessionID)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func waitRunnerCalls(t *testing.T, runner *schedulerRunner, want int, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		got := runner.CallCount()
		if got >= want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting runner calls = %d, want %d", got, want)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

var errInjectedIssueSave = errors.New("injected save issue error")

type flakyIssueSaveStore struct {
	core.Store

	mu          sync.Mutex
	failIssueID string
	failStatus  core.IssueStatus
	failedOnce  bool
}

func (s *flakyIssueSaveStore) SaveIssue(issue *core.Issue) error {
	s.mu.Lock()
	shouldFail := !s.failedOnce &&
		issue != nil &&
		issue.ID == s.failIssueID &&
		issue.Status == s.failStatus
	if shouldFail {
		s.failedOnce = true
	}
	s.mu.Unlock()

	if shouldFail {
		return errInjectedIssueSave
	}
	return s.Store.SaveIssue(issue)
}

type warningTracker struct{}

func (w *warningTracker) Name() string { return "warning-tracker" }

func (w *warningTracker) Init(context.Context) error { return nil }

func (w *warningTracker) Close() error { return nil }

func (w *warningTracker) CreateIssue(context.Context, *core.Issue) (string, error) {
	return "", core.NewTrackerWarning("create issue", errors.New("github api unavailable"))
}

func (w *warningTracker) UpdateStatus(context.Context, string, core.IssueStatus) error {
	return core.NewTrackerWarning("update issue", errors.New("github api unavailable"))
}

func (w *warningTracker) SyncDependencies(context.Context, *core.Issue, []*core.Issue) error {
	return core.NewTrackerWarning("sync dependencies", errors.New("github api unavailable"))
}

func (w *warningTracker) OnExternalComplete(context.Context, string) error {
	return nil
}
