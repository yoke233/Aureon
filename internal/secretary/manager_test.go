package secretary

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/user/ai-workflow/internal/core"
	storesqlite "github.com/user/ai-workflow/internal/plugins/store-sqlite"
)

func TestManager_StartCallsRecoverExecutingPlans(t *testing.T) {
	t.Parallel()

	store := newManagerTestStore(t)
	defer store.Close()

	scheduler := &fakeManagerScheduler{}
	manager, err := NewManager(store, &fakeManagerAgent{}, &fakeManagerReviewOrchestrator{}, scheduler)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	if err := manager.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if !scheduler.startCalled {
		t.Fatal("scheduler.Start should be called")
	}
	if !scheduler.recoverCalled {
		t.Fatal("scheduler.RecoverExecutingPlans should be called")
	}
}

func TestManager_CreateDraftSubmitReviewApproveFlow(t *testing.T) {
	t.Parallel()

	store := newManagerTestStore(t)
	defer store.Close()

	project := mustCreateManagerProject(t, store, "proj-manager-flow")
	agent := &fakeManagerAgent{
		outputs: []*core.TaskPlan{
			{
				Name: "draft-login-plan",
				Tasks: []core.TaskItem{
					{
						ID:          "task-flow-1",
						Title:       "拆分认证模块任务",
						Description: "分析认证模块并生成执行任务",
						Template:    "standard",
						Status:      core.ItemPending,
					},
				},
			},
		},
	}

	review := &fakeManagerReviewOrchestrator{
		runFn: func(_ context.Context, plan *core.TaskPlan, _ ReviewInput) (*ReviewResult, error) {
			out := cloneManagerTestPlan(plan)
			out.Status = core.PlanWaitingHuman
			out.WaitReason = core.WaitFinalApproval
			out.ReviewRound = 1
			out.Tasks = []core.TaskItem{
				{
					ID:          "task-flow-2",
					PlanID:      plan.ID,
					Title:       "补全回归测试",
					Description: "补充登录流程回归测试并更新依赖",
					Template:    "standard",
					Status:      core.ItemPending,
				},
			}
			return &ReviewResult{
				Plan:     out,
				Decision: DecisionApprove,
				Round:    1,
			}, nil
		},
	}

	scheduler := &fakeManagerScheduler{store: store}
	manager, err := NewManager(store, agent, review, scheduler)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	draft, err := manager.CreateDraft(context.Background(), CreateDraftInput{
		ProjectID: project.ID,
		Request: Request{
			Conversation: "给认证系统增加可审计的登录流程",
			ProjectName:  "manager-flow",
			TechStack:    "go",
			RepoPath:     project.RepoPath,
		},
	})
	if err != nil {
		t.Fatalf("CreateDraft() error = %v", err)
	}
	if draft.Status != core.PlanDraft {
		t.Fatalf("draft status = %q, want %q", draft.Status, core.PlanDraft)
	}
	if _, err := store.GetTaskItem("task-flow-1"); err != nil {
		t.Fatalf("task-flow-1 should be persisted after CreateDraft, got error = %v", err)
	}

	reviewed, err := manager.SubmitReview(context.Background(), draft.ID, ReviewInput{
		Conversation:   "给认证系统增加可审计的登录流程",
		ProjectContext: "manager flow test",
	})
	if err != nil {
		t.Fatalf("SubmitReview() error = %v", err)
	}
	if reviewed.Status != core.PlanWaitingHuman {
		t.Fatalf("reviewed status = %q, want %q", reviewed.Status, core.PlanWaitingHuman)
	}
	if reviewed.WaitReason != core.WaitFinalApproval {
		t.Fatalf("reviewed wait_reason = %q, want %q", reviewed.WaitReason, core.WaitFinalApproval)
	}
	if _, err := store.GetTaskItem("task-flow-2"); err != nil {
		t.Fatalf("task-flow-2 should be upserted after SubmitReview, got error = %v", err)
	}
	if _, err := store.GetTaskItem("task-flow-1"); err == nil {
		t.Fatal("task-flow-1 should be replaced after SubmitReview")
	}

	executing, err := manager.ApplyPlanAction(context.Background(), reviewed.ID, PlanAction{
		Action: PlanActionApprove,
	})
	if err != nil {
		t.Fatalf("ApplyPlanAction(approve) error = %v", err)
	}
	if executing.Status != core.PlanExecuting {
		t.Fatalf("plan status after approve = %q, want %q", executing.Status, core.PlanExecuting)
	}
	if scheduler.startPlanCalls != 1 {
		t.Fatalf("scheduler StartPlan calls = %d, want 1", scheduler.startPlanCalls)
	}
}

