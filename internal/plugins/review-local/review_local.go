package reviewlocal

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/yoke233/ai-workflow/internal/core"
)

const localReviewer = "local_human"

// LocalReviewGate stores local human review states in review_records.
type LocalReviewGate struct {
	store  core.Store
	mu     sync.RWMutex
	closed bool
}

func New(store core.Store) *LocalReviewGate {
	return &LocalReviewGate{
		store: store,
	}
}

func (g *LocalReviewGate) Name() string {
	return "local"
}

func (g *LocalReviewGate) Init(context.Context) error {
	if g == nil {
		return errors.New("review-local gate is nil")
	}
	if g.store == nil {
		return errors.New("review-local store is nil")
	}

	g.mu.Lock()
	g.closed = false
	g.mu.Unlock()
	return nil
}

func (g *LocalReviewGate) Close() error {
	if g == nil {
		return nil
	}
	g.mu.Lock()
	g.closed = true
	g.mu.Unlock()
	return nil
}

func (g *LocalReviewGate) Submit(ctx context.Context, plan *core.TaskPlan) (string, error) {
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
		return "", errors.New("review-local submit: plan is nil")
	}

	planID := strings.TrimSpace(plan.ID)
	if planID == "" {
		return "", errors.New("review-local submit: plan id is required")
	}

	records, err := g.store.GetReviewRecords(planID)
	if err != nil {
		return "", fmt.Errorf("review-local submit list records: %w", err)
	}
	round := nextRound(records)
	if plan.ReviewRound > round {
		round = plan.ReviewRound
	}

	record := &core.ReviewRecord{
		PlanID:   planID,
		Round:    round,
		Reviewer: localReviewer,
		Verdict:  "pending",
	}
	if err := g.store.SaveReviewRecord(record); err != nil {
		return "", fmt.Errorf("review-local submit save record: %w", err)
	}

	return planID, nil
}

func (g *LocalReviewGate) Check(ctx context.Context, reviewID string) (*core.ReviewResult, error) {
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
		return nil, errors.New("review-local check: review id is required")
	}

	records, err := g.store.GetReviewRecords(planID)
	if err != nil {
		return nil, fmt.Errorf("review-local check list records: %w", err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("review-local check: review %q not found", planID)
	}

	latest := records[len(records)-1]
	status, decision := mapVerdict(latest.Verdict)

	score := 0
	if latest.Score != nil {
		score = *latest.Score
	}

	verdict := core.ReviewVerdict{
		Reviewer: latest.Reviewer,
		Status:   strings.TrimSpace(latest.Verdict),
		Issues:   append([]core.ReviewIssue(nil), latest.Issues...),
		Score:    score,
	}

	return &core.ReviewResult{
		Status:   status,
		Verdicts: []core.ReviewVerdict{verdict},
		Decision: decision,
	}, nil
}

func (g *LocalReviewGate) Cancel(ctx context.Context, reviewID string) error {
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
		return errors.New("review-local cancel: review id is required")
	}

	records, err := g.store.GetReviewRecords(planID)
	if err != nil {
		return fmt.Errorf("review-local cancel list records: %w", err)
	}
	if len(records) == 0 {
		return fmt.Errorf("review-local cancel: review %q not found", planID)
	}

	latest := records[len(records)-1]
	if normalizedVerdict(latest.Verdict) == "cancelled" {
		return nil
	}

	round := latest.Round
	if round <= 0 {
		round = 1
	}

	record := &core.ReviewRecord{
		PlanID:   planID,
		Round:    round,
		Reviewer: localReviewer,
		Verdict:  "cancelled",
	}
	if err := g.store.SaveReviewRecord(record); err != nil {
		return fmt.Errorf("review-local cancel save record: %w", err)
	}
	return nil
}

func (g *LocalReviewGate) ensureReady() error {
	if g == nil {
		return errors.New("review-local gate is nil")
	}
	if g.store == nil {
		return errors.New("review-local store is nil")
	}

	g.mu.RLock()
	defer g.mu.RUnlock()
	if g.closed {
		return errors.New("review-local gate is closed")
	}
	return nil
}

func nextRound(records []core.ReviewRecord) int {
	maxRound := 0
	for _, record := range records {
		if record.Round > maxRound {
			maxRound = record.Round
		}
	}
	return maxRound + 1
}

func mapVerdict(verdict string) (status string, decision string) {
	switch normalizedVerdict(verdict) {
	case "", "pending":
		return "pending", "pending"
	case "approved", "pass":
		return "approved", "approve"
	case "rejected":
		return "rejected", "reject"
	case "changes_requested", "issues_found":
		return "changes_requested", "fix"
	case "cancelled":
		return "cancelled", "cancelled"
	default:
		unknown := strings.TrimSpace(verdict)
		if unknown == "" {
			return "pending", "pending"
		}
		return unknown, unknown
	}
}

func normalizedVerdict(verdict string) string {
	value := strings.ToLower(strings.TrimSpace(verdict))
	if value == "canceled" {
		return "cancelled"
	}
	return value
}

var _ core.ReviewGate = (*LocalReviewGate)(nil)
