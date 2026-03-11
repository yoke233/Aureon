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
// Implementations:
//   - LocalSessionManager: in-process, wraps ACPSessionPool (default)
//   - RemoteSessionManager: connects to an external session daemon over Unix socket,
//     allowing agent processes to survive server restarts
type SessionManager interface {
	// Acquire gets or creates an ACP session for the given agent+flow.
	Acquire(ctx context.Context, in SessionAcquireInput) (*SessionHandle, error)

	// Prompt sends a prompt to an acquired session.
	// Events are streamed to sink during execution.
	Prompt(ctx context.Context, handle *SessionHandle, text string, sink EventSink) (*SessionPromptResult, error)

	// Release marks a handle as no longer active.
	// Pooled sessions stay alive for reuse; standalone sessions are closed.
	Release(ctx context.Context, handle *SessionHandle) error

	// CleanupFlow releases all sessions for a completed/failed flow.
	CleanupFlow(flowID int64)

	// DrainActive blocks until all in-flight prompts complete (for graceful upgrade).
	DrainActive(ctx context.Context) error

	// ActiveCount returns the number of currently executing prompts.
	ActiveCount() int

	// Close shuts down all managed sessions and agent processes.
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

	// MCPFactory resolves MCP servers after agent capabilities are known.
	// Used by local mode; remote mode uses its own resolver.
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
	HasPriorTurns  bool   // whether session had prior prompts (for prompt building)
}

// SessionPromptResult contains the outcome of a prompt execution.
type SessionPromptResult struct {
	Text         string
	StopReason   string
	InputTokens  int64
	OutputTokens int64
}
