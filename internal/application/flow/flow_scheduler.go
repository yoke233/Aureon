package flow

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/yoke233/zhanggui/internal/core"
)

type dependentWorkItemReader interface {
	ListDependentWorkItems(ctx context.Context, workItemID int64) ([]*core.WorkItem, error)
}

// WorkItemScheduler manages a queue of WorkItems and limits concurrent execution.
// API callers submit WorkItems via Submit(); the scheduler runs them when capacity
// is available.
type WorkItemScheduler struct {
	engine *WorkItemEngine
	store  Store
	bus    EventPublisher

	maxConcurrent int // max work items running in parallel

	mu      sync.Mutex
	queue   []int64                      // work item IDs waiting to run
	running map[int64]context.CancelFunc // work item ID → cancel func
	closed  bool

	// notify is signalled when a work item finishes or a new work item is submitted.
	notify chan struct{}
	done   chan struct{} // closed when scheduler loop exits
}

// WorkItemSchedulerConfig configures the WorkItemScheduler.
type WorkItemSchedulerConfig struct {
	MaxConcurrentWorkItems int // default 2
	MaxConcurrentFlows     int // deprecated compatibility field
}

// NewWorkItemScheduler creates a multi-work-item scheduler.
func NewWorkItemScheduler(engine *WorkItemEngine, store Store, bus EventPublisher, cfg WorkItemSchedulerConfig) *WorkItemScheduler {
	if cfg.MaxConcurrentWorkItems <= 0 && cfg.MaxConcurrentFlows > 0 {
		cfg.MaxConcurrentWorkItems = cfg.MaxConcurrentFlows
	}
	if cfg.MaxConcurrentWorkItems <= 0 {
		cfg.MaxConcurrentWorkItems = 2
	}
	return &WorkItemScheduler{
		engine:        engine,
		store:         store,
		bus:           bus,
		maxConcurrent: cfg.MaxConcurrentWorkItems,
		running:       make(map[int64]context.CancelFunc),
		notify:        make(chan struct{}, 1),
		done:          make(chan struct{}),
	}
}

// IssueSchedulerConfig is a compatibility alias.
type IssueSchedulerConfig = WorkItemSchedulerConfig

// IssueScheduler is a compatibility alias.
type IssueScheduler = WorkItemScheduler

// NewIssueScheduler is an alias for backward compatibility.
func NewIssueScheduler(engine *WorkItemEngine, store Store, bus EventPublisher, cfg IssueSchedulerConfig) *WorkItemScheduler {
	return NewWorkItemScheduler(engine, store, bus, WorkItemSchedulerConfig{
		MaxConcurrentWorkItems: cfg.MaxConcurrentWorkItems,
		MaxConcurrentFlows:     cfg.MaxConcurrentFlows,
	})
}

// FlowSchedulerConfig is a compatibility wrapper for older callers.
type FlowSchedulerConfig struct {
	MaxConcurrentWorkItems int
	MaxConcurrentFlows     int
}

// NewFlowScheduler is an alias for backward compatibility.
func NewFlowScheduler(engine *WorkItemEngine, store Store, bus EventPublisher, cfg FlowSchedulerConfig) *WorkItemScheduler {
	return NewWorkItemScheduler(engine, store, bus, WorkItemSchedulerConfig{
		MaxConcurrentWorkItems: cfg.MaxConcurrentWorkItems,
		MaxConcurrentFlows:     cfg.MaxConcurrentFlows,
	})
}

// Start begins the scheduler loop. It blocks until ctx is cancelled.
func (s *WorkItemScheduler) Start(ctx context.Context) {
	defer close(s.done)

	for {
		s.dispatch(ctx)

		select {
		case <-ctx.Done():
			s.drainRunning()
			return
		case <-s.notify:
			// new submission or a work item finished — re-check
		}
	}
}

