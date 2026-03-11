package engine

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	acpproto "github.com/coder/acp-go-sdk"
	"github.com/yoke233/ai-workflow/internal/acpclient"
	"github.com/yoke233/ai-workflow/internal/teamleader"
	"github.com/yoke233/ai-workflow/internal/v2/core"
	v2sandbox "github.com/yoke233/ai-workflow/internal/v2/sandbox"
)

// LocalSessionManager manages ACP sessions in the same process.
// This is the default mode — no external dependencies, same behavior as before.
//
// SubmitPrompt executes synchronously (blocks until prompt completes).
// WatchPrompt returns the cached result immediately.
type LocalSessionManager struct {
	pool    *ACPSessionPool
	store   core.Store
	sandbox v2sandbox.Sandbox

	mu      sync.Mutex
	handles map[string]*localHandle
	prompts map[string]*localPrompt
	nextID  int64

	activeCount atomic.Int32
	drainWg     sync.WaitGroup
}

type localHandle struct {
	pooled     *pooledACPSession
	standalone *acpclient.Client
	events     *switchingEventHandler
	sessionID  acpproto.SessionId
	agentCtx   *core.AgentContext
	reuse      bool
	flowID     int64
}

type localPrompt struct {
	id        string
	handleID  string
	execID    int64
	flowID    int64
	stepID    int64
	status    PromptState
	result    *SessionPromptResult
	err       error
	done      chan struct{} // closed when prompt completes
	events    []acpclient.SessionUpdate
	createdAt time.Time
}

// NewLocalSessionManager creates a session manager that runs agents in-process.
func NewLocalSessionManager(pool *ACPSessionPool, store core.Store, sandbox v2sandbox.Sandbox) *LocalSessionManager {
	return &LocalSessionManager{
		pool:    pool,
		store:   store,
		sandbox: sandbox,
		handles: make(map[string]*localHandle),
		prompts: make(map[string]*localPrompt),
	}
}

func (m *LocalSessionManager) nextHandleID() string {
	m.nextID++
	return fmt.Sprintf("local-%d", m.nextID)
}

// Acquire gets or creates an ACP session.
func (m *LocalSessionManager) Acquire(ctx context.Context, in SessionAcquireInput) (*SessionHandle, error) {
	sb := m.sandbox
	if sb == nil {
		sb = v2sandbox.NoopSandbox{}
	}
	scope := fmt.Sprintf("flow-%d", in.FlowID)
	if !in.Reuse {
		scope = fmt.Sprintf("flow-%d-exec-%d", in.FlowID, in.ExecID)
	}
	sandboxedLaunch, err := sb.Prepare(ctx, v2sandbox.PrepareInput{
		Profile: in.Profile,
		Driver:  in.Driver,
		Launch:  in.Launch,
		Scope:   scope,
	})
	if err != nil {
		return nil, fmt.Errorf("prepare sandbox: %w", err)
	}

	m.mu.Lock()
	handleID := m.nextHandleID()
	m.mu.Unlock()

	lh := &localHandle{
		reuse:  in.Reuse,
		flowID: in.FlowID,
	}

	if in.Reuse && m.pool != nil {
		sess, ac, err := m.pool.Acquire(ctx, acpSessionAcquireInput{
			Profile:    in.Profile,
			Driver:     in.Driver,
			Launch:     sandboxedLaunch,
			Caps:       in.Caps,
			WorkDir:    in.WorkDir,
			MCPFactory: in.MCPFactory,
			FlowID:     in.FlowID,
			StepID:     in.StepID,
			ExecID:     in.ExecID,
			IdleTTL:    in.IdleTTL,
			MaxTurns:   in.MaxTurns,
		})
		if err != nil {
			return nil, err
		}
		lh.pooled = sess
		lh.agentCtx = ac
		lh.sessionID = sess.sessionID
		lh.events = sess.events
	} else {
		switcher := &switchingEventHandler{}
		handler := teamleader.NewACPHandler(in.WorkDir, "", nil)
		handler.SetSuppressEvents(true)
		client, err := acpclient.New(sandboxedLaunch, handler,
			acpclient.WithEventHandler(switcher))
		if err != nil {
			return nil, fmt.Errorf("launch ACP agent %q: %w", in.Driver.ID, err)
		}
		if err := client.Initialize(ctx, in.Caps); err != nil {
			_ = client.Close(context.Background())
			return nil, fmt.Errorf("initialize ACP agent %q: %w", in.Driver.ID, err)
		}

		var mcpServers []acpproto.McpServer
		if in.MCPFactory != nil {
			mcpServers = in.MCPFactory(client.SupportsSSEMCP())
		}

		sid, err := client.NewSession(ctx, acpproto.NewSessionRequest{
			Cwd:        in.WorkDir,
			McpServers: mcpServers,
		})
		if err != nil {
			_ = client.Close(context.Background())
			return nil, fmt.Errorf("create ACP session: %w", err)
		}
		handler.SetSessionID(string(sid))

		lh.standalone = client
		lh.sessionID = sid
		lh.events = switcher
	}

	handle := &SessionHandle{ID: handleID}
	if lh.agentCtx != nil && lh.agentCtx.ID > 0 {
		id := lh.agentCtx.ID
		handle.AgentContextID = &id
	}
	if lh.reuse && lh.pooled != nil && lh.pooled.turns > 0 {
		handle.HasPriorTurns = true
	}

	m.mu.Lock()
	m.handles[handleID] = lh
	m.mu.Unlock()

	return handle, nil
}

