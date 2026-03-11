package engine

import (
	"context"
	"time"

	acpproto "github.com/coder/acp-go-sdk"
	"github.com/yoke233/ai-workflow/internal/acpclient"
	"github.com/yoke233/ai-workflow/internal/v2/core"
)

// SessionManager abstracts ACP agent session lifecycle management.
//
// Two modes:
//   - Local (default): in-process, wraps ACPSessionPool. No external dependencies.
//   - NATS (opt-in): async prompt submission via NATS JetStream. Agents survive
//     server restarts. Supports multiple remote executors with queue-group load balancing.
type SessionManager interface {
	// Acquire gets or creates an ACP session for the given agent+flow.
	Acquire(ctx context.Context, in SessionAcquireInput) (*SessionHandle, error)

	// SubmitPrompt submits a prompt for execution. Returns a prompt ID.
	// In local mode this executes synchronously and the result is available immediately.
	// In NATS mode this publishes to JetStream and returns before execution starts.
	SubmitPrompt(ctx context.Context, handle *SessionHandle, text string) (string, error)

	// WatchPrompt subscribes to events for a submitted prompt. Blocks until the
	// prompt completes or ctx is cancelled. Events are forwarded to sink.
	// Can reconnect with lastEventSeq to resume from where we left off.
	WatchPrompt(ctx context.Context, promptID string, lastEventSeq int64, sink EventSink) (*SessionPromptResult, error)

	// RecoverPrompts returns the status of all prompts that were active or completed
	// since the given timestamp. Called after server restart to resume tracking.
	RecoverPrompts(ctx context.Context, since time.Time) ([]PromptStatus, error)

	// Release marks a session handle as no longer active.
	Release(ctx context.Context, handle *SessionHandle) error

	// CleanupFlow releases all sessions for a completed/failed flow.
	CleanupFlow(flowID int64)

	// DrainActive blocks until all in-flight prompts complete (for graceful upgrade).
	DrainActive(ctx context.Context) error

	// ActiveCount returns the number of currently executing prompts.
	ActiveCount() int

	// Close shuts down all managed sessions.
	Close()
}

// EventSink receives streaming events during prompt execution.
type EventSink interface {
	HandleSessionUpdate(ctx context.Context, update acpclient.SessionUpdate) error
}

// SessionAcquireInput contains everything needed to acquire an agent session.
type SessionAcquireInput struct {
	Profile *core.AgentProfile
	Driver  *core.AgentDriver
	Launch  acpclient.LaunchConfig
	Caps    acpclient.ClientCapabilities
	WorkDir string

	// MCPFactory resolves MCP servers after connecting to the agent.
	// Local mode only; remote executors use their own MCP resolver.
	MCPFactory func(agentSupportsSSE bool) []acpproto.McpServer

	FlowID int64
	StepID int64
	ExecID int64

	Reuse    bool
	IdleTTL  time.Duration
	MaxTurns int
}

// SessionHandle is an opaque reference to an acquired session.
type SessionHandle struct {
	ID             string // opaque handle identifier
	AgentContextID *int64 // persisted context ID (for execution record)
	HasPriorTurns  bool   // whether session had prior prompts
}

// SessionPromptResult contains the outcome of a prompt execution.
type SessionPromptResult struct {
	Text         string
	StopReason   string
	InputTokens  int64
	OutputTokens int64
}

// PromptStatus represents the state of a prompt (for recovery after restart).
type PromptStatus struct {
	PromptID  string
	ExecID    int64
	FlowID    int64
	StepID    int64
	Status    PromptState
	Result    *SessionPromptResult
	Error     string
	CreatedAt time.Time
}

// PromptState is the lifecycle state of a submitted prompt.
type PromptState string

const (
	PromptPending PromptState = "pending"
	PromptRunning PromptState = "running"
	PromptDone    PromptState = "done"
	PromptFailed  PromptState = "failed"
)