func TestManager_CreateDraftSubmitReviewApproveFlowViaReviewGate(t *testing.T) {
	t.Parallel()

	store := newManagerTestStore(t)
	defer store.Close()

	project := mustCreateManagerProject(t, store, "proj-manager-flow-gate")
	agent := &fakeManagerAgent{
		outputs: []*core.TaskPlan{
			{
				Name: "draft-login-plan",
				Tasks: []core.TaskItem{
					{
						ID:          "task-flow-gate-1",
						Title:       "拆分认证模块任务",
						Description: "分析认证模块并生成执行任务",
						Template:    "standard",
						Status:      core.ItemPending,
					},
				},
			},
		},
	}

	review := &fakeManagerReviewOrchestrator{
		runFn: func(_ context.Context, _ *core.TaskPlan, _ ReviewInput) (*ReviewResult, error) {
			t.Fatal("ReviewOrchestrator.Run should not be called when ReviewGate is enabled")
			return nil, errors.New("unexpected review orchestrator run")
		},
	}
	gate := &fakeManagerReviewGate{
		submitFn: func(_ context.Context, plan *core.TaskPlan, callNo int) (string, error) {
			updated := cloneManagerTestPlan(plan)
			updated.Status = core.PlanReviewing
			updated.WaitReason = core.WaitNone
			updated.ReviewRound = callNo
			if err := store.SaveTaskPlan(updated); err != nil {
				return "", err
			}

			go func(planID string, round int) {
				time.Sleep(40 * time.Millisecond)
				next, err := store.GetTaskPlan(planID)
				if err != nil {
					return
				}
				next.Status = core.PlanWaitingHuman
				next.WaitReason = core.WaitFinalApproval
				next.ReviewRound = round
				_ = store.SaveTaskPlan(next)
			}(updated.ID, callNo)
			return updated.ID, nil
		},
	}

	scheduler := &fakeManagerScheduler{store: store}
	manager, err := NewManager(store, agent, review, scheduler, WithReviewGate(gate))
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	draft, err := manager.CreateDraft(context.Background(), CreateDraftInput{
		ProjectID: project.ID,
		Request: Request{
			Conversation: "给认证系统增加可审计的登录流程",
			ProjectName:  "manager-flow-gate",
			TechStack:    "go",
			RepoPath:     project.RepoPath,
		},
	})
	if err != nil {
		t.Fatalf("CreateDraft() error = %v", err)
	}

	reviewing, err := manager.SubmitReview(context.Background(), draft.ID, ReviewInput{
		Conversation:   "给认证系统增加可审计的登录流程",
		ProjectContext: "manager flow review gate",
	})
	if err != nil {
		t.Fatalf("SubmitReview() error = %v", err)
	}
	if reviewing.Status != core.PlanReviewing {
		t.Fatalf("reviewing status = %q, want %q", reviewing.Status, core.PlanReviewing)
	}
	if reviewing.WaitReason != core.WaitNone {
		t.Fatalf("reviewing wait_reason = %q, want %q", reviewing.WaitReason, core.WaitNone)
	}
	if gate.submitCalls != 1 {
		t.Fatalf("review gate submit calls = %d, want 1", gate.submitCalls)
	}

	waiting := waitManagerPlanState(
		t,
		manager,
		draft.ID,
		core.PlanWaitingHuman,
		core.WaitFinalApproval,
		2*time.Second,
	)
	if waiting.ReviewRound != 1 {
		t.Fatalf("waiting review_round = %d, want 1", waiting.ReviewRound)
	}

	executing, err := manager.ApplyPlanAction(context.Background(), waiting.ID, PlanAction{
		Action: PlanActionApprove,
	})
	if err != nil {
		t.Fatalf("ApplyPlanAction(approve) error = %v", err)
	}
	if executing.Status != core.PlanExecuting {
		t.Fatalf("plan status after approve = %q, want %q", executing.Status, core.PlanExecuting)
	}
	if scheduler.startPlanCalls != 1 {
		t.Fatalf("scheduler StartPlan calls = %d, want 1", scheduler.startPlanCalls)
	}
}

func TestManager_ApplyPlanActionApproveRequiresFinalApproval(t *testing.T) {
	t.Parallel()

	store := newManagerTestStore(t)
	defer store.Close()

	project := mustCreateManagerProject(t, store, "proj-manager-approve-invalid")
	plan := mustCreateManagerPlan(t, store, project.ID, "plan-manager-approve-invalid", core.PlanReviewing, core.WaitNone)

	scheduler := &fakeManagerScheduler{}
	manager, err := NewManager(store, &fakeManagerAgent{}, &fakeManagerReviewOrchestrator{}, scheduler)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	_, err = manager.ApplyPlanAction(context.Background(), plan.ID, PlanAction{Action: PlanActionApprove})
	if err == nil {
		t.Fatal("ApplyPlanAction(approve) should fail for non-waiting_human status")
	}
	if !strings.Contains(err.Error(), "approve requires waiting_human/final_approval") {
		t.Fatalf("error = %v, want contains %q", err, "approve requires waiting_human/final_approval")
	}
	if scheduler.startPlanCalls != 0 {
		t.Fatalf("scheduler StartPlan calls = %d, want 0", scheduler.startPlanCalls)
	}
}