// Submit enqueues a work item for execution. The work item must be in open/accepted state.
// It transitions the work item to queued and returns immediately.
func (s *WorkItemScheduler) Submit(ctx context.Context, workItemID int64) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return fmt.Errorf("scheduler is closed")
	}
	s.mu.Unlock()

	workItem, err := s.store.GetWorkItem(ctx, workItemID)
	if err != nil {
		return fmt.Errorf("get work item %d: %w", workItemID, err)
	}
	ready, err := s.dependenciesSatisfied(ctx, workItem)
	if err != nil {
		return err
	}
	if !ready {
		if err := s.store.UpdateWorkItemStatus(ctx, workItemID, core.WorkItemAccepted); err != nil && !errors.Is(err, core.ErrNotFound) {
			return fmt.Errorf("hold work item %d until dependencies resolve: %w", workItemID, err)
		}
		return nil
	}

	// Atomically transition open/accepted, unarchived work items to queued.
	if err := s.store.PrepareWorkItemRun(ctx, workItemID, core.WorkItemQueued); err != nil {
		return fmt.Errorf("queue work item %d: %w", workItemID, err)
	}
	s.publishQueued(ctx, workItemID)

	s.mu.Lock()
	s.queue = append(s.queue, workItemID)
	s.mu.Unlock()

	s.signal()
	return nil
}

// Cancel cancels a work item. If queued, removes from queue. If running, cancels its context.
func (s *WorkItemScheduler) Cancel(ctx context.Context, workItemID int64) error {
	s.mu.Lock()

	// Check if in queue — remove it.
	for i, id := range s.queue {
		if id == workItemID {
			s.queue = append(s.queue[:i], s.queue[i+1:]...)
			s.mu.Unlock()
			// Update state to cancelled.
			if err := s.store.UpdateWorkItemStatus(ctx, workItemID, core.WorkItemCancelled); err != nil {
				return err
			}
			s.bus.Publish(ctx, core.Event{
				Type:       core.EventWorkItemCancelled,
				WorkItemID: workItemID,
				Timestamp:  time.Now().UTC(),
			})
			return nil
		}
	}

	// Check if running — cancel its context.
	cancel, ok := s.running[workItemID]
	s.mu.Unlock()

	if ok {
		cancel()
		// The engine.Run goroutine will handle state transition to cancelled/failed.
		return nil
	}

	// Fallback: delegate to engine's Cancel for direct state update.
	return s.engine.Cancel(ctx, workItemID)
}

// QueueLen returns the number of work items waiting to run.
func (s *WorkItemScheduler) QueueLen() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.queue)
}

// RunningCount returns the number of currently running work items.
func (s *WorkItemScheduler) RunningCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.running)
}

// Stats returns scheduler statistics.
func (s *WorkItemScheduler) Stats() SchedulerStats {
	s.mu.Lock()
	defer s.mu.Unlock()

	runningIDs := make([]int64, 0, len(s.running))
	for id := range s.running {
		runningIDs = append(runningIDs, id)
	}
	queuedIDs := make([]int64, len(s.queue))
	copy(queuedIDs, s.queue)

	return SchedulerStats{
		MaxConcurrent: s.maxConcurrent,
		RunningCount:  len(s.running),
		QueuedCount:   len(s.queue),
		RunningIDs:    runningIDs,
		QueuedIDs:     queuedIDs,
	}
}

// SchedulerStats holds runtime stats for the scheduler.
type SchedulerStats struct {
	MaxConcurrent int     `json:"max_concurrent"`
	RunningCount  int     `json:"running_count"`
	QueuedCount   int     `json:"queued_count"`
	RunningIDs    []int64 `json:"running_ids"`
	QueuedIDs     []int64 `json:"queued_ids"`
}

// Shutdown gracefully stops the scheduler and waits for it to finish.
func (s *WorkItemScheduler) Shutdown() {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
	// The caller should cancel the context passed to Start().
	<-s.done
}

