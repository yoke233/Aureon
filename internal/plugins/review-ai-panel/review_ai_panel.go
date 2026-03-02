package reviewaipanel

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/user/ai-workflow/internal/core"
	"github.com/user/ai-workflow/internal/secretary"
)

const gateReviewer = "review_gate"

type reviewPanel interface {
	Run(ctx context.Context, plan *core.TaskPlan, input secretary.ReviewInput) (*secretary.ReviewResult, error)
}

type runState struct {
	round       int
	cancel      context.CancelFunc
	running     bool
	cancelled   bool
	terminalErr error
}

// AIReviewGate runs secretary.ReviewOrchestrator asynchronously and exposes polling/cancel APIs.
type AIReviewGate struct {
	store core.Store
	panel reviewPanel
	input secretary.ReviewInput

	mu     sync.Mutex
	closed bool
	runs   map[string]*runState
}

func New(store core.Store, panel reviewPanel) *AIReviewGate {
	return &AIReviewGate{
		store: store,
		panel: panel,
		runs:  make(map[string]*runState),
	}
}

func (g *AIReviewGate) Name() string {
	return "ai-panel"
}

func (g *AIReviewGate) Init(context.Context) error {
	if g == nil {
		return errors.New("review-ai-panel gate is nil")
	}
	if g.store == nil {
		return errors.New("review-ai-panel store is nil")
	}
	g.mu.Lock()
	if g.runs == nil {
		g.runs = make(map[string]*runState)
	}
	g.closed = false
	g.mu.Unlock()
	return nil
}

func (g *AIReviewGate) Close() error {
	if g == nil {
		return nil
	}

	g.mu.Lock()
	if g.closed {
		g.mu.Unlock()
		return nil
	}
	g.closed = true

	states := make([]*runState, 0, len(g.runs))
	for _, state := range g.runs {
		states = append(states, state)
	}
	g.mu.Unlock()

	for _, state := range states {
		if state != nil && state.running && state.cancel != nil {
			state.cancel()
		}
	}
	return nil
}