// SubmitPrompt executes the prompt synchronously (local mode).
// Returns a prompt ID; the result is available immediately via WatchPrompt.
func (m *LocalSessionManager) SubmitPrompt(ctx context.Context, handle *SessionHandle, text string) (string, error) {
	m.mu.Lock()
	lh, ok := m.handles[handle.ID]
	if !ok {
		m.mu.Unlock()
		return "", fmt.Errorf("session handle %q not found", handle.ID)
	}
	promptID := fmt.Sprintf("lp-%d-%d", time.Now().UnixNano(), m.nextID)
	m.nextID++
	lp := &localPrompt{
		id:        promptID,
		handleID:  handle.ID,
		execID:    lh.flowID, // will be set properly via step context
		flowID:    lh.flowID,
		status:    PromptRunning,
		done:      make(chan struct{}),
		createdAt: time.Now().UTC(),
	}
	m.prompts[promptID] = lp
	m.mu.Unlock()

	m.activeCount.Add(1)
	m.drainWg.Add(1)

	// Execute synchronously.
	result, err := m.executePrompt(ctx, lh, text, lp)

	m.mu.Lock()
	if err != nil {
		lp.status = PromptFailed
		lp.err = err
	} else {
		lp.status = PromptDone
		lp.result = result
	}
	close(lp.done)
	m.mu.Unlock()

	m.activeCount.Add(-1)
	m.drainWg.Done()

	if err != nil {
		return promptID, err
	}
	return promptID, nil
}

