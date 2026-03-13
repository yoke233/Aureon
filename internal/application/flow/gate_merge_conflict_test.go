package flow

import (
	"context"
	"fmt"
	"testing"

	"github.com/yoke233/ai-workflow/internal/core"
)

// TestSaveMergeConflictToConfig: verifies conflict metadata is persisted to step config.
func TestSaveMergeConflictToConfig(t *testing.T) {
	step := &core.Step{ID: 1, IssueID: 10}
	metadata := map[string]any{
		"merge_error":     "PR has conflicts",
		"pr_number":       float64(42),
		"pr_url":          "https://github.com/test/repo/pull/42",
		"mergeable_state": "dirty",
		"merge_provider":  "github",
	}
	saveMergeConflictToConfig(step, "merge failed: conflicts", metadata)

	if step.Config == nil {
		t.Fatal("expected config to be initialized")
	}
	if bt, _ := step.Config["blocked_type"].(string); bt != "merge_conflict" {
		t.Fatalf("expected blocked_type=merge_conflict, got %q", bt)
	}
	if _, ok := step.Config["blocked_at"].(string); !ok {
		t.Fatal("expected blocked_at timestamp")
	}
	if br, _ := step.Config["blocked_reason"].(string); br != "merge failed: conflicts" {
		t.Fatalf("expected blocked_reason, got %q", br)
	}
	if ms, _ := step.Config["mergeable_state"].(string); ms != "dirty" {
		t.Fatalf("expected mergeable_state=dirty, got %q", ms)
	}
	if prn, _ := step.Config["pr_number"].(float64); prn != 42 {
		t.Fatalf("expected pr_number=42, got %v", prn)
	}
	if url, _ := step.Config["pr_url"].(string); url != "https://github.com/test/repo/pull/42" {
		t.Fatalf("expected pr_url, got %q", url)
	}
}

// TestSaveMergeConflictToConfig_NilConfig: works on a step with nil Config.
func TestSaveMergeConflictToConfig_NilConfig(t *testing.T) {
	step := &core.Step{ID: 2, Config: nil}
	saveMergeConflictToConfig(step, "conflict", map[string]any{})
	if step.Config == nil {
		t.Fatal("config should have been initialized")
	}
	if bt, _ := step.Config["blocked_type"].(string); bt != "merge_conflict" {
		t.Fatalf("expected blocked_type=merge_conflict, got %q", bt)
	}
}

// TestHandleMergeConflictBlock_DirtyReturnsTrue: dirty merge error → handled (returns true).
func TestHandleMergeConflictBlock_DirtyReturnsTrue(t *testing.T) {
	store, bus := setup(t)
	ctx := context.Background()
	eng := New(store, bus, nil, WithConcurrency(1))

	// Subscribe to events.
	sub := bus.Subscribe(core.SubscribeOpts{
		Types:      []core.EventType{core.EventGateAwaitingHuman},
		BufferSize: 10,
	})
	defer sub.Cancel()

	issueID, _ := store.CreateIssue(ctx, &core.Issue{Title: "conflict-test", Status: core.IssueRunning})
	stepID, _ := store.CreateStep(ctx, &core.Step{
		IssueID:  issueID,
		Name:     "gate",
		Type:     core.StepGate,
		Status:   core.StepRunning,
		Position: 0,
	})
	step, _ := store.GetStep(ctx, stepID)

	mergeErr := &MergeError{
		Provider:       "github",
		Number:         42,
		URL:            "https://github.com/test/repo/pull/42",
		Message:        "This branch has conflicts",
		MergeableState: "dirty",
	}

	handled := eng.handleMergeConflictBlock(ctx, step, mergeErr)
	if !handled {
		t.Fatal("expected handleMergeConflictBlock to return true for dirty merge error")
	}

	// Verify step was updated with conflict info.
	updated, _ := store.GetStep(ctx, stepID)
	if updated.Status != core.StepBlocked {
		t.Fatalf("expected step status=blocked, got %s", updated.Status)
	}
	if bt, _ := updated.Config["blocked_type"].(string); bt != "merge_conflict" {
		t.Fatalf("expected blocked_type=merge_conflict, got %q", bt)
	}
	if ms, _ := updated.Config["mergeable_state"].(string); ms != "dirty" {
		t.Fatalf("expected mergeable_state=dirty, got %q", ms)
	}

	// Verify EventGateAwaitingHuman was published.
	found := false
	for {
		select {
		case ev := <-sub.C:
			if ev.Type == core.EventGateAwaitingHuman {
				found = true
			}
		default:
			goto done
		}
	}
done:
	if !found {
		t.Fatal("expected EventGateAwaitingHuman event")
	}
}

// TestHandleMergeConflictBlock_BehindReturnsFalse: "behind" merge error → not handled (returns false).
func TestHandleMergeConflictBlock_BehindReturnsFalse(t *testing.T) {
	store, bus := setup(t)
	ctx := context.Background()
	eng := New(store, bus, nil, WithConcurrency(1))

	issueID, _ := store.CreateIssue(ctx, &core.Issue{Title: "behind-test", Status: core.IssueRunning})
	stepID, _ := store.CreateStep(ctx, &core.Step{
		IssueID:  issueID,
		Name:     "gate",
		Type:     core.StepGate,
		Status:   core.StepRunning,
		Position: 0,
	})
	step, _ := store.GetStep(ctx, stepID)

	mergeErr := &MergeError{
		Provider:       "github",
		Number:         42,
		Message:        "Branch is out of date",
		MergeableState: "behind",
	}

	handled := eng.handleMergeConflictBlock(ctx, step, mergeErr)
	if handled {
		t.Fatal("expected handleMergeConflictBlock to return false for 'behind' merge error")
	}

	// Step status should remain running (not blocked).
	updated, _ := store.GetStep(ctx, stepID)
	if updated.Status != core.StepRunning {
		t.Fatalf("expected step status=running (unchanged), got %s", updated.Status)
	}
}

// TestHandleMergeConflictBlock_NonMergeErrorReturnsFalse: generic error → not handled.
func TestHandleMergeConflictBlock_NonMergeErrorReturnsFalse(t *testing.T) {
	store, bus := setup(t)
	ctx := context.Background()
	eng := New(store, bus, nil, WithConcurrency(1))

	issueID, _ := store.CreateIssue(ctx, &core.Issue{Title: "generic-err", Status: core.IssueRunning})
	stepID, _ := store.CreateStep(ctx, &core.Step{
		IssueID:  issueID,
		Name:     "gate",
		Type:     core.StepGate,
		Status:   core.StepRunning,
		Position: 0,
	})
	step, _ := store.GetStep(ctx, stepID)

	genericErr := fmt.Errorf("workspace is required for merge")

	handled := eng.handleMergeConflictBlock(ctx, step, genericErr)
	if handled {
		t.Fatal("expected handleMergeConflictBlock to return false for generic error")
	}
}