func TestManager_ApplyPlanActionRejectTriggersRegeneration(t *testing.T) {
	t.Parallel()

	store := newManagerTestStore(t)
	defer store.Close()

	project := mustCreateManagerProject(t, store, "proj-manager-reject")
	agent := &fakeManagerAgent{
		outputs: []*core.TaskPlan{
			{
				Name: "initial-plan",
				Tasks: []core.TaskItem{
					{
						ID:          "task-reject-initial",
						Title:       "初始任务",
						Description: "先生成一版任务计划",
						Template:    "standard",
						Status:      core.ItemPending,
					},
				},
			},
			{
				Name: "regenerated-plan",
				Tasks: []core.TaskItem{
					{
						ID:          "task-regenerated",
						Title:       "重生成任务",
						Description: "根据人工反馈重生成任务结构",
						Template:    "standard",
						Status:      core.ItemPending,
					},
				},
			},
		},
	}

	review := &fakeManagerReviewOrchestrator{}
	review.runFn = func(_ context.Context, plan *core.TaskPlan, _ ReviewInput) (*ReviewResult, error) {
		review.mu.Lock()
		callNo := review.runCalls
		review.mu.Unlock()

		out := cloneManagerTestPlan(plan)
		switch callNo {
		case 1:
			out.Status = core.PlanWaitingHuman
			out.WaitReason = core.WaitFinalApproval
			out.ReviewRound = 1
			out.Tasks = []core.TaskItem{
				{
					ID:          "task-review-first",
					PlanID:      plan.ID,
					Title:       "首轮审核结果",
					Description: "首轮审核后等待人工最终确认",
					Template:    "standard",
					Status:      core.ItemPending,
				},
			}
			return &ReviewResult{
				Plan:     out,
				Decision: DecisionApprove,
				Round:    1,
			}, nil
		case 2:
			if plan.Status != core.PlanReviewing {
				t.Fatalf("second review should receive reviewing plan, got %q", plan.Status)
			}
			out.Status = core.PlanWaitingHuman
			out.WaitReason = core.WaitFinalApproval
			out.ReviewRound = 1
			out.Tasks = []core.TaskItem{
				{
					ID:          "task-review-second",
					PlanID:      plan.ID,
					Title:       "重审后结果",
					Description: "重审后再次进入人工最终确认",
					Template:    "standard",
					Status:      core.ItemPending,
				},
			}
			return &ReviewResult{
				Plan:     out,
				Decision: DecisionApprove,
				Round:    1,
			}, nil
		default:
			return nil, errors.New("unexpected review run call")
		}
	}

	review.handleRejectFn = func(ctx context.Context, plan *core.TaskPlan, feedback HumanFeedback, regenerator Regenerator) (*core.TaskPlan, error) {
		if plan.Status != core.PlanWaitingHuman {
			t.Fatalf("HandleHumanReject should receive waiting_human plan, got %q", plan.Status)
		}
		if plan.WaitReason != core.WaitFinalApproval {
			t.Fatalf("HandleHumanReject should receive final_approval wait reason, got %q", plan.WaitReason)
		}
		nextPlan, err := regenerator.Regenerate(ctx, RegenerationRequest{
			PlanID:       plan.ID,
			RevisionFrom: plan.ReviewRound,
			WaitReason:   plan.WaitReason,
			Feedback:     feedback,
			AIReviewSummary: AIReviewSummary{
				Rounds:       plan.ReviewRound,
				LastDecision: DecisionApprove,
				TopIssues:    []string{"需要补齐验收步骤"},
			},
		})
		if err != nil {
			return nil, err
		}
		nextPlan.ID = plan.ID
		nextPlan.ProjectID = plan.ProjectID
		nextPlan.Status = core.PlanReviewing
		nextPlan.WaitReason = core.WaitNone
		nextPlan.ReviewRound = 0
		return nextPlan, nil
	}

	manager, err := NewManager(store, agent, review, &fakeManagerScheduler{store: store})
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	draft, err := manager.CreateDraft(context.Background(), CreateDraftInput{
		ProjectID: project.ID,
		Request: Request{
			Conversation: "把发布流程拆成可回滚的子任务",
			ProjectName:  "manager-reject",
			TechStack:    "go",
			RepoPath:     project.RepoPath,
		},
	})
	if err != nil {
		t.Fatalf("CreateDraft() error = %v", err)
	}
	if _, err := manager.SubmitReview(context.Background(), draft.ID, ReviewInput{
		Conversation:   "把发布流程拆成可回滚的子任务",
		ProjectContext: "reject path",
	}); err != nil {
		t.Fatalf("SubmitReview() error = %v", err)
	}

	updated, err := manager.ApplyPlanAction(context.Background(), draft.ID, PlanAction{
		Action: PlanActionReject,
		Feedback: &HumanFeedback{
			Category:          FeedbackCoverageGap,
			Detail:            "当前计划没有覆盖上线回滚演练和异常告警回归，请补齐这两个任务并明确依赖关系。",
			ExpectedDirection: "补齐回滚和告警回归任务",
		},
	})
	if err != nil {
		t.Fatalf("ApplyPlanAction(reject) error = %v", err)
	}

	if review.handleRejectCalls != 1 {
		t.Fatalf("HandleHumanReject calls = %d, want 1", review.handleRejectCalls)
	}
	if review.runCalls != 2 {
		t.Fatalf("Run calls = %d, want 2 (initial + rerun)", review.runCalls)
	}
	if len(agent.calls) != 2 {
		t.Fatalf("agent Decompose calls = %d, want 2 (create + regenerate)", len(agent.calls))
	}
	if updated.Status != core.PlanWaitingHuman {
		t.Fatalf("updated plan status = %q, want %q", updated.Status, core.PlanWaitingHuman)
	}
	if updated.WaitReason != core.WaitFinalApproval {
		t.Fatalf("updated wait_reason = %q, want %q", updated.WaitReason, core.WaitFinalApproval)
	}
	if _, err := store.GetTaskItem("task-review-second"); err != nil {
		t.Fatalf("task-review-second should be upserted after rerun review, got error = %v", err)
	}
	if _, err := store.GetTaskItem("task-review-first"); err == nil {
		t.Fatal("task-review-first should be replaced after rerun review")
	}
}