func (m *LocalSessionManager) executePrompt(ctx context.Context, lh *localHandle, text string, lp *localPrompt) (*SessionPromptResult, error) {
	// Capture events for the prompt record.
	collector := &eventCollector{lp: lp, mu: &m.mu}

	if lh.events != nil {
		lh.events.Set(collector)
		defer lh.events.Set(nil)
	}

	var client *acpclient.Client
	if lh.reuse && lh.pooled != nil {
		lh.pooled.mu.Lock()
		defer lh.pooled.mu.Unlock()
		client = lh.pooled.client
	} else {
		client = lh.standalone
	}

	result, err := client.Prompt(ctx, acpproto.PromptRequest{
		SessionId: lh.sessionID,
		Prompt: []acpproto.ContentBlock{
			{Text: &acpproto.ContentBlockText{Text: text}},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("ACP prompt failed: %w", err)
	}

	if lh.reuse && lh.pooled != nil && m.pool != nil {
		m.pool.NoteTurn(ctx, lh.agentCtx, lh.pooled)
	}

	out := &SessionPromptResult{
		Text:       strings.TrimSpace(result.Text),
		StopReason: string(result.StopReason),
	}
	if result.Usage != nil {
		out.InputTokens = int64(result.Usage.InputTokens)
		out.OutputTokens = int64(result.Usage.OutputTokens)
	}
	return out, nil
}

// WatchPrompt returns the result of a completed prompt (local mode completes synchronously).
// If the prompt is still running (shouldn't happen in local mode), it waits.
func (m *LocalSessionManager) WatchPrompt(ctx context.Context, promptID string, _ int64, sink EventSink) (*SessionPromptResult, error) {
	m.mu.Lock()
	lp, ok := m.prompts[promptID]
	m.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("prompt %q not found", promptID)
	}

	// Wait for completion.
	select {
	case <-lp.done:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Replay events to sink if requested.
	if sink != nil {
		m.mu.Lock()
		events := append([]acpclient.SessionUpdate{}, lp.events...)
		m.mu.Unlock()
		for _, ev := range events {
			_ = sink.HandleSessionUpdate(ctx, ev)
		}
	}

	if lp.err != nil {
		return nil, lp.err
	}
	return lp.result, nil
}

// RecoverPrompts returns recent prompt statuses (local mode: only in-memory).
func (m *LocalSessionManager) RecoverPrompts(_ context.Context, since time.Time) ([]PromptStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var out []PromptStatus
	for _, lp := range m.prompts {
		if lp.createdAt.Before(since) {
			continue
		}
		ps := PromptStatus{
			PromptID:  lp.id,
			FlowID:    lp.flowID,
			Status:    lp.status,
			CreatedAt: lp.createdAt,
		}
		if lp.result != nil {
			ps.Result = lp.result
		}
		if lp.err != nil {
			ps.Error = lp.err.Error()
		}
		out = append(out, ps)
	}
	return out, nil
}

// Release marks a session handle as no longer active.
func (m *LocalSessionManager) Release(ctx context.Context, handle *SessionHandle) error {
	m.mu.Lock()
	lh, ok := m.handles[handle.ID]
	if ok {
		delete(m.handles, handle.ID)
	}
	m.mu.Unlock()

	if !ok || lh == nil {
		return nil
	}
	if !lh.reuse && lh.standalone != nil {
		return lh.standalone.Close(ctx)
	}
	return nil
}

// CleanupFlow releases all sessions for a flow.
func (m *LocalSessionManager) CleanupFlow(flowID int64) {
	if m.pool != nil {
		m.pool.CleanupFlow(flowID)
	}
}

// DrainActive blocks until all in-flight prompts complete.
func (m *LocalSessionManager) DrainActive(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		m.drainWg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// ActiveCount returns the number of executing prompts.
func (m *LocalSessionManager) ActiveCount() int {
	return int(m.activeCount.Load())
}

// Close shuts down all sessions.
func (m *LocalSessionManager) Close() {
	if m.pool != nil {
		m.pool.Close()
	}
	m.mu.Lock()
	for id, lh := range m.handles {
		if lh.standalone != nil {
			_ = lh.standalone.Close(context.Background())
		}
		delete(m.handles, id)
	}
	m.mu.Unlock()
}

// eventCollector captures events for a local prompt record.
type eventCollector struct {
	lp *localPrompt
	mu *sync.Mutex
}

func (c *eventCollector) HandleSessionUpdate(ctx context.Context, update acpclient.SessionUpdate) error {
	c.mu.Lock()
	c.lp.events = append(c.lp.events, update)
	c.mu.Unlock()
	return nil
}
