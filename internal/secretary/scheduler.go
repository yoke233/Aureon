package secretary

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/yoke233/ai-workflow/internal/core"
	"github.com/yoke233/ai-workflow/internal/engine"
)

type eventSubscriber interface {
	Subscribe() chan core.Event
	Unsubscribe(ch chan core.Event)
}

type eventPublisher interface {
	Publish(evt core.Event)
}

type pipelineRef struct {
	sessionID string
	issueID   string
}

type readyDispatch struct {
	sessionID string
	issueID   string
}

type runningSession struct {
	SessionID string
	ProjectID string
	Graph     *DAG
	Running   map[string]string
	IssueByID map[string]*core.Issue
	Parents   map[string][]string
	HaltNew   bool
	Recovered bool
}

func newRunningSession(sessionID, projectID string, issues []*core.Issue, graph *DAG) *runningSession {
	issueByID := make(map[string]*core.Issue, len(issues))
	for _, issue := range issues {
		if issue == nil {
			continue
		}
		issueByID[issue.ID] = issue
	}

	parents := make(map[string][]string, len(graph.Nodes))
	for issueID := range graph.Nodes {
		parents[issueID] = []string{}
	}
	for from, downstream := range graph.Downstream {
		for _, to := range downstream {
			parents[to] = append(parents[to], from)
		}
	}
	for issueID := range parents {
		sort.Strings(parents[issueID])
	}

	return &runningSession{
		SessionID: sessionID,
		ProjectID: projectID,
		Graph:     graph,
		Running:   make(map[string]string),
		IssueByID: issueByID,
		Parents:   parents,
	}
}

// DepScheduler schedules Issues by DAG dependencies and maps each issue to one pipeline.
type DepScheduler struct {
	store   core.Store
	bus     eventSubscriber
	pub     eventPublisher
	tracker core.Tracker

	runPipeline func(context.Context, string) error
	sem         chan struct{}

	mu            sync.Mutex
	sessions      map[string]*runningSession
	pipelineIndex map[string]pipelineRef
	lastSessionID string

	loopCancel  context.CancelFunc
	loopWG      sync.WaitGroup
	reconcileWG sync.WaitGroup

	reconcileInterval time.Duration
	reconcileRun      func(context.Context) error
}

func NewDepScheduler(
	store core.Store,
	bus eventSubscriber,
	runPipeline func(context.Context, string) error,
	tracker core.Tracker,
	maxConcurrent int,
) *DepScheduler {
	if maxConcurrent <= 0 {
		maxConcurrent = 1
	}
	if runPipeline == nil {
		runPipeline = func(context.Context, string) error { return nil }
	}

	var pub eventPublisher
	if typed, ok := bus.(eventPublisher); ok {
		pub = typed
	}

	return &DepScheduler{
		store:         store,
		bus:           bus,
		pub:           pub,
		tracker:       tracker,
		runPipeline:   runPipeline,
		sem:           make(chan struct{}, maxConcurrent),
		sessions:      make(map[string]*runningSession),
		pipelineIndex: make(map[string]pipelineRef),
	}
}

