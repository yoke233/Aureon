package teamleader

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/yoke233/ai-workflow/internal/core"
)

// GateStore is the persistence interface for gate checks.
type GateStore interface {
	SaveGateCheck(gc *core.GateCheck) error
	GetGateChecks(issueID string) ([]core.GateCheck, error)
	GetLatestGateCheck(issueID, gateName string) (*core.GateCheck, error)
	SaveTaskStep(step *core.TaskStep) (core.IssueStatus, error)
}

// GateChain executes a sequence of gates for an issue.
type GateChain struct {
	Store   GateStore
	Runners map[core.GateType]core.GateRunner
}

// GateChainResult holds the outcome of running a gate chain.
type GateChainResult struct {
	AllPassed   bool
	PendingGate string
	FailedCheck *core.GateCheck
	ForcePassed bool
}

// Run executes gates sequentially. For each gate it invokes the matching
// runner and retries up to MaxAttempts on failure. When all attempts are
// exhausted, the gate's Fallback strategy decides the outcome.
func (c *GateChain) Run(ctx context.Context, issue *core.Issue, gates []core.Gate) (*GateChainResult, error) {
	if len(gates) == 0 {
		return &GateChainResult{AllPassed: true}, nil
	}

	for _, gate := range gates {
		runner, ok := c.Runners[gate.Type]
		if !ok {
			slog.Warn("no runner for gate type, skipping", "gate", gate.Name, "type", gate.Type)
			c.recordStep(issue.ID, gate.Name, core.StepGatePassed, "skipped: no runner for type "+string(gate.Type))
			continue
		}

		for attempt := 1; ; attempt++ {
			check, err := runner.Check(ctx, issue, gate, attempt)
			if err != nil {
				return nil, fmt.Errorf("gate %q attempt %d: %w", gate.Name, attempt, err)
			}

			if err := c.Store.SaveGateCheck(check); err != nil {
				slog.Error("failed to save gate check", "err", err)
			}

			switch check.Status {
			case core.GateStatusPassed:
				c.recordStep(issue.ID, gate.Name, core.StepGatePassed, check.Reason)
				goto nextGate

			case core.GateStatusPending:
				c.recordStep(issue.ID, gate.Name, core.StepGateCheck, "awaiting resolution")
				return &GateChainResult{PendingGate: gate.Name}, nil

			case core.GateStatusFailed:
				c.recordStep(issue.ID, gate.Name, core.StepGateCheck, fmt.Sprintf("attempt %d failed: %s", attempt, check.Reason))
				if gate.MaxAttempts > 0 && attempt >= gate.MaxAttempts {
					return c.applyFallback(issue.ID, gate, check)
				}

			default:
				return nil, fmt.Errorf("gate %q: unexpected status %q", gate.Name, check.Status)
			}
		}
	nextGate:
	}

	return &GateChainResult{AllPassed: true}, nil
}

func (c *GateChain) applyFallback(issueID string, gate core.Gate, check *core.GateCheck) (*GateChainResult, error) {
	fallback := gate.Fallback
	if fallback == "" {
		fallback = core.GateFallbackEscalate
	}

	switch fallback {
	case core.GateFallbackForcePass:
		c.recordStep(issueID, gate.Name, core.StepGatePassed, "force_pass after max attempts")
		return &GateChainResult{AllPassed: true, ForcePassed: true}, nil

	case core.GateFallbackAbort:
		c.recordStep(issueID, gate.Name, core.StepGateFailed, "aborted after max attempts")
		return &GateChainResult{FailedCheck: check}, nil

	case core.GateFallbackEscalate:
		c.recordStep(issueID, gate.Name, core.StepGateFailed, "escalated after max attempts")
		return &GateChainResult{FailedCheck: check}, nil

	default:
		c.recordStep(issueID, gate.Name, core.StepGateFailed, "unknown fallback: "+string(fallback))
		return &GateChainResult{FailedCheck: check}, nil
	}
}

func (c *GateChain) recordStep(issueID, gateName string, action core.TaskStepAction, note string) {
	step := &core.TaskStep{
		ID:        core.NewTaskStepID(),
		IssueID:   issueID,
		Action:    action,
		Note:      fmt.Sprintf("[gate:%s] %s", gateName, note),
		CreatedAt: time.Now(),
	}
	if _, err := c.Store.SaveTaskStep(step); err != nil {
		slog.Error("failed to save gate task step", "err", err, "gate", gateName)
	}
}