func TestManager_ApplyPlanActionRejectResubmitsToReviewGate(t *testing.T) {
	t.Parallel()

	store := newManagerTestStore(t)
	defer store.Close()

	project := mustCreateManagerProject(t, store, "proj-manager-reject-gate")
	agent := &fakeManagerAgent{
		outputs: []*core.TaskPlan{
			{
				Name: "initial-plan",
				Tasks: []core.TaskItem{
					{
						ID:          "task-reject-gate-initial",
						Title:       "初始任务",
						Description: "先生成一版任务计划",
						Template:    "standard",
						Status:      core.ItemPending,
					},
				},
			},
			{
				Name: "regenerated-plan",
				Tasks: []core.TaskItem{
					{
						ID:          "task-reject-gate-regenerated",
						Title:       "重生成任务",
						Description: "根据人工反馈重生成任务结构",
						Template:    "standard",
						Status:      core.ItemPending,
					},
				},
			},
		},
	}

	review := &fakeManagerReviewOrchestrator{
		runFn: func(_ context.Context, _ *core.TaskPlan, _ ReviewInput) (*ReviewResult, error) {
			t.Fatal("ReviewOrchestrator.Run should not be called when ReviewGate is enabled")
			return nil, errors.New("unexpected review orchestrator run")
		},
	}
	review.handleRejectFn = func(ctx context.Context, plan *core.TaskPlan, feedback HumanFeedback, regenerator Regenerator) (*core.TaskPlan, error) {
		if plan.Status != core.PlanWaitingHuman {
			t.Fatalf("HandleHumanReject should receive waiting_human plan, got %q", plan.Status)
		}
		if plan.WaitReason != core.WaitFinalApproval {
			t.Fatalf("HandleHumanReject should receive final_approval wait reason, got %q", plan.WaitReason)
		}
		nextPlan, err := regenerator.Regenerate(ctx, RegenerationRequest{
			PlanID:       plan.ID,
			RevisionFrom: plan.ReviewRound,
			WaitReason:   plan.WaitReason,
			Feedback:     feedback,
			AIReviewSummary: AIReviewSummary{
				Rounds:       plan.ReviewRound,
				LastDecision: DecisionApprove,
				TopIssues:    []string{"补齐回滚演练任务"},
			},
		})
		if err != nil {
			return nil, err
		}
		nextPlan.ID = plan.ID
		nextPlan.ProjectID = plan.ProjectID
		nextPlan.Status = core.PlanReviewing
		nextPlan.WaitReason = core.WaitNone
		nextPlan.ReviewRound = 0
		return nextPlan, nil
	}

	gate := &fakeManagerReviewGate{
		submitFn: func(_ context.Context, plan *core.TaskPlan, callNo int) (string, error) {
			updated := cloneManagerTestPlan(plan)
			updated.Status = core.PlanReviewing
			updated.WaitReason = core.WaitNone
			updated.ReviewRound = callNo
			if err := store.SaveTaskPlan(updated); err != nil {
				return "", err
			}

			go func(planID string, round int) {
				time.Sleep(40 * time.Millisecond)
				next, err := store.GetTaskPlan(planID)
				if err != nil {
					return
				}
				next.Status = core.PlanWaitingHuman
				next.WaitReason = core.WaitFinalApproval
				next.ReviewRound = round
				_ = store.SaveTaskPlan(next)
			}(updated.ID, callNo)
			return updated.ID, nil
		},
	}

	manager, err := NewManager(store, agent, review, &fakeManagerScheduler{store: store}, WithReviewGate(gate))
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	draft, err := manager.CreateDraft(context.Background(), CreateDraftInput{
		ProjectID: project.ID,
		Request: Request{
			Conversation: "把发布流程拆成可回滚的子任务",
			ProjectName:  "manager-reject-gate",
			TechStack:    "go",
			RepoPath:     project.RepoPath,
		},
	})
	if err != nil {
		t.Fatalf("CreateDraft() error = %v", err)
	}

	if _, err := manager.SubmitReview(context.Background(), draft.ID, ReviewInput{
		Conversation:   "把发布流程拆成可回滚的子任务",
		ProjectContext: "reject gate path",
	}); err != nil {
		t.Fatalf("SubmitReview() error = %v", err)
	}

	waitManagerPlanState(
		t,
		manager,
		draft.ID,
		core.PlanWaitingHuman,
		core.WaitFinalApproval,
		2*time.Second,
	)

	reviewing, err := manager.ApplyPlanAction(context.Background(), draft.ID, PlanAction{
		Action: PlanActionReject,
		Feedback: &HumanFeedback{
			Category:          FeedbackCoverageGap,
			Detail:            "当前计划没有覆盖上线回滚演练和异常告警回归，请补齐这两个任务并明确依赖关系。",
			ExpectedDirection: "补齐回滚和告警回归任务",
		},
	})
	if err != nil {
		t.Fatalf("ApplyPlanAction(reject) error = %v", err)
	}

	if reviewing.Status != core.PlanReviewing {
		t.Fatalf("updated plan status = %q, want %q", reviewing.Status, core.PlanReviewing)
	}
	if reviewing.WaitReason != core.WaitNone {
		t.Fatalf("updated plan wait_reason = %q, want %q", reviewing.WaitReason, core.WaitNone)
	}
	if review.handleRejectCalls != 1 {
		t.Fatalf("HandleHumanReject calls = %d, want 1", review.handleRejectCalls)
	}
	if review.runCalls != 0 {
		t.Fatalf("Run calls = %d, want 0 when ReviewGate is enabled", review.runCalls)
	}
	if gate.submitCalls != 2 {
		t.Fatalf("review gate submit calls = %d, want 2 (initial + reject resubmit)", gate.submitCalls)
	}
	if len(agent.calls) != 2 {
		t.Fatalf("agent Decompose calls = %d, want 2 (create + regenerate)", len(agent.calls))
	}

	waitingAgain := waitManagerPlanState(
		t,
		manager,
		draft.ID,
		core.PlanWaitingHuman,
		core.WaitFinalApproval,
		2*time.Second,
	)
	if waitingAgain.ReviewRound != 2 {
		t.Fatalf("waiting review_round after reject = %d, want 2", waitingAgain.ReviewRound)
	}
}

