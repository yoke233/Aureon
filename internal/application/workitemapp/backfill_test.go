package workitemapp

import (
	"context"
	"testing"

	agentapp "github.com/yoke233/zhanggui/internal/application/agent"
	"github.com/yoke233/zhanggui/internal/core"
)

func TestBackfillLegacyWorkItems(t *testing.T) {
	store := newWorkItemAppTestStore(t)
	ctx := context.Background()

	registry := agentapp.NewConfigRegistry()
	registry.LoadProfiles([]*core.AgentProfile{
		{ID: "ceo", Role: core.RoleLead},
		{ID: "lead", Role: core.RoleLead, ManagerProfileID: "ceo"},
		{ID: "worker", Role: core.RoleWorker, ManagerProfileID: "lead"},
	})

	workItemID, err := store.CreateWorkItem(ctx, &core.WorkItem{
		Title:    "legacy",
		Status:   core.WorkItemOpen,
		Priority: core.PriorityMedium,
		Metadata: map[string]any{
			"ceo": map[string]any{"assigned_profile": "worker"},
		},
	})
	if err != nil {
		t.Fatalf("CreateWorkItem() error = %v", err)
	}
	actionID, err := store.CreateAction(ctx, &core.Action{
		WorkItemID: workItemID,
		Name:       "implement",
		Type:       core.ActionExec,
		Status:     core.ActionDone,
	})
	if err != nil {
		t.Fatalf("CreateAction() error = %v", err)
	}
	runID, err := store.CreateRun(ctx, &core.Run{
		ActionID:   actionID,
		WorkItemID: workItemID,
		Status:     core.RunSucceeded,
		Attempt:    1,
	})
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	run, err := store.GetRun(ctx, runID)
	if err != nil {
		t.Fatalf("GetRun() error = %v", err)
	}
	run.ResultMarkdown = "# Legacy result"
	run.ResultMetadata = map[string]any{
		core.ResultMetaArtifactType:  "document",
		core.ResultMetaArtifactTitle: "Legacy result",
	}
	if err := store.UpdateRun(ctx, run); err != nil {
		t.Fatalf("UpdateRun() error = %v", err)
	}

	if err := BackfillLegacyWorkItems(ctx, store, registry); err != nil {
		t.Fatalf("BackfillLegacyWorkItems() error = %v", err)
	}

	got, err := store.GetWorkItem(ctx, workItemID)
	if err != nil {
		t.Fatalf("GetWorkItem() error = %v", err)
	}
	if got.ExecutorProfileID != "worker" || got.ActiveProfileID != "worker" {
		t.Fatalf("executor/active = %q/%q, want worker/worker", got.ExecutorProfileID, got.ActiveProfileID)
	}
	if got.ReviewerProfileID != "lead" || got.SponsorProfileID != "ceo" {
		t.Fatalf("reviewer/sponsor = %q/%q, want lead/ceo", got.ReviewerProfileID, got.SponsorProfileID)
	}
	if got.RootWorkItemID == nil || *got.RootWorkItemID != workItemID {
		t.Fatalf("RootWorkItemID = %v, want %d", got.RootWorkItemID, workItemID)
	}
	if got.FinalDeliverableID == nil {
		t.Fatal("expected final deliverable to be backfilled")
	}
	if len(got.EscalationPath) != 3 || got.EscalationPath[2] != "human" {
		t.Fatalf("EscalationPath = %#v", got.EscalationPath)
	}

	journalCount, err := store.CountJournal(ctx, core.JournalFilter{
		WorkItemID: &workItemID,
		Kinds:      []core.JournalKind{core.JournalBackfill},
	})
	if err != nil {
		t.Fatalf("CountJournal() error = %v", err)
	}
	if journalCount != 1 {
		t.Fatalf("JournalBackfill count = %d, want 1", journalCount)
	}
}
