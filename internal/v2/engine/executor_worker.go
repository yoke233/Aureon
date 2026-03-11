package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	acpproto "github.com/coder/acp-go-sdk"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/yoke233/ai-workflow/internal/acpclient"
	"github.com/yoke233/ai-workflow/internal/teamleader"
	"github.com/yoke233/ai-workflow/internal/v2/core"
	v2sandbox "github.com/yoke233/ai-workflow/internal/v2/sandbox"
)

// ExecutorWorkerConfig configures a remote executor worker.
type ExecutorWorkerConfig struct {
	// NATSConn is an already-connected NATS connection.
	NATSConn *nats.Conn

	// StreamPrefix is the JetStream stream name prefix (default: "aiworkflow").
	StreamPrefix string

	// AgentTypes are the agent driver IDs this worker can handle (e.g., ["claude", "codex"]).
	// If empty, the worker consumes from all agent types ("*").
	AgentTypes []string

	// Store is used for agent context persistence.
	Store core.Store

	// Registry resolves agent profiles and drivers.
	Registry core.AgentRegistry

	// Sandbox provides optional per-process isolation.
	Sandbox v2sandbox.Sandbox

	// MCPResolver resolves MCP servers for an agent profile.
	MCPResolver func(profileID string, agentSupportsSSE bool) []acpproto.McpServer

	// MCPEnv provides MCP environment configuration.
	MCPEnv teamleader.MCPEnvConfig

	// DefaultWorkDir is the fallback working directory.
	DefaultWorkDir string

	// MaxConcurrent limits parallel prompt execution. Default: 2.
	MaxConcurrent int
}

// ExecutorWorker consumes prompt messages from NATS JetStream, executes them locally
// via ACP agents, and publishes results + events back to NATS.
type ExecutorWorker struct {
	cfg    ExecutorWorkerConfig
	js     jetstream.JetStream
	prefix string
	pool   *ACPSessionPool

	mu      sync.Mutex
	running int
	cancel  context.CancelFunc
}

// NewExecutorWorker creates a new remote executor worker.
func NewExecutorWorker(cfg ExecutorWorkerConfig) (*ExecutorWorker, error) {
	if cfg.NATSConn == nil {
		return nil, fmt.Errorf("NATS connection is required")
	}

	prefix := strings.TrimSpace(cfg.StreamPrefix)
	if prefix == "" {
		prefix = "aiworkflow"
	}

	js, err := jetstream.New(cfg.NATSConn)
	if err != nil {
		return nil, fmt.Errorf("create JetStream context: %w", err)
	}

	maxConc := cfg.MaxConcurrent
	if maxConc <= 0 {
		maxConc = 2
	}
	cfg.MaxConcurrent = maxConc

	var pool *ACPSessionPool
	if cfg.Store != nil {
		pool = NewACPSessionPool(cfg.Store, nil)
	}

	return &ExecutorWorker{
		cfg:    cfg,
		js:     js,
		prefix: prefix,
		pool:   pool,
	}, nil
}

// Start begins consuming prompt messages. Blocks until ctx is cancelled.
func (w *ExecutorWorker) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	w.cancel = cancel
	defer cancel()

	// Determine subjects to consume.
	subjects := w.buildSubjects()
	slog.Info("executor worker: starting",
		"subjects", subjects, "max_concurrent", w.cfg.MaxConcurrent)

	// Create durable consumer with queue group for load balancing.
	consumer, err := w.js.CreateOrUpdateConsumer(ctx, w.prefix+"_prompts", jetstream.ConsumerConfig{
		Durable:       w.prefix + "_executor",
		FilterSubject: subjects[0], // primary subject filter
		AckPolicy:     jetstream.AckExplicitPolicy,
		MaxAckPending: w.cfg.MaxConcurrent,
		AckWait:       10 * time.Minute, // allow up to 10 min for prompt execution
	})
	if err != nil {
		return fmt.Errorf("create consumer: %w", err)
	}

	// Consume messages with concurrency control.
	sem := make(chan struct{}, w.cfg.MaxConcurrent)
	for {
		msgs, err := consumer.Fetch(1, jetstream.FetchMaxWait(5*time.Second))
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			continue
		}

		for msg := range msgs.Messages() {
			sem <- struct{}{}
			go func(m jetstream.Msg) {
				defer func() { <-sem }()
				w.handleMessage(ctx, m)
			}(msg)
		}

		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
}

func (w *ExecutorWorker) buildSubjects() []string {
	if len(w.cfg.AgentTypes) == 0 {
		return []string{fmt.Sprintf("%s.prompt.submit.>", w.prefix)}
	}
	subjects := make([]string, 0, len(w.cfg.AgentTypes))
	for _, at := range w.cfg.AgentTypes {
		subjects = append(subjects, fmt.Sprintf("%s.prompt.submit.%s", w.prefix, at))
	}
	return subjects
}