func TestManager_ApplyPlanActionRejectRequiresAllowedWaitReason(t *testing.T) {
	t.Parallel()

	store := newManagerTestStore(t)
	defer store.Close()

	project := mustCreateManagerProject(t, store, "proj-manager-reject-invalid-wait-reason")
	plan := mustCreateManagerPlan(t, store, project.ID, "plan-manager-reject-invalid-wait-reason", core.PlanWaitingHuman, core.WaitNone)

	manager, err := NewManager(store, &fakeManagerAgent{}, &fakeManagerReviewOrchestrator{}, &fakeManagerScheduler{})
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	_, err = manager.ApplyPlanAction(context.Background(), plan.ID, PlanAction{
		Action: PlanActionReject,
		Feedback: &HumanFeedback{
			Category: FeedbackCoverageGap,
			Detail:   "当前计划仍缺少回滚演练和告警回归任务，请补齐并明确依赖关系以便调度执行。",
		},
	})
	if err == nil {
		t.Fatal("ApplyPlanAction(reject) should fail for waiting_human + empty wait_reason")
	}
	if !strings.Contains(err.Error(), "reject requires waiting_human/final_approval|feedback_required") {
		t.Fatalf("error = %v, want wait_reason guard", err)
	}
}

func TestManager_CreateDraft_TaskIDCollisionAcrossPlans(t *testing.T) {
	t.Parallel()

	store := newManagerTestStore(t)
	defer store.Close()

	project := mustCreateManagerProject(t, store, "proj-manager-id-collision")
	agent := &fakeManagerAgent{
		outputs: []*core.TaskPlan{
			{
				Name: "plan-a",
				Tasks: []core.TaskItem{
					{ID: "task-1", Title: "A", Description: "task A", Template: "standard"},
				},
			},
			{
				Name: "plan-b",
				Tasks: []core.TaskItem{
					{ID: "task-1", Title: "B", Description: "task B", Template: "standard"},
				},
			},
		},
	}

	manager, err := NewManager(store, agent, &fakeManagerReviewOrchestrator{}, &fakeManagerScheduler{})
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	planA, err := manager.CreateDraft(context.Background(), CreateDraftInput{
		ProjectID: project.ID,
		Request: Request{
			Conversation: "first plan",
			ProjectName:  "id-collision",
			RepoPath:     project.RepoPath,
		},
	})
	if err != nil {
		t.Fatalf("CreateDraft(planA) error = %v", err)
	}
	planB, err := manager.CreateDraft(context.Background(), CreateDraftInput{
		ProjectID: project.ID,
		Request: Request{
			Conversation: "second plan",
			ProjectName:  "id-collision",
			RepoPath:     project.RepoPath,
		},
	})
	if err != nil {
		t.Fatalf("CreateDraft(planB) error = %v", err)
	}

	if len(planA.Tasks) != 1 || len(planB.Tasks) != 1 {
		t.Fatalf("unexpected task count: planA=%d planB=%d", len(planA.Tasks), len(planB.Tasks))
	}
	if planA.Tasks[0].ID == planB.Tasks[0].ID {
		t.Fatalf("task ids should be disambiguated across plans, got duplicated id %q", planA.Tasks[0].ID)
	}

	taskA, err := store.GetTaskItem(planA.Tasks[0].ID)
	if err != nil {
		t.Fatalf("GetTaskItem(planA task) error = %v", err)
	}
	if taskA.PlanID != planA.ID {
		t.Fatalf("planA task plan_id = %q, want %q", taskA.PlanID, planA.ID)
	}
}

