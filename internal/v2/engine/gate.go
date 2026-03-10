package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/yoke233/ai-workflow/internal/v2/core"
)

// GateResult represents the outcome of a gate evaluation.
type GateResult struct {
	Passed  bool
	Reason  string
	ResetTo []int64 // Step IDs to reset on reject (upstream rework)
}

// ProcessGate handles a gate Step: pass → downstream continue, reject → reset upstream + gate re-enters loop.
func (e *FlowEngine) ProcessGate(ctx context.Context, step *core.Step, result GateResult) error {
	if step.Type != core.StepGate {
		return fmt.Errorf("step %d is not a gate (type=%s)", step.ID, step.Type)
	}

	if result.Passed {
		if err := e.transitionStep(ctx, step, core.StepDone); err != nil {
			return err
		}
		e.bus.Publish(ctx, core.Event{
			Type:      core.EventGatePassed,
			FlowID:    step.FlowID,
			StepID:    step.ID,
			Timestamp: time.Now().UTC(),
			Data:      map[string]any{"reason": result.Reason},
		})
		return nil
	}

	// Gate rejected.
	e.bus.Publish(ctx, core.Event{
		Type:      core.EventGateRejected,
		FlowID:    step.FlowID,
		StepID:    step.ID,
		Timestamp: time.Now().UTC(),
		Data:      map[string]any{"reason": result.Reason},
	})

	// Reset upstream steps for rework — persist retry_count via UpdateStep.
	for _, upID := range result.ResetTo {
		up, err := e.store.GetStep(ctx, upID)
		if err != nil {
			return fmt.Errorf("get upstream step %d: %w", upID, err)
		}
		if up.MaxRetries > 0 && up.RetryCount >= up.MaxRetries {
			return core.ErrMaxRetriesExceeded
		}
		up.RetryCount++
		up.Status = core.StepPending
		if err := e.store.UpdateStep(ctx, up); err != nil {
			return fmt.Errorf("reset step %d: %w", upID, err)
		}
	}

	// Gate itself → pending (will be re-promoted after upstream completes).
	return e.transitionStep(ctx, step, core.StepPending)
}

// finalizeGate is called after a gate step's executor succeeds.
// It reads the latest Artifact's metadata.verdict to decide pass/reject.
func (e *FlowEngine) finalizeGate(ctx context.Context, step *core.Step) error {
	art, err := e.store.GetLatestArtifactByStep(ctx, step.ID)
	if err == core.ErrNotFound {
		// No artifact — default to pass.
		e.bus.Publish(ctx, core.Event{
			Type:      core.EventGatePassed,
			FlowID:    step.FlowID,
			StepID:    step.ID,
			Timestamp: time.Now().UTC(),
		})
		return e.transitionStep(ctx, step, core.StepDone)
	}
	if err != nil {
		return fmt.Errorf("get gate artifact for step %d: %w", step.ID, err)
	}

	verdict, _ := art.Metadata["verdict"].(string)
	if verdict != "reject" {
		// "pass" or unrecognized → default pass.
		e.bus.Publish(ctx, core.Event{
			Type:      core.EventGatePassed,
			FlowID:    step.FlowID,
			StepID:    step.ID,
			Timestamp: time.Now().UTC(),
		})
		return e.transitionStep(ctx, step, core.StepDone)
	}

	// Reject — determine targets and delegate to ProcessGate.
	resetTo := extractResetTargets(art.Metadata, step.DependsOn)
	reason, _ := art.Metadata["reason"].(string)
	if reason == "" {
		reason = "gate rejected"
	}

	return e.ProcessGate(ctx, step, GateResult{
		Passed:  false,
		Reason:  reason,
		ResetTo: resetTo,
	})
}

// extractResetTargets reads reject_targets from metadata, falling back to deps.
func extractResetTargets(metadata map[string]any, fallback []int64) []int64 {
	targets, ok := metadata["reject_targets"].([]any)
	if !ok || len(targets) == 0 {
		return fallback
	}
	var result []int64
	for _, t := range targets {
		if id, ok := toInt64(t); ok {
			result = append(result, id)
		}
	}
	if len(result) == 0 {
		return fallback
	}
	return result
}

func toInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case float64:
		return int64(n), true
	case int64:
		return n, true
	case int:
		return int64(n), true
	default:
		return 0, false
	}
}