func (w *ExecutorWorker) handleMessage(ctx context.Context, msg jetstream.Msg) {
	var prompt natsPromptMessage
	if err := json.Unmarshal(msg.Data(), &prompt); err != nil {
		slog.Error("executor worker: invalid prompt message", "error", err)
		_ = msg.Nak()
		return
	}

	slog.Info("executor worker: executing prompt",
		"prompt_id", prompt.PromptID, "agent", prompt.AgentID, "flow_id", prompt.FlowID)

	// Create event forwarder that publishes to NATS.
	eventSeq := int64(0)
	eventSubject := fmt.Sprintf("%s.prompt.events.%s", w.prefix, prompt.PromptID)
	eventForwarder := &natsEventForwarder{
		js:      w.js,
		subject: eventSubject,
		seq:     &eventSeq,
	}

	result, execErr := w.executePrompt(ctx, &prompt, eventForwarder)

	// Publish result.
	resultMsg := natsPromptResult{
		PromptID: prompt.PromptID,
	}
	if execErr != nil {
		resultMsg.Error = execErr.Error()
	} else if result != nil {
		resultMsg.Text = result.Text
		resultMsg.StopReason = result.StopReason
		resultMsg.InputTokens = result.InputTokens
		resultMsg.OutputTokens = result.OutputTokens
	}

	resultData, _ := json.Marshal(resultMsg)
	resultSubject := fmt.Sprintf("%s.prompt.result.%s", w.prefix, prompt.PromptID)
	if _, err := w.js.Publish(ctx, resultSubject, resultData); err != nil {
		slog.Error("executor worker: failed to publish result",
			"prompt_id", prompt.PromptID, "error", err)
	}

	_ = msg.Ack()

	if execErr != nil {
		slog.Error("executor worker: prompt failed",
			"prompt_id", prompt.PromptID, "error", execErr)
	} else {
		slog.Info("executor worker: prompt completed",
			"prompt_id", prompt.PromptID, "output_len", len(resultMsg.Text))
	}
}

func (w *ExecutorWorker) executePrompt(ctx context.Context, prompt *natsPromptMessage, eventHandler acpclient.EventHandler) (*SessionPromptResult, error) {
	workDir := prompt.WorkDir
	if workDir == "" {
		workDir = w.cfg.DefaultWorkDir
	}

	// Resolve the agent driver configuration.
	agentID := prompt.AgentID
	profiles, err := w.cfg.Registry.ListProfiles(ctx)
	if err != nil {
		return nil, fmt.Errorf("list profiles: %w", err)
	}

	var profile *core.AgentProfile
	var driver *core.AgentDriver
	for _, p := range profiles {
		if p.DriverID == agentID || p.ID == agentID {
			profile = p
			break
		}
	}
	if profile == nil {
		return nil, fmt.Errorf("agent profile not found for %q", agentID)
	}

	driver, err = w.cfg.Registry.GetDriver(ctx, profile.DriverID)
	if err != nil {
		return nil, fmt.Errorf("get driver %q: %w", profile.DriverID, err)
	}

	launchCfg := acpclient.LaunchConfig{
		Command: driver.LaunchCommand,
		Args:    driver.LaunchArgs,
		WorkDir: workDir,
		Env:     cloneEnv(driver.Env),
	}

	sb := w.cfg.Sandbox
	if sb == nil {
		sb = v2sandbox.NoopSandbox{}
	}
	sandboxedLaunch, err := sb.Prepare(ctx, v2sandbox.PrepareInput{
		Profile: profile,
		Driver:  driver,
		Launch:  launchCfg,
		Scope:   fmt.Sprintf("flow-%d-exec-%d", prompt.FlowID, prompt.ExecID),
	})
	if err != nil {
		return nil, fmt.Errorf("prepare sandbox: %w", err)
	}

	caps := profile.EffectiveCapabilities()
	acpCaps := acpclient.ClientCapabilities{
		FSRead:   caps.FSRead,
		FSWrite:  caps.FSWrite,
		Terminal: caps.Terminal,
	}

	switcher := &switchingEventHandler{}
	switcher.Set(eventHandler)

	handler := teamleader.NewACPHandler(workDir, "", nil)
	handler.SetSuppressEvents(true)
	client, err := acpclient.New(sandboxedLaunch, handler,
		acpclient.WithEventHandler(switcher))
	if err != nil {
		return nil, fmt.Errorf("launch ACP agent %q: %w", driver.ID, err)
	}
	defer client.Close(context.Background())

	if err := client.Initialize(ctx, acpCaps); err != nil {
		return nil, fmt.Errorf("initialize ACP agent %q: %w", driver.ID, err)
	}

	var mcpServers []acpproto.McpServer
	if w.cfg.MCPResolver != nil {
		mcpServers = w.cfg.MCPResolver(profile.ID, client.SupportsSSEMCP())
	}

	sid, err := client.NewSession(ctx, acpproto.NewSessionRequest{
		Cwd:        workDir,
		McpServers: mcpServers,
	})
	if err != nil {
		return nil, fmt.Errorf("create ACP session: %w", err)
	}
	handler.SetSessionID(string(sid))

	result, err := client.Prompt(ctx, acpproto.PromptRequest{
		SessionId: sid,
		Prompt: []acpproto.ContentBlock{
			{Text: &acpproto.ContentBlockText{Text: prompt.Text}},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("ACP prompt failed: %w", err)
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

// Stop gracefully shuts down the executor worker.
func (w *ExecutorWorker) Stop() {
	if w.cancel != nil {
		w.cancel()
	}
	if w.pool != nil {
		w.pool.Close()
	}
}

// natsEventForwarder publishes ACP events to NATS JetStream.
type natsEventForwarder struct {
	js      jetstream.JetStream
	subject string
	seq     *int64
}

func (f *natsEventForwarder) HandleSessionUpdate(ctx context.Context, update acpclient.SessionUpdate) error {
	*f.seq++
	msg := natsEventMessage{
		Seq:    *f.seq,
		Update: update,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = f.js.Publish(ctx, f.subject, data)
	return err
}