func TestManager_ApplyPlanActionRejectAfterManagerRestart(t *testing.T) {
	t.Parallel()

	store := newManagerTestStore(t)
	defer store.Close()

	project := mustCreateManagerProject(t, store, "proj-manager-restart-reject")

	agentCreate := &fakeManagerAgent{
		outputs: []*core.TaskPlan{
			{
				Name: "initial-plan",
				Tasks: []core.TaskItem{
					{ID: "task-initial", Title: "Initial", Description: "initial task", Template: "standard"},
				},
			},
		},
	}
	reviewCreate := &fakeManagerReviewOrchestrator{
		runFn: func(_ context.Context, plan *core.TaskPlan, _ ReviewInput) (*ReviewResult, error) {
			out := cloneManagerTestPlan(plan)
			out.Status = core.PlanWaitingHuman
			out.WaitReason = core.WaitFinalApproval
			out.ReviewRound = 1
			return &ReviewResult{Plan: out, Decision: DecisionApprove, Round: 1}, nil
		},
	}

	managerCreate, err := NewManager(store, agentCreate, reviewCreate, &fakeManagerScheduler{store: store})
	if err != nil {
		t.Fatalf("NewManager(create) error = %v", err)
	}
	draft, err := managerCreate.CreateDraft(context.Background(), CreateDraftInput{
		ProjectID: project.ID,
		Request: Request{
			Conversation: "restart reject flow",
			ProjectName:  "manager-restart",
			RepoPath:     project.RepoPath,
		},
	})
	if err != nil {
		t.Fatalf("CreateDraft() error = %v", err)
	}
	if _, err := managerCreate.SubmitReview(context.Background(), draft.ID, ReviewInput{
		Conversation:   "restart reject flow",
		ProjectContext: "restart reject",
	}); err != nil {
		t.Fatalf("SubmitReview() error = %v", err)
	}

	agentRestart := &fakeManagerAgent{
		outputs: []*core.TaskPlan{
			{
				Name: "regenerated-plan",
				Tasks: []core.TaskItem{
					{ID: "task-regenerated-restart", Title: "Regenerated", Description: "regen task", Template: "standard"},
				},
			},
		},
	}
	reviewRestart := &fakeManagerReviewOrchestrator{
		handleRejectFn: func(ctx context.Context, plan *core.TaskPlan, feedback HumanFeedback, regenerator Regenerator) (*core.TaskPlan, error) {
			next, err := regenerator.Regenerate(ctx, RegenerationRequest{
				PlanID:       plan.ID,
				RevisionFrom: plan.ReviewRound,
				WaitReason:   plan.WaitReason,
				Feedback:     feedback,
				AIReviewSummary: AIReviewSummary{
					Rounds:       plan.ReviewRound,
					LastDecision: DecisionApprove,
				},
			})
			if err != nil {
				return nil, err
			}
			next.ID = plan.ID
			next.ProjectID = plan.ProjectID
			next.Status = core.PlanReviewing
			next.WaitReason = core.WaitNone
			return next, nil
		},
		runFn: func(_ context.Context, plan *core.TaskPlan, _ ReviewInput) (*ReviewResult, error) {
			out := cloneManagerTestPlan(plan)
			out.Status = core.PlanWaitingHuman
			out.WaitReason = core.WaitFinalApproval
			out.ReviewRound = 1
			return &ReviewResult{Plan: out, Decision: DecisionApprove, Round: 1}, nil
		},
	}

	// 构造新的 manager 实例，验证无内存 planMeta 时 reject 仍可重生成。
	managerRestart, err := NewManager(store, agentRestart, reviewRestart, &fakeManagerScheduler{store: store})
	if err != nil {
		t.Fatalf("NewManager(restart) error = %v", err)
	}

	updated, err := managerRestart.ApplyPlanAction(context.Background(), draft.ID, PlanAction{
		Action: PlanActionReject,
		Feedback: &HumanFeedback{
			Category: FeedbackCoverageGap,
			Detail:   "重启后继续驳回流程，要求补齐异常回滚和告警验证步骤，确保任务可独立执行。",
		},
	})
	if err != nil {
		t.Fatalf("ApplyPlanAction(reject after restart) error = %v", err)
	}
	if updated.Status != core.PlanWaitingHuman || updated.WaitReason != core.WaitFinalApproval {
		t.Fatalf("updated plan = %s/%s, want waiting_human/final_approval", updated.Status, updated.WaitReason)
	}
	if len(agentRestart.calls) != 1 {
		t.Fatalf("regeneration decompose calls = %d, want 1", len(agentRestart.calls))
	}
}