// SetReconcileRunner configures periodic reconcile hook for status drift repair.
func (s *DepScheduler) SetReconcileRunner(interval time.Duration, run func(context.Context) error) {
	if s == nil {
		return
	}
	if interval <= 0 {
		interval = 10 * time.Minute
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reconcileInterval = interval
	s.reconcileRun = run
}

func (s *DepScheduler) Start(ctx context.Context) error {
	if s == nil || s.bus == nil {
		return nil
	}

	s.mu.Lock()
	if s.loopCancel != nil {
		s.mu.Unlock()
		return nil
	}
	runCtx, cancel := context.WithCancel(ctx)
	s.loopCancel = cancel
	ch := s.bus.Subscribe()
	reconcileRun := s.reconcileRun
	reconcileInterval := s.reconcileInterval
	if reconcileInterval <= 0 {
		reconcileInterval = 10 * time.Minute
	}
	s.loopWG.Add(1)
	s.mu.Unlock()

	go func() {
		defer s.loopWG.Done()
		defer s.bus.Unsubscribe(ch)
		for {
			select {
			case <-runCtx.Done():
				return
			case evt, ok := <-ch:
				if !ok {
					return
				}
				_ = s.OnEvent(context.Background(), evt)
			}
		}
	}()

	if reconcileRun != nil {
		s.reconcileWG.Add(1)
		go func(runCtx context.Context, interval time.Duration, runFn func(context.Context) error) {
			defer s.reconcileWG.Done()
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for {
				select {
				case <-runCtx.Done():
					return
				case <-ticker.C:
					_ = runFn(context.Background())
				}
			}
		}(runCtx, reconcileInterval, reconcileRun)
	}

	return nil
}

func (s *DepScheduler) Stop(ctx context.Context) error {
	if s == nil {
		return nil
	}

	s.mu.Lock()
	cancel := s.loopCancel
	s.loopCancel = nil
	s.mu.Unlock()
	if cancel == nil {
		return nil
	}
	cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		s.loopWG.Wait()
		s.reconcileWG.Wait()
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// StartPlan is a compatibility wrapper. New code should call ScheduleIssues.
func (s *DepScheduler) StartPlan(ctx context.Context, legacy any) error {
	switch v := legacy.(type) {
	case nil:
		return errors.New("legacy start payload is nil")
	case *core.Issue:
		return s.ScheduleIssues(ctx, []*core.Issue{v})
	case core.Issue:
		issue := v
		return s.ScheduleIssues(ctx, []*core.Issue{&issue})
	case []*core.Issue:
		return s.ScheduleIssues(ctx, v)
	case []core.Issue:
		ptrs := make([]*core.Issue, 0, len(v))
		for i := range v {
			ptrs = append(ptrs, &v[i])
		}
		return s.ScheduleIssues(ctx, ptrs)
	default:
		return fmt.Errorf("unsupported legacy start payload type: %T", legacy)
	}
}

// RecoverExecutingPlans is a compatibility wrapper. New code should call RecoverExecutingIssues.
func (s *DepScheduler) RecoverExecutingPlans(ctx context.Context) error {
	return s.RecoverExecutingIssues(ctx, "")
}

// RecoverPlan is a compatibility wrapper retained for older call sites.
func (s *DepScheduler) RecoverPlan(ctx context.Context, _ string) error {
	return s.RecoverExecutingIssues(ctx, "")
}

// ScheduleIssues builds DAG, validates and transitive-reduces it, then dispatches initial ready issues.
func (s *DepScheduler) ScheduleIssues(ctx context.Context, issues []*core.Issue) error {
	if s == nil || s.store == nil {
		return errors.New("scheduler store is not configured")
	}
	if len(issues) == 0 {
		return nil
	}

	grouped, err := groupIssuesBySession(issues)
	if err != nil {
		return err
	}

	for _, sessionID := range sortedSessionIDs(grouped) {
		if err := s.scheduleSession(ctx, sessionID, grouped[sessionID]); err != nil {
			return err
		}
	}
	return nil
}

func (s *DepScheduler) scheduleSession(ctx context.Context, sessionID string, issues []*core.Issue) error {
	if len(issues) == 0 {
		return nil
	}

	s.mu.Lock()
	if _, exists := s.sessions[sessionID]; exists {
		s.mu.Unlock()
		return nil
	}
	s.mu.Unlock()

	projectID := strings.TrimSpace(issues[0].ProjectID)
	graph := Build(issues)
	if err := graph.Validate(); err != nil {
		return err
	}
	graph.TransitiveReduce()
	if err := graph.Validate(); err != nil {
		return err
	}

	rs := newRunningSession(sessionID, projectID, issues, graph)

	for _, issueID := range sortedIssueIDs(rs.IssueByID) {
		issue := rs.IssueByID[issueID]
		if issue == nil {
			continue
		}
		if issue.FailPolicy == "" {
			issue.FailPolicy = core.FailBlock
		}
		if isIssueTerminal(issue.Status) {
			continue
		}

		switch issue.Status {
		case core.IssueStatusExecuting:
			if strings.TrimSpace(issue.PipelineID) == "" {
				issue.Status = core.IssueStatusQueued
			} else {
				rs.Running[issueID] = issue.PipelineID
			}
		case core.IssueStatusReady, core.IssueStatusQueued:
		default:
			issue.Status = core.IssueStatusQueued
			issue.PipelineID = ""
		}

		if err := s.saveIssue(issue); err != nil {
			return err
		}
		if issue.Status == core.IssueStatusQueued {
			s.publishIssueEvent(core.EventIssueQueued, issue, nil, "")
		}
	}
	s.syncIssueDependencies(issues)

	if err := s.markReadyByInDegreeLocked(rs); err != nil {
		return err
	}

	if err := s.registerSessionRuntime(sessionID, rs); err != nil {
		return err
	}
	return s.dispatchReadyAcrossSessions(ctx)
}

// RecoverExecutingIssues is the crash-recovery entrypoint in issue semantics.
func (s *DepScheduler) RecoverExecutingIssues(ctx context.Context, projectID string) error {
	if s == nil || s.store == nil {
		return errors.New("scheduler store is not configured")
	}

	active, err := s.store.GetActiveIssues(strings.TrimSpace(projectID))
	if err != nil {
		return err
	}
	if len(active) == 0 {
		return nil
	}

	ptrs := make([]*core.Issue, 0, len(active))
	for i := range active {
		ptrs = append(ptrs, &active[i])
	}

	grouped, err := groupIssuesBySession(ptrs)
	if err != nil {
		return err
	}

	for _, sessionID := range sortedSessionIDs(grouped) {
		if err := s.recoverSession(ctx, sessionID, grouped[sessionID]); err != nil {
			return err
		}
	}
	return nil
}

func (s *DepScheduler) recoverSession(ctx context.Context, sessionID string, issues []*core.Issue) error {
	if len(issues) == 0 {
		return nil
	}

	projectID := strings.TrimSpace(issues[0].ProjectID)
	graph := Build(issues)
	if err := graph.Validate(); err != nil {
		return err
	}
	graph.TransitiveReduce()
	if err := graph.Validate(); err != nil {
		return err
	}

	rs := newRunningSession(sessionID, projectID, issues, graph)
	rs.Recovered = true
	replayEvents := make([]core.Event, 0)

	for _, issueID := range sortedIssueIDs(rs.IssueByID) {
		issue := rs.IssueByID[issueID]
		if issue == nil {
			continue
		}
		if issue.FailPolicy == "" {
			issue.FailPolicy = core.FailBlock
		}

		switch issue.Status {
		case core.IssueStatusDone:
			rs.unlockDownstream(issueID)
		case core.IssueStatusExecuting:
			if strings.TrimSpace(issue.PipelineID) == "" {
				issue.Status = core.IssueStatusQueued
				if err := s.saveIssue(issue); err != nil {
					return err
				}
				continue
			}
			pipeline, getErr := s.store.GetPipeline(issue.PipelineID)
			if getErr != nil {
				return fmt.Errorf("recover issue %s pipeline %s: %w", issueID, issue.PipelineID, getErr)
			}
			rs.Running[issueID] = issue.PipelineID
			if evtType, terminal := pipelineRecoveryEvent(pipeline.Status); terminal {
				replayEvents = append(replayEvents, core.Event{
					Type:       evtType,
					PipelineID: issue.PipelineID,
					Error:      pipeline.ErrorMessage,
					Timestamp:  time.Now(),
				})
			}
		case core.IssueStatusReady, core.IssueStatusQueued:
		default:
			if isIssueTerminal(issue.Status) {
				continue
			}
			issue.Status = core.IssueStatusQueued
			issue.PipelineID = ""
			if err := s.saveIssue(issue); err != nil {
				return err
			}
			s.publishIssueEvent(core.EventIssueQueued, issue, nil, "")
		}
	}

	if err := s.markReadyByInDegreeLocked(rs); err != nil {
		return err
	}
	if err := s.registerSessionRuntime(sessionID, rs); err != nil {
		return err
	}

	for i := range replayEvents {
		if err := s.OnEvent(ctx, replayEvents[i]); err != nil {
			return err
		}
	}
	if rs.HaltNew {
		return nil
	}
	return s.dispatchReadyAcrossSessions(ctx)
}

func (s *DepScheduler) registerSessionRuntime(sessionID string, rs *runningSession) error {
	if rs == nil {
		return nil
	}

	acquired := 0
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.sessions[sessionID]; exists {
		return nil
	}

	s.sessions[sessionID] = rs
	for issueID, pipelineID := range rs.Running {
		s.pipelineIndex[pipelineID] = pipelineRef{sessionID: sessionID, issueID: issueID}
		if !s.tryAcquireSlot() {
			delete(s.pipelineIndex, pipelineID)
			delete(s.sessions, sessionID)
			for acquired > 0 {
				s.releaseSlot()
				acquired--
			}
			return fmt.Errorf("recover session %s exceeds max concurrency %d", sessionID, cap(s.sem))
		}
		acquired++
	}
	return nil
}

// OnEvent handles pipeline_done/pipeline_failed events and advances Issue state.
func (s *DepScheduler) OnEvent(ctx context.Context, evt core.Event) error {
	if s == nil {
		return nil
	}
	if evt.Type != core.EventPipelineDone && evt.Type != core.EventPipelineFailed {
		return nil
	}
	if strings.TrimSpace(evt.PipelineID) == "" {
		return nil
	}

	s.mu.Lock()
	err := s.handlePipelineEventLocked(evt)
	s.mu.Unlock()
	if err != nil {
		return err
	}
	return s.dispatchReadyAcrossSessions(ctx)
}

func (s *DepScheduler) handlePipelineEventLocked(evt core.Event) error {
	ref, ok := s.pipelineIndex[evt.PipelineID]
	if !ok {
		issue, err := s.store.GetIssueByPipeline(evt.PipelineID)
		if err != nil || issue == nil {
			return err
		}
		sessionID := makeSessionID(issue.ProjectID, issue.SessionID)
		rs := s.sessions[sessionID]
		if rs == nil {
			return nil
		}
		if _, exists := rs.IssueByID[issue.ID]; !exists {
			return nil
		}
		ref = pipelineRef{sessionID: sessionID, issueID: issue.ID}
		s.pipelineIndex[evt.PipelineID] = ref
	}

	rs := s.sessions[ref.sessionID]
	if rs == nil {
		delete(s.pipelineIndex, evt.PipelineID)
		return nil
	}

	issue := rs.IssueByID[ref.issueID]
	if issue == nil {
		delete(s.pipelineIndex, evt.PipelineID)
		delete(rs.Running, ref.issueID)
		s.releaseSlot()
		return nil
	}

	switch evt.Type {
	case core.EventPipelineDone:
		issue.Status = core.IssueStatusDone
		if err := s.saveIssue(issue); err != nil {
			return err
		}
		s.publishIssueEvent(core.EventIssueDone, issue, nil, "")
		rs.unlockDownstream(issue.ID)
	case core.EventPipelineFailed:
		issue.Status = core.IssueStatusFailed
		if err := s.saveIssue(issue); err != nil {
			return err
		}
		s.publishIssueEvent(core.EventIssueFailed, issue, nil, evt.Error)
		switch issue.FailPolicy {
		case core.FailSkip:
			rs.unlockDownstream(issue.ID)
		case core.FailHuman:
			rs.HaltNew = true
		default:
			if err := s.applyBlockPolicyLocked(rs, issue.ID); err != nil {
				return err
			}
		}
	default:
		return nil
	}

	if err := s.markReadyByInDegreeLocked(rs); err != nil {
		return err
	}

	if _, running := rs.Running[ref.issueID]; running {
		delete(rs.Running, ref.issueID)
		s.releaseSlot()
	}
	delete(s.pipelineIndex, evt.PipelineID)
	return nil
}

func (s *DepScheduler) applyBlockPolicyLocked(rs *runningSession, failedIssueID string) error {
	queue := []string{failedIssueID}
	seen := map[string]struct{}{failedIssueID: {}}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		for _, downID := range rs.Graph.Downstream[current] {
			if _, ok := seen[downID]; !ok {
				seen[downID] = struct{}{}
				queue = append(queue, downID)
			}

			downIssue := rs.IssueByID[downID]
			if downIssue == nil || isIssueTerminal(downIssue.Status) || downIssue.Status == core.IssueStatusExecuting {
				continue
			}

			downIssue.Status = core.IssueStatusFailed
			if err := s.saveIssue(downIssue); err != nil {
				return err
			}
			s.publishIssueEvent(core.EventIssueFailed, downIssue, map[string]string{
				"reason":          "blocked_by_dependency_failure",
				"cause_issue_id":  failedIssueID,
				"dependency_mode": "hard",
			}, "")
		}
	}
	return nil
}

func (s *DepScheduler) markReadyByInDegreeLocked(rs *runningSession) error {
	if rs == nil {
		return nil
	}
	for _, issueID := range sortedIssueIDs(rs.IssueByID) {
		issue := rs.IssueByID[issueID]
		if issue == nil || issue.Status != core.IssueStatusQueued {
			continue
		}
		if rs.Graph.InDegree[issueID] != 0 {
			continue
		}
		issue.Status = core.IssueStatusReady
		if err := s.saveIssue(issue); err != nil {
			return err
		}
		s.publishIssueEvent(core.EventIssueReady, issue, nil, "")
	}
	return nil
}

func (s *DepScheduler) dispatchIssue(ctx context.Context, sessionID, issueID string) (bool, error) {
	if s == nil || s.store == nil {
		return false, errors.New("scheduler store is not configured")
	}
	if strings.TrimSpace(sessionID) == "" || strings.TrimSpace(issueID) == "" {
		return false, errors.New("session id and issue id are required")
	}

	s.mu.Lock()
	rs := s.sessions[sessionID]
	if rs == nil {
		s.mu.Unlock()
		return false, fmt.Errorf("session %s is not running", sessionID)
	}
	if rs.HaltNew {
		s.mu.Unlock()
		return false, nil
	}
	issue := rs.IssueByID[issueID]
	if issue == nil {
		s.mu.Unlock()
		return false, fmt.Errorf("issue %s not found in session %s", issueID, sessionID)
	}
	if issue.Status != core.IssueStatusReady {
		s.mu.Unlock()
		return false, nil
	}
	if _, running := rs.Running[issueID]; running {
		s.mu.Unlock()
		return false, nil
	}
	if !s.tryAcquireSlot() {
		s.mu.Unlock()
		return false, nil
	}

	pipeline, err := buildPipelineFromIssue(issue)
	if err != nil {
		s.releaseSlot()
		s.mu.Unlock()
		return false, err
	}

	issue.Status = core.IssueStatusExecuting
	issue.PipelineID = pipeline.ID
	rs.Running[issueID] = pipeline.ID
	s.pipelineIndex[pipeline.ID] = pipelineRef{sessionID: sessionID, issueID: issueID}
	s.lastSessionID = sessionID
	s.mu.Unlock()

	if err := s.store.SavePipeline(pipeline); err != nil {
		s.rollbackDispatch(sessionID, issueID, pipeline.ID)
		return false, err
	}
	if err := s.saveIssue(issue); err != nil {
		s.rollbackDispatch(sessionID, issueID, pipeline.ID)
		return false, err
	}
	s.publishIssueEvent(core.EventIssueExecuting, issue, nil, "")

	runCtx := context.Background()
	if ctx != nil {
		runCtx = context.WithoutCancel(ctx)
	}
	go func(runCtx context.Context, pipelineID string) {
		if runErr := s.runPipeline(runCtx, pipelineID); runErr != nil {
			_ = s.OnEvent(context.Background(), core.Event{
				Type:       core.EventPipelineFailed,
				PipelineID: pipelineID,
				Error:      runErr.Error(),
				Timestamp:  time.Now(),
			})
		}
	}(runCtx, pipeline.ID)

	return true, nil
}

func (s *DepScheduler) rollbackDispatch(sessionID, issueID, pipelineID string) {
	var issue *core.Issue

	s.mu.Lock()
	rs := s.sessions[sessionID]
	if rs != nil {
		if candidate := rs.IssueByID[issueID]; candidate != nil &&
			candidate.Status == core.IssueStatusExecuting &&
			candidate.PipelineID == pipelineID {
			candidate.Status = core.IssueStatusReady
			candidate.PipelineID = ""
			issue = candidate
		}
		delete(rs.Running, issueID)
	}
	delete(s.pipelineIndex, pipelineID)
	s.releaseSlot()
	s.mu.Unlock()

	if issue != nil {
		_ = s.saveIssue(issue)
	}
}

func (s *DepScheduler) dispatchReadyAcrossSessions(ctx context.Context) error {
	if s == nil {
		return nil
	}

	for {
		s.mu.Lock()
		if cap(s.sem) > 0 && len(s.sem) >= cap(s.sem) {
			s.mu.Unlock()
			return nil
		}
		candidates := s.globalReadyCandidatesLocked()
		s.mu.Unlock()
		if len(candidates) == 0 {
			return nil
		}

		dispatchedAny := false
		for _, candidate := range candidates {
			dispatched, err := s.dispatchIssue(ctx, candidate.sessionID, candidate.issueID)
			if err != nil {
				return err
			}
			if dispatched {
				dispatchedAny = true
			}
		}
		if !dispatchedAny {
			return nil
		}
	}
}

func (s *DepScheduler) globalReadyCandidatesLocked() []readyDispatch {
	sessionIDs := make([]string, 0, len(s.sessions))
	readyBySession := make(map[string][]string, len(s.sessions))
	maxReady := 0

	for sessionID, rs := range s.sessions {
		if rs == nil || rs.HaltNew {
			continue
		}
		ready := rs.readyToDispatchIDs()
		if len(ready) == 0 {
			continue
		}
		sessionIDs = append(sessionIDs, sessionID)
		readyBySession[sessionID] = ready
		if len(ready) > maxReady {
			maxReady = len(ready)
		}
	}
	if len(sessionIDs) == 0 {
		return nil
	}

	sort.Strings(sessionIDs)
	start := 0
	if s.lastSessionID != "" {
		idx := sort.SearchStrings(sessionIDs, s.lastSessionID)
		if idx < len(sessionIDs) && sessionIDs[idx] == s.lastSessionID {
			start = (idx + 1) % len(sessionIDs)
		} else if idx < len(sessionIDs) {
			start = idx
		}
	}

	orderedSessionIDs := append([]string{}, sessionIDs[start:]...)
	orderedSessionIDs = append(orderedSessionIDs, sessionIDs[:start]...)

	candidates := make([]readyDispatch, 0, len(sessionIDs))
	for i := 0; i < maxReady; i++ {
		for _, sessionID := range orderedSessionIDs {
			ready := readyBySession[sessionID]
			if i >= len(ready) {
				continue
			}
			candidates = append(candidates, readyDispatch{sessionID: sessionID, issueID: ready[i]})
		}
	}
	return candidates
}

func (s *DepScheduler) saveIssue(issue *core.Issue) error {
	if issue == nil {
		return nil
	}
	issue.UpdatedAt = time.Now()
	if err := s.store.SaveIssue(issue); err != nil {
		return err
	}

	if s.tracker == nil {
		return nil
	}
	if strings.TrimSpace(issue.ExternalID) == "" {
		externalID, err := s.tracker.CreateIssue(context.Background(), issue)
		if err == nil && strings.TrimSpace(externalID) != "" {
			issue.ExternalID = externalID
			issue.UpdatedAt = time.Now()
			if saveErr := s.store.SaveIssue(issue); saveErr != nil {
				return saveErr
			}
		}
	}
	if strings.TrimSpace(issue.ExternalID) != "" {
		_ = s.tracker.UpdateStatus(context.Background(), issue.ExternalID, issue.Status)
	}
	return nil
}

func (s *DepScheduler) syncIssueDependencies(issues []*core.Issue) {
	if s == nil || s.tracker == nil {
		return
	}
	for _, issue := range issues {
		if issue == nil {
			continue
		}
		_ = s.tracker.SyncDependencies(context.Background(), issue, issues)
	}
}

func (s *DepScheduler) publishEvent(evt core.Event) {
	if s == nil || s.pub == nil {
		return
	}
	if evt.Timestamp.IsZero() {
		evt.Timestamp = time.Now()
	}
	s.pub.Publish(evt)
}

func (s *DepScheduler) publishIssueEvent(eventType core.EventType, issue *core.Issue, data map[string]string, eventErr string) {
	if issue == nil {
		return
	}

	evtData := map[string]string{
		"issue_status": string(issue.Status),
	}
	for k, v := range data {
		evtData[k] = v
	}
	if eventErr != "" {
		evtData["error"] = eventErr
	}

	s.publishEvent(core.Event{
		Type:       eventType,
		PipelineID: issue.PipelineID,
		ProjectID:  issue.ProjectID,
		IssueID:    issue.ID,
		Data:       evtData,
		Error:      eventErr,
		Timestamp:  time.Now(),
	})
}

func (s *DepScheduler) tryAcquireSlot() bool {
	select {
	case s.sem <- struct{}{}:
		return true
	default:
		return false
	}
}

func (s *DepScheduler) releaseSlot() {
	select {
	case <-s.sem:
	default:
	}
}

func pipelineRecoveryEvent(status core.PipelineStatus) (core.EventType, bool) {
	switch status {
	case core.StatusDone:
		return core.EventPipelineDone, true
	case core.StatusFailed, core.StatusAborted:
		return core.EventPipelineFailed, true
	default:
		return "", false
	}
}

func (rs *runningSession) unlockDownstream(issueID string) {
	for _, downID := range rs.Graph.Downstream[issueID] {
		rs.decrementInDegree(downID)
	}
}

func (rs *runningSession) decrementInDegree(issueID string) {
	if rs.Graph.InDegree[issueID] > 0 {
		rs.Graph.InDegree[issueID]--
	}
}

func (rs *runningSession) readyToDispatchIDs() []string {
	ready := make([]string, 0, len(rs.IssueByID))
	for _, issueID := range sortedIssueIDs(rs.IssueByID) {
		issue := rs.IssueByID[issueID]
		if issue == nil {
			continue
		}
		if issue.Status != core.IssueStatusReady {
			continue
		}
		if _, running := rs.Running[issueID]; running {
			continue
		}
		ready = append(ready, issueID)
	}
	return ready
}

func buildPipelineFromIssue(issue *core.Issue) (*core.Pipeline, error) {
	if issue == nil {
		return nil, errors.New("issue cannot be nil")
	}

	template := strings.TrimSpace(issue.Template)
	if template == "" {
		template = "standard"
	}
	stages, err := buildSchedulerStages(template)
	if err != nil {
		return nil, err
	}

	name := strings.TrimSpace(issue.Title)
	if name == "" {
		name = issue.ID
	}

	now := time.Now()
	return &core.Pipeline{
		ID:              engine.NewPipelineID(),
		ProjectID:       issue.ProjectID,
		Name:            name,
		Description:     issue.Body,
		Template:        template,
		Status:          core.StatusCreated,
		Stages:          stages,
		Artifacts:       map[string]string{},
		Config:          map[string]any{},
		IssueID:         issue.ID,
		MaxTotalRetries: 5,
		QueuedAt:        now,
		CreatedAt:       now,
		UpdatedAt:       now,
	}, nil
}

func buildSchedulerStages(template string) ([]core.StageConfig, error) {
	stageIDs, ok := engine.Templates[template]
	if !ok {
		return nil, fmt.Errorf("unknown template: %s", template)
	}

	stages := make([]core.StageConfig, len(stageIDs))
	for i, stageID := range stageIDs {
		stages[i] = schedulerDefaultStageConfig(stageID)
	}
	return stages, nil
}

func schedulerDefaultStageConfig(id core.StageID) core.StageConfig {
	cfg := core.StageConfig{
		Name:           id,
		PromptTemplate: string(id),
		Timeout:        30 * time.Minute,
		MaxRetries:     1,
		OnFailure:      core.OnFailureHuman,
	}

	switch id {
	case core.StageRequirements, core.StageCodeReview:
		cfg.Agent = "codex"
	case core.StageImplement, core.StageFixup:
		cfg.Agent = "codex"
	case core.StageE2ETest:
		cfg.Agent = "codex"
		cfg.Timeout = 15 * time.Minute
	case core.StageWorktreeSetup, core.StageMerge, core.StageCleanup:
		cfg.Timeout = 2 * time.Minute
	}
	return cfg
}

func isIssueTerminal(status core.IssueStatus) bool {
	switch status {
	case core.IssueStatusDone, core.IssueStatusFailed, core.IssueStatusSuperseded, core.IssueStatusAbandoned:
		return true
	default:
		return false
	}
}

func makeSessionID(projectID, sessionID string) string {
	trimmedSessionID := strings.TrimSpace(sessionID)
	if trimmedSessionID != "" {
		return trimmedSessionID
	}
	return "project:" + strings.TrimSpace(projectID)
}

func groupIssuesBySession(issues []*core.Issue) (map[string][]*core.Issue, error) {
	grouped := make(map[string][]*core.Issue)
	sessionProject := make(map[string]string)

	for _, issue := range issues {
		if issue == nil {
			continue
		}
		issueID := strings.TrimSpace(issue.ID)
		projectID := strings.TrimSpace(issue.ProjectID)
		if issueID == "" {
			return nil, errors.New("issue id is required")
		}
		if projectID == "" {
			return nil, fmt.Errorf("issue %s project id is required", issueID)
		}

		issue.ID = issueID
		issue.ProjectID = projectID
		issue.SessionID = strings.TrimSpace(issue.SessionID)

		sessionID := makeSessionID(projectID, issue.SessionID)
		if existingProjectID, ok := sessionProject[sessionID]; ok && existingProjectID != projectID {
			return nil, fmt.Errorf("session %s has mixed project ids: %s vs %s", sessionID, existingProjectID, projectID)
		}
		sessionProject[sessionID] = projectID
		grouped[sessionID] = append(grouped[sessionID], issue)
	}

	if len(grouped) == 0 {
		return nil, errors.New("no issues provided")
	}
	return grouped, nil
}

func sortedSessionIDs(grouped map[string][]*core.Issue) []string {
	sessionIDs := make([]string, 0, len(grouped))
	for sessionID := range grouped {
		sessionIDs = append(sessionIDs, sessionID)
	}
	sort.Strings(sessionIDs)
	return sessionIDs
}

func sortedIssueIDs(issueByID map[string]*core.Issue) []string {
	ids := make([]string, 0, len(issueByID))
	for issueID := range issueByID {
		ids = append(ids, issueID)
	}
	sort.Strings(ids)
	return ids
}