func (g *AIReviewGate) Submit(ctx context.Context, plan *core.TaskPlan) (string, error) {
	if err := g.ensureReady(); err != nil {
		return "", err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if plan == nil {
		return "", errors.New("review-ai-panel submit: plan is nil")
	}
	if g.panel == nil {
		return "", errors.New("review-ai-panel submit: review orchestrator is nil")
	}

	planID := strings.TrimSpace(plan.ID)
	if planID == "" {
		return "", errors.New("review-ai-panel submit: plan id is required")
	}

	records, err := g.store.GetReviewRecords(planID)
	if err != nil {
		return "", fmt.Errorf("review-ai-panel submit list records: %w", err)
	}
	round := nextRound(records, plan)

	runCtx, cancel := context.WithCancel(context.Background())
	if err := g.markRunning(planID, &runState{round: round, cancel: cancel, running: true}); err != nil {
		cancel()
		return "", err
	}

	previousPlan, prevErr := g.store.GetTaskPlan(planID)
	if prevErr != nil && !isNotFoundErr(prevErr) {
		g.unmarkRunning(planID)
		cancel()
		return "", fmt.Errorf("review-ai-panel submit load previous plan: %w", prevErr)
	}
	hadPreviousPlan := prevErr == nil

	runPlan := clonePlan(plan)
	runPlan.ID = planID
	runPlan.Status = core.PlanReviewing
	runPlan.WaitReason = core.WaitNone
	if runPlan.ReviewRound < round-1 {
		runPlan.ReviewRound = round - 1
	}
	if err := g.store.SaveTaskPlan(runPlan); err != nil {
		g.unmarkRunning(planID)
		cancel()
		return "", fmt.Errorf("review-ai-panel submit save plan: %w", err)
	}
	if err := g.store.SaveReviewRecord(&core.ReviewRecord{
		PlanID:   planID,
		Round:    round,
		Reviewer: gateReviewer,
		Verdict:  "pending",
	}); err != nil {
		g.unmarkRunning(planID)
		cancel()
		if hadPreviousPlan && previousPlan != nil {
			_ = g.store.SaveTaskPlan(previousPlan)
		}
		return "", fmt.Errorf("review-ai-panel submit save pending record: %w", err)
	}

	go g.runAsync(planID, round, runPlan, runCtx)
	return planID, nil
}

func (g *AIReviewGate) Check(ctx context.Context, reviewID string) (*core.ReviewResult, error) {
	if err := g.ensureReady(); err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	planID := strings.TrimSpace(reviewID)
	if planID == "" {
		return nil, errors.New("review-ai-panel check: review id is required")
	}

	state, _ := g.getRunState(planID)
	running := g.isRunning(planID)

	records, err := g.store.GetReviewRecords(planID)
	if err != nil {
		return nil, fmt.Errorf("review-ai-panel check list records: %w", err)
	}
	plan, planErr := g.store.GetTaskPlan(planID)
	if planErr != nil && !isNotFoundErr(planErr) {
		return nil, fmt.Errorf("review-ai-panel check load plan: %w", planErr)
	}
	if isNotFoundErr(planErr) {
		plan = nil
	}

	if len(records) == 0 {
		if running {
			return &core.ReviewResult{
				Status:   "pending",
				Decision: "pending",
			}, nil
		}
		return nil, fmt.Errorf("review-ai-panel check: review %q not found", planID)
	}

	if running {
		if state != nil && state.cancelled {
			return &core.ReviewResult{
				Status:   "cancelled",
				Decision: "cancelled",
				Verdicts: recordsToVerdicts(records, latestVerdictRound(records)),
			}, nil
		}
		latestRound := latestVerdictRound(records)
		return &core.ReviewResult{
			Status:   "pending",
			Decision: "pending",
			Verdicts: recordsToVerdicts(records, latestRound),
		}, nil
	}
	if state != nil && state.terminalErr != nil {
		return nil, fmt.Errorf("review-ai-panel check terminal state persistence: %w", state.terminalErr)
	}

	latest := records[len(records)-1]
	latestRound := latestVerdictRound(records)
	if latestRound <= 0 {
		latestRound = latest.Round
	}
	status, decision := mapStatusDecision(latest.Verdict, plan)
	result := &core.ReviewResult{
		Status:   status,
		Decision: decision,
		Verdicts: recordsToVerdicts(records, latestRound),
	}
	if plan != nil {
		result.Revised = clonePlan(plan)
	}
	return result, nil
}

func (g *AIReviewGate) Cancel(ctx context.Context, reviewID string) error {
	if err := g.ensureReady(); err != nil {
		return err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	planID := strings.TrimSpace(reviewID)
	if planID == "" {
		return errors.New("review-ai-panel cancel: review id is required")
	}

	state, running := g.getRunState(planID)
	if running {
		g.markCancelled(planID)
		if state.cancel != nil {
			state.cancel()
		}
		if err := g.persistCancelled(planID, state.round); err != nil {
			g.setTerminalError(planID, err)
			return fmt.Errorf("review-ai-panel cancel persist state: %w", err)
		}
		return nil
	}

	records, err := g.store.GetReviewRecords(planID)
	if err != nil {
		return fmt.Errorf("review-ai-panel cancel list records: %w", err)
	}
	if len(records) == 0 {
		return fmt.Errorf("review-ai-panel cancel: review %q not found", planID)
	}
	if normalizeVerdict(records[len(records)-1].Verdict) == "cancelled" {
		return nil
	}
	return fmt.Errorf("review-ai-panel cancel: review %q is not running", planID)
}

func (g *AIReviewGate) runAsync(planID string, round int, plan *core.TaskPlan, runCtx context.Context) {
	defer g.unmarkRunning(planID)

	_, err := g.panel.Run(runCtx, clonePlan(plan), g.input)
	if err == nil {
		return
	}
	cancelled := g.wasCancelled(planID) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
	if cancelled {
		if persistErr := g.persistCancelled(planID, round); persistErr != nil {
			g.setTerminalError(planID, persistErr)
		}
		return
	}
	if persistErr := g.persistRejected(planID, round, err); persistErr != nil {
		g.setTerminalError(planID, persistErr)
	}
}

func (g *AIReviewGate) persistCancelled(planID string, fallbackRound int) error {
	records, err := g.store.GetReviewRecords(planID)
	if err != nil {
		return err
	}
	if len(records) > 0 && normalizeVerdict(records[len(records)-1].Verdict) == "cancelled" {
		return nil
	}

	round := fallbackRound
	for _, record := range records {
		if record.Round > round {
			round = record.Round
		}
	}
	if round <= 0 {
		round = 1
	}

	if err := g.store.SaveReviewRecord(&core.ReviewRecord{
		PlanID:   planID,
		Round:    round,
		Reviewer: gateReviewer,
		Verdict:  "cancelled",
	}); err != nil {
		return err
	}

	plan, planErr := g.store.GetTaskPlan(planID)
	if planErr != nil {
		if isNotFoundErr(planErr) {
			return nil
		}
		return planErr
	}
	plan.Status = core.PlanWaitingHuman
	plan.WaitReason = core.WaitFeedbackReq
	if plan.ReviewRound < round {
		plan.ReviewRound = round
	}
	return g.store.SaveTaskPlan(plan)
}

func (g *AIReviewGate) persistRejected(planID string, fallbackRound int, runErr error) error {
	records, err := g.store.GetReviewRecords(planID)
	if err != nil {
		return err
	}
	if len(records) > 0 && normalizeVerdict(records[len(records)-1].Verdict) == "cancelled" {
		return nil
	}

	round := fallbackRound
	for _, record := range records {
		if record.Round > round {
			round = record.Round
		}
	}
	if round <= 0 {
		round = 1
	}

	record := &core.ReviewRecord{
		PlanID:   planID,
		Round:    round,
		Reviewer: gateReviewer,
		Verdict:  "rejected",
	}
	if runErr != nil {
		record.Issues = []core.ReviewIssue{
			{
				Severity:    "error",
				Description: strings.TrimSpace(runErr.Error()),
			},
		}
	}
	if err := g.store.SaveReviewRecord(record); err != nil {
		return err
	}

	plan, planErr := g.store.GetTaskPlan(planID)
	if planErr != nil {
		if isNotFoundErr(planErr) {
			return nil
		}
		return planErr
	}
	plan.Status = core.PlanWaitingHuman
	plan.WaitReason = core.WaitFeedbackReq
	if plan.ReviewRound < round {
		plan.ReviewRound = round
	}
	return g.store.SaveTaskPlan(plan)
}

func (g *AIReviewGate) ensureReady() error {
	if g == nil {
		return errors.New("review-ai-panel gate is nil")
	}
	if g.store == nil {
		return errors.New("review-ai-panel store is nil")
	}
	g.mu.Lock()
	closed := g.closed
	g.mu.Unlock()
	if closed {
		return errors.New("review-ai-panel gate is closed")
	}
	return nil
}

func (g *AIReviewGate) markRunning(planID string, state *runState) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closed {
		return errors.New("review-ai-panel gate is closed")
	}
	if existing, ok := g.runs[planID]; ok && existing != nil && existing.running {
		return fmt.Errorf("review-ai-panel submit: review %q is already running", planID)
	}
	state.running = true
	state.terminalErr = nil
	state.cancelled = false
	g.runs[planID] = state
	return nil
}

func (g *AIReviewGate) unmarkRunning(planID string) {
	g.mu.Lock()
	if state, ok := g.runs[planID]; ok && state != nil {
		state.running = false
		state.cancel = nil
	}
	g.mu.Unlock()
}

func (g *AIReviewGate) isRunning(planID string) bool {
	g.mu.Lock()
	state, ok := g.runs[planID]
	g.mu.Unlock()
	return ok && state != nil && state.running
}

func (g *AIReviewGate) getRunState(planID string) (*runState, bool) {
	g.mu.Lock()
	state, ok := g.runs[planID]
	g.mu.Unlock()
	return state, ok && state != nil && state.running
}

func (g *AIReviewGate) markCancelled(planID string) {
	g.mu.Lock()
	if state, ok := g.runs[planID]; ok && state != nil {
		state.cancelled = true
	}
	g.mu.Unlock()
}

func (g *AIReviewGate) wasCancelled(planID string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	state, ok := g.runs[planID]
	return ok && state != nil && state.cancelled
}

func (g *AIReviewGate) setTerminalError(planID string, err error) {
	if err == nil {
		return
	}
	g.mu.Lock()
	if state, ok := g.runs[planID]; ok && state != nil {
		state.terminalErr = err
	}
	g.mu.Unlock()
}

func nextRound(records []core.ReviewRecord, plan *core.TaskPlan) int {
	maxRound := 0
	for _, record := range records {
		if record.Round > maxRound {
			maxRound = record.Round
		}
	}
	if plan != nil && plan.ReviewRound > maxRound {
		maxRound = plan.ReviewRound
	}
	return maxRound + 1
}

func recordsToVerdicts(records []core.ReviewRecord, round int) []core.ReviewVerdict {
	if len(records) == 0 {
		return nil
	}
	out := make([]core.ReviewVerdict, 0, len(records))
	for _, record := range records {
		if round > 0 && record.Round != round {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(record.Reviewer), gateReviewer) {
			continue
		}
		score := 0
		if record.Score != nil {
			score = *record.Score
		}
		out = append(out, core.ReviewVerdict{
			Reviewer: strings.TrimSpace(record.Reviewer),
			Status:   strings.TrimSpace(record.Verdict),
			Issues:   append([]core.ReviewIssue(nil), record.Issues...),
			Score:    score,
		})
	}
	if len(out) > 0 {
		return out
	}

	last := records[len(records)-1]
	score := 0
	if last.Score != nil {
		score = *last.Score
	}
	return []core.ReviewVerdict{
		{
			Reviewer: strings.TrimSpace(last.Reviewer),
			Status:   strings.TrimSpace(last.Verdict),
			Issues:   append([]core.ReviewIssue(nil), last.Issues...),
			Score:    score,
		},
	}
}

func latestVerdictRound(records []core.ReviewRecord) int {
	latest := 0
	for _, record := range records {
		if strings.EqualFold(strings.TrimSpace(record.Reviewer), gateReviewer) {
			continue
		}
		if record.Round > latest {
			latest = record.Round
		}
	}
	if latest > 0 {
		return latest
	}
	if len(records) > 0 {
		return records[len(records)-1].Round
	}
	return 0
}

func mapStatusDecision(verdict string, plan *core.TaskPlan) (status string, decision string) {
	normalized := normalizeVerdict(verdict)
	if normalized == "cancelled" {
		return "cancelled", "cancelled"
	}

	if plan != nil && plan.Status == core.PlanWaitingHuman {
		switch plan.WaitReason {
		case core.WaitFinalApproval:
			return "approved", "approve"
		case core.WaitFeedbackReq:
			return "rejected", "escalate"
		}
	}

	switch normalized {
	case "", "pending":
		return "pending", "pending"
	case "approved", "approve", "pass":
		return "approved", "approve"
	case "escalate":
		return "rejected", "escalate"
	case "rejected", "reject":
		return "rejected", "reject"
	case "changes_requested", "fix", "issues_found":
		return "changes_requested", "fix"
	}

	if plan != nil {
		switch plan.WaitReason {
		case core.WaitFinalApproval:
			return "approved", "approve"
		case core.WaitFeedbackReq:
			return "rejected", "reject"
		}
	}

	unknown := strings.TrimSpace(verdict)
	if unknown == "" {
		return "pending", "pending"
	}
	return unknown, unknown
}

func normalizeVerdict(verdict string) string {
	value := strings.ToLower(strings.TrimSpace(verdict))
	if value == "canceled" {
		return "cancelled"
	}
	return value
}

func clonePlan(plan *core.TaskPlan) *core.TaskPlan {
	if plan == nil {
		return nil
	}
	cp := *plan
	if len(plan.Tasks) > 0 {
		cp.Tasks = make([]core.TaskItem, len(plan.Tasks))
		for i, task := range plan.Tasks {
			cp.Tasks[i] = task
			cp.Tasks[i].Labels = append([]string(nil), task.Labels...)
			cp.Tasks[i].DependsOn = append([]string(nil), task.DependsOn...)
		}
	} else {
		cp.Tasks = nil
	}
	return &cp
}

func isNotFoundErr(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "not found")
}

var _ core.ReviewGate = (*AIReviewGate)(nil)