func TestManager_ApplyPlanActionAbandonOnlyWaitingHuman(t *testing.T) {
	t.Parallel()

	store := newManagerTestStore(t)
	defer store.Close()

	project := mustCreateManagerProject(t, store, "proj-manager-abandon")

	notAllowed := mustCreateManagerPlan(t, store, project.ID, "plan-manager-abandon-invalid", core.PlanReviewing, core.WaitNone)
	manager, err := NewManager(store, &fakeManagerAgent{}, &fakeManagerReviewOrchestrator{}, &fakeManagerScheduler{})
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	_, err = manager.ApplyPlanAction(context.Background(), notAllowed.ID, PlanAction{Action: PlanActionAbandon})
	if err == nil {
		t.Fatal("ApplyPlanAction(abandon) should fail for non-waiting_human status")
	}
	if !strings.Contains(err.Error(), "abandon requires waiting_human") {
		t.Fatalf("error = %v, want contains %q", err, "abandon requires waiting_human")
	}

	allowed := mustCreateManagerPlan(t, store, project.ID, "plan-manager-abandon-valid", core.PlanWaitingHuman, core.WaitFeedbackReq)
	got, err := manager.ApplyPlanAction(context.Background(), allowed.ID, PlanAction{Action: PlanActionAbandon})
	if err != nil {
		t.Fatalf("ApplyPlanAction(abandon waiting_human) error = %v", err)
	}
	if got.Status != core.PlanAbandoned {
		t.Fatalf("abandon result status = %q, want %q", got.Status, core.PlanAbandoned)
	}
	if got.WaitReason != core.WaitNone {
		t.Fatalf("abandon result wait_reason = %q, want empty", got.WaitReason)
	}
}

func newManagerTestStore(t *testing.T) core.Store {
	t.Helper()

	store, err := storesqlite.New(":memory:")
	if err != nil {
		t.Fatalf("storesqlite.New() error = %v", err)
	}
	return store
}

func mustCreateManagerProject(t *testing.T, store core.Store, id string) *core.Project {
	t.Helper()

	project := &core.Project{
		ID:       id,
		Name:     id,
		RepoPath: t.TempDir(),
	}
	if err := store.CreateProject(project); err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	return project
}

func mustCreateManagerPlan(
	t *testing.T,
	store core.Store,
	projectID string,
	planID string,
	status core.TaskPlanStatus,
	waitReason core.WaitReason,
) *core.TaskPlan {
	t.Helper()

	plan := &core.TaskPlan{
		ID:         planID,
		ProjectID:  projectID,
		Name:       planID,
		Status:     status,
		WaitReason: waitReason,
		FailPolicy: core.FailBlock,
	}
	if err := store.CreateTaskPlan(plan); err != nil {
		t.Fatalf("CreateTaskPlan() error = %v", err)
	}
	return plan
}

type fakeManagerAgent struct {
	mu      sync.Mutex
	outputs []*core.TaskPlan
	calls   []Request
	err     error
}

func (a *fakeManagerAgent) Decompose(_ context.Context, req Request) (*core.TaskPlan, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.calls = append(a.calls, req)
	if a.err != nil {
		return nil, a.err
	}
	if len(a.outputs) == 0 {
		return nil, errors.New("no fake agent output configured")
	}

	next := cloneManagerTestPlan(a.outputs[0])
	a.outputs = a.outputs[1:]
	return next, nil
}

type fakeManagerReviewOrchestrator struct {
	mu                sync.Mutex
	runCalls          int
	handleRejectCalls int
	runFn             func(ctx context.Context, plan *core.TaskPlan, input ReviewInput) (*ReviewResult, error)
	handleRejectFn    func(ctx context.Context, plan *core.TaskPlan, feedback HumanFeedback, regenerator Regenerator) (*core.TaskPlan, error)
}