// dispatch starts as many queued work items as capacity allows.
func (s *WorkItemScheduler) dispatch(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for len(s.queue) > 0 && len(s.running) < s.maxConcurrent {
		workItemID := s.queue[0]
		s.queue = s.queue[1:]

		wiCtx, cancel := context.WithCancel(ctx)
		s.running[workItemID] = cancel

		go s.runWorkItem(wiCtx, workItemID)
	}
}

// runWorkItem executes a single work item and cleans up when done.
func (s *WorkItemScheduler) runWorkItem(ctx context.Context, workItemID int64) {
	defer func() {
		s.mu.Lock()
		delete(s.running, workItemID)
		s.mu.Unlock()
		s.signal()
	}()

	err := s.engine.Run(ctx, workItemID)
	if err != nil {
		// If context was cancelled, mark as cancelled (not failed).
		if ctx.Err() != nil {
			_ = s.store.UpdateWorkItemStatus(context.Background(), workItemID, core.WorkItemCancelled)
			s.bus.Publish(context.Background(), core.Event{
				Type:       core.EventWorkItemCancelled,
				WorkItemID: workItemID,
				Timestamp:  time.Now().UTC(),
			})
		}
		slog.Error("work item execution failed", "work_item_id", workItemID, "error", err)
		return
	}
	s.autoQueueDependents(workItemID)
}

// signal pokes the scheduler loop to re-check capacity.
func (s *WorkItemScheduler) signal() {
	select {
	case s.notify <- struct{}{}:
	default:
	}
}

func (s *WorkItemScheduler) dependenciesSatisfied(ctx context.Context, workItem *core.WorkItem) (bool, error) {
	if workItem == nil || len(workItem.DependsOn) == 0 {
		return true, nil
	}
	for _, depID := range workItem.DependsOn {
		dep, err := s.store.GetWorkItem(ctx, depID)
		if err != nil {
			return false, fmt.Errorf("get dependency work item %d: %w", depID, err)
		}
		if dep.Status != core.WorkItemDone {
			return false, nil
		}
	}
	return true, nil
}

func (s *WorkItemScheduler) autoQueueDependents(workItemID int64) {
	reader, ok := s.store.(dependentWorkItemReader)
	if !ok {
		return
	}
	dependents, err := reader.ListDependentWorkItems(context.Background(), workItemID)
	if err != nil {
		slog.Warn("work item scheduler: list dependents failed", "work_item_id", workItemID, "error", err)
		return
	}
	for _, dependent := range dependents {
		if dependent == nil {
			continue
		}
		ready, err := s.dependenciesSatisfied(context.Background(), dependent)
		if err != nil {
			slog.Warn("work item scheduler: dependency check failed", "work_item_id", dependent.ID, "error", err)
			continue
		}
		if !ready {
			continue
		}
		if err := s.store.PrepareWorkItemRun(context.Background(), dependent.ID, core.WorkItemQueued); err != nil {
			if !errors.Is(err, core.ErrInvalidTransition) {
				slog.Warn("work item scheduler: auto queue dependent failed", "work_item_id", dependent.ID, "error", err)
			}
			continue
		}
		s.publishQueued(context.Background(), dependent.ID)
		s.mu.Lock()
		s.queue = append(s.queue, dependent.ID)
		s.mu.Unlock()
		s.signal()
	}
}

func (s *WorkItemScheduler) publishQueued(ctx context.Context, workItemID int64) {
	s.bus.Publish(ctx, core.Event{
		Type:       core.EventWorkItemQueued,
		WorkItemID: workItemID,
		Timestamp:  time.Now().UTC(),
	})
}

// drainRunning cancels all running work items and waits for them to finish.
func (s *WorkItemScheduler) drainRunning() {
	s.mu.Lock()
	for _, cancel := range s.running {
		cancel()
	}
	s.mu.Unlock()

	// Wait for all goroutines to exit.
	for {
		s.mu.Lock()
		n := len(s.running)
		s.mu.Unlock()
		if n == 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
}