func (p *fakeManagerReviewOrchestrator) Run(ctx context.Context, plan *core.TaskPlan, input ReviewInput) (*ReviewResult, error) {
	p.mu.Lock()
	p.runCalls++
	runFn := p.runFn
	p.mu.Unlock()

	if runFn == nil {
		return &ReviewResult{
			Plan:     cloneManagerTestPlan(plan),
			Decision: DecisionApprove,
		}, nil
	}
	return runFn(ctx, cloneManagerTestPlan(plan), input)
}

func (p *fakeManagerReviewOrchestrator) HandleHumanReject(ctx context.Context, plan *core.TaskPlan, feedback HumanFeedback, regenerator Regenerator) (*core.TaskPlan, error) {
	p.mu.Lock()
	p.handleRejectCalls++
	handleFn := p.handleRejectFn
	p.mu.Unlock()

	if handleFn == nil {
		return cloneManagerTestPlan(plan), nil
	}
	return handleFn(ctx, cloneManagerTestPlan(plan), feedback, regenerator)
}

type fakeManagerScheduler struct {
	mu             sync.Mutex
	store          core.Store
	startCalled    bool
	stopCalled     bool
	recoverCalled  bool
	startPlanCalls int
	startErr       error
	stopErr        error
	recoverErr     error
	startPlanErr   error
}

func (s *fakeManagerScheduler) Start(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.startCalled = true
	return s.startErr
}

func (s *fakeManagerScheduler) Stop(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopCalled = true
	return s.stopErr
}

func (s *fakeManagerScheduler) RecoverExecutingPlans(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recoverCalled = true
	return s.recoverErr
}

func (s *fakeManagerScheduler) StartPlan(_ context.Context, plan *core.TaskPlan) error {
	s.mu.Lock()
	s.startPlanCalls++
	store := s.store
	err := s.startPlanErr
	s.mu.Unlock()

	if err != nil {
		return err
	}
	if store == nil {
		return nil
	}

	updated := cloneManagerTestPlan(plan)
	updated.Status = core.PlanExecuting
	updated.WaitReason = core.WaitNone
	return store.SaveTaskPlan(updated)
}

func cloneManagerTestPlan(plan *core.TaskPlan) *core.TaskPlan {
	if plan == nil {
		return nil
	}
	cp := *plan
	cp.Tasks = append([]core.TaskItem(nil), plan.Tasks...)
	return &cp
}

type fakeManagerReviewGate struct {
	mu          sync.Mutex
	submitCalls int
	checkCalls  int
	cancelCalls int
	submitFn    func(ctx context.Context, plan *core.TaskPlan, callNo int) (string, error)
	checkFn     func(ctx context.Context, reviewID string) (*core.ReviewResult, error)
	cancelFn    func(ctx context.Context, reviewID string) error
}

func (g *fakeManagerReviewGate) Name() string {
	return "fake-manager-review-gate"
}

func (g *fakeManagerReviewGate) Init(context.Context) error {
	return nil
}

func (g *fakeManagerReviewGate) Close() error {
	return nil
}

func (g *fakeManagerReviewGate) Submit(ctx context.Context, plan *core.TaskPlan) (string, error) {
	g.mu.Lock()
	g.submitCalls++
	callNo := g.submitCalls
	submitFn := g.submitFn
	g.mu.Unlock()

	if submitFn == nil {
		return plan.ID, nil
	}
	return submitFn(ctx, cloneManagerTestPlan(plan), callNo)
}

func (g *fakeManagerReviewGate) Check(ctx context.Context, reviewID string) (*core.ReviewResult, error) {
	g.mu.Lock()
	g.checkCalls++
	checkFn := g.checkFn
	g.mu.Unlock()

	if checkFn == nil {
		return &core.ReviewResult{
			Status:   "pending",
			Decision: "pending",
		}, nil
	}
	return checkFn(ctx, strings.TrimSpace(reviewID))
}

func (g *fakeManagerReviewGate) Cancel(ctx context.Context, reviewID string) error {
	g.mu.Lock()
	g.cancelCalls++
	cancelFn := g.cancelFn
	g.mu.Unlock()

	if cancelFn == nil {
		return nil
	}
	return cancelFn(ctx, strings.TrimSpace(reviewID))
}

func waitManagerPlanState(
	t *testing.T,
	manager *Manager,
	planID string,
	wantStatus core.TaskPlanStatus,
	wantWaitReason core.WaitReason,
	timeout time.Duration,
) *core.TaskPlan {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		plan, err := manager.GetPlan(context.Background(), planID)
		if err != nil {
			t.Fatalf("GetPlan(%s) error = %v", planID, err)
		}
		if plan.Status == wantStatus && plan.WaitReason == wantWaitReason {
			return plan
		}
		time.Sleep(20 * time.Millisecond)
	}

	last, err := manager.GetPlan(context.Background(), planID)
	if err != nil {
		t.Fatalf("GetPlan(%s) error = %v", planID, err)
	}
	t.Fatalf(
		"plan status = %s/%s, want %s/%s within %s",
		last.Status,
		last.WaitReason,
		wantStatus,
		wantWaitReason,
		timeout,
	)
	return nil
}
