package acp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	acpproto "github.com/coder/acp-go-sdk"
	"github.com/yoke233/ai-workflow/internal/adapters/agent/acpclient"
	eventbridge "github.com/yoke233/ai-workflow/internal/adapters/events/bridge"
	v2sandbox "github.com/yoke233/ai-workflow/internal/adapters/sandbox"
	chatapp "github.com/yoke233/ai-workflow/internal/application/chat"
	"github.com/yoke233/ai-workflow/internal/core"
)

const (
	defaultLeadProfileID  = "lead"
	defaultLeadTimeout    = 120 * time.Second
	defaultSessionIdleTTL = 30 * time.Minute
)

type LeadAgentConfig struct {
	Registry  core.AgentRegistry
	Bus       core.EventBus
	ProfileID string
	Timeout   time.Duration
	IdleTTL   time.Duration
	Sandbox   v2sandbox.Sandbox
	DataDir   string
	NewClient func(cfg acpclient.LaunchConfig, h acpproto.Client, opts ...acpclient.Option) (ChatACPClient, error)
}

type LeadAgent struct {
	cfg LeadAgentConfig

	mu          sync.Mutex
	sessions    map[string]*leadSession
	catalog     map[string]*persistedLeadSession
	catalogPath string

	activeMu   sync.Mutex
	activeRuns map[string]context.CancelFunc
}

type leadSession struct {
	client    ChatACPClient
	sessionID acpproto.SessionId
	bridge    *eventbridge.EventBridge
	events    *suppressibleEventHandler
	workDir   string
	scope     string

	mu        sync.Mutex
	idleTimer *time.Timer
	closed    bool
}

type ChatACPClient interface {
	Initialize(ctx context.Context, caps acpclient.ClientCapabilities) error
	NewSession(ctx context.Context, req acpproto.NewSessionRequest) (acpproto.SessionId, error)
	LoadSession(ctx context.Context, req acpproto.LoadSessionRequest) (acpproto.SessionId, error)
	Prompt(ctx context.Context, req acpproto.PromptRequest) (*acpclient.PromptResult, error)
	Cancel(ctx context.Context, req acpproto.CancelNotification) error
	Close(ctx context.Context) error
}

type suppressibleEventHandler struct {
	mu       sync.RWMutex
	suppress bool
	inner    acpclient.EventHandler
	onUpdate func(acpclient.SessionUpdate)
}

func (h *suppressibleEventHandler) SetSuppress(v bool) {
	h.mu.Lock()
	h.suppress = v
	h.mu.Unlock()
}

func (h *suppressibleEventHandler) SetUpdateCallback(cb func(acpclient.SessionUpdate)) {
	h.mu.Lock()
	h.onUpdate = cb
	h.mu.Unlock()
}

func (h *suppressibleEventHandler) HandleSessionUpdate(ctx context.Context, update acpclient.SessionUpdate) error {
	h.mu.RLock()
	suppress := h.suppress
	inner := h.inner
	onUpdate := h.onUpdate
	h.mu.RUnlock()
	if onUpdate != nil {
		onUpdate(update)
	}
	if suppress || inner == nil {
		return nil
	}
	return inner.HandleSessionUpdate(ctx, update)
}

func NewLeadAgent(cfg LeadAgentConfig) *LeadAgent {
	if cfg.ProfileID == "" {
		cfg.ProfileID = defaultLeadProfileID
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = defaultLeadTimeout
	}
	if cfg.IdleTTL <= 0 {
		cfg.IdleTTL = defaultSessionIdleTTL
	}
	if cfg.NewClient == nil {
		cfg.NewClient = func(launch acpclient.LaunchConfig, h acpproto.Client, opts ...acpclient.Option) (ChatACPClient, error) {
			return acpclient.New(launch, h, opts...)
		}
	}

	catalogPath := ""
	if strings.TrimSpace(cfg.DataDir) != "" {
		catalogPath = filepath.Join(cfg.DataDir, leadSessionCatalogFileName)
	}
	catalog, err := loadLeadCatalog(catalogPath)
	if err != nil {
		slog.Warn("lead chat: load catalog failed", "path", catalogPath, "error", err)
		catalog = map[string]*persistedLeadSession{}
	}

	return &LeadAgent{
		cfg:         cfg,
		sessions:    make(map[string]*leadSession),
		catalog:     catalog,
		catalogPath: catalogPath,
		activeRuns:  make(map[string]context.CancelFunc),
	}
}

func (l *LeadAgent) Chat(ctx context.Context, req chatapp.Request) (*chatapp.Response, error) {
	sess, publicSessionID, message, err := l.prepareChat(ctx, req)
	if err != nil {
		return nil, err
	}

	reply, err := l.runPrompt(ctx, publicSessionID, sess, message)
	if err != nil {
		return nil, err
	}

	return &chatapp.Response{
		SessionID: publicSessionID,
		Reply:     reply,
		WSPath:    buildChatWSPath(publicSessionID),
	}, nil
}

func (l *LeadAgent) StartChat(ctx context.Context, req chatapp.Request) (*chatapp.AcceptedResponse, error) {
	sess, publicSessionID, message, err := l.prepareChat(ctx, req)
	if err != nil {
		return nil, err
	}

	go func() {
		if _, runErr := l.runPrompt(context.Background(), publicSessionID, sess, message); runErr != nil {
			sess.bridge.PublishData(context.Background(), map[string]any{
				"type":    "error",
				"content": runErr.Error(),
			})
			slog.Warn("lead chat async prompt failed", "session_id", publicSessionID, "error", runErr)
		}
	}()

	return &chatapp.AcceptedResponse{
		SessionID: publicSessionID,
		WSPath:    buildChatWSPath(publicSessionID),
	}, nil
}

func (l *LeadAgent) prepareChat(ctx context.Context, req chatapp.Request) (*leadSession, string, string, error) {
	message := strings.TrimSpace(req.Message)
	if message == "" {
		return nil, "", "", errors.New("message is required")
	}

	workDir, err := resolveLeadWorkDir(req.WorkDir)
	if err != nil {
		return nil, "", "", err
	}

	sess, publicSessionID, err := l.getOrCreateSession(ctx, req, workDir)
	if err != nil {
		return nil, "", "", err
	}
	sess.stopIdleTimer()

	return sess, publicSessionID, message, nil
}

func (l *LeadAgent) runPrompt(ctx context.Context, publicSessionID string, sess *leadSession, message string) (string, error) {
	if sess == nil {
		return "", errors.New("session is not initialized")
	}

	promptCtx, promptCancel := context.WithTimeout(ctx, l.cfg.Timeout)
	if err := l.beginRun(publicSessionID, promptCancel); err != nil {
		promptCancel()
		l.resetSessionIdle(publicSessionID, sess)
		return "", err
	}
	defer l.endRun(publicSessionID)
	defer promptCancel()

	l.appendMessage(publicSessionID, "user", message)

	result, err := sess.client.Prompt(promptCtx, acpproto.PromptRequest{
		SessionId: sess.sessionID,
		Prompt: []acpproto.ContentBlock{
			{Text: &acpproto.ContentBlockText{Text: message}},
		},
	})

	sess.bridge.FlushPending(ctx)

	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			l.resetSessionIdle(publicSessionID, sess)
		} else {
			l.removeSession(publicSessionID)
		}
		return "", fmt.Errorf("prompt failed: %w", err)
	}
	if result == nil {
		l.removeSession(publicSessionID)
		return "", errors.New("empty result from agent")
	}

	reply := strings.TrimSpace(result.Text)
	if reply == "" {
		l.removeSession(publicSessionID)
		return "", errors.New("empty reply from agent")
	}

	sess.bridge.PublishData(ctx, map[string]any{
		"type":    "done",
		"content": reply,
	})

	l.appendMessage(publicSessionID, "assistant", reply)
	l.resetSessionIdle(publicSessionID, sess)
	return reply, nil
}

func (l *LeadAgent) beginRun(sessionID string, cancel context.CancelFunc) error {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return errors.New("session_id is required")
	}

	l.activeMu.Lock()
	defer l.activeMu.Unlock()
	if _, exists := l.activeRuns[sessionID]; exists {
		return errors.New("session is already running")
	}
	l.activeRuns[sessionID] = cancel
	return nil
}

func (l *LeadAgent) endRun(sessionID string) {
	l.activeMu.Lock()
	delete(l.activeRuns, strings.TrimSpace(sessionID))
	l.activeMu.Unlock()
}

func (l *LeadAgent) ListSessions(context.Context) ([]chatapp.SessionSummary, error) {
	running := l.runningSessionSet()

	l.mu.Lock()
	defer l.mu.Unlock()

	items := make([]chatapp.SessionSummary, 0, len(l.catalog))
	for _, record := range l.catalog {
		if record == nil || strings.TrimSpace(record.SessionID) == "" {
			continue
		}
		live := false
		if sess, ok := l.sessions[record.SessionID]; ok && !sess.isClosed() {
			live = true
		}
		items = append(items, buildSessionSummary(record, live, running[record.SessionID]))
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].UpdatedAt.Equal(items[j].UpdatedAt) {
			return items[i].SessionID < items[j].SessionID
		}
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})
	return items, nil
}

func (l *LeadAgent) GetSession(_ context.Context, sessionID string) (*chatapp.SessionDetail, error) {
	id := strings.TrimSpace(sessionID)
	if id == "" {
		return nil, errors.New("session_id is required")
	}
	running := l.runningSessionSet()

	l.mu.Lock()
	defer l.mu.Unlock()

	record, ok := l.catalog[id]
	if !ok {
		return nil, core.ErrNotFound
	}
	live := false
	if sess, ok := l.sessions[id]; ok && !sess.isClosed() {
		live = true
	}

	detail := &chatapp.SessionDetail{
		SessionSummary:    buildSessionSummary(record, live, running[id]),
		Messages:          append([]chatapp.Message(nil), record.Messages...),
		AvailableCommands: cloneAvailableCommands(record.AvailableCommands),
		ConfigOptions:     cloneConfigOptions(record.ConfigOptions),
	}
	return detail, nil
}

func (l *LeadAgent) CancelChat(sessionID string) error {
	id := strings.TrimSpace(sessionID)
	if id == "" {
		return errors.New("session_id is required")
	}

	l.activeMu.Lock()
	cancel, ok := l.activeRuns[id]
	l.activeMu.Unlock()
	if !ok {
		return errors.New("session is not running")
	}
	cancel()

	l.mu.Lock()
	sess := l.sessions[id]
	l.mu.Unlock()
	if sess != nil {
		cancelCtx, c := context.WithTimeout(context.Background(), 3*time.Second)
		defer c()
		_ = sess.client.Cancel(cancelCtx, acpproto.CancelNotification{SessionId: sess.sessionID})
	}
	return nil
}

func (l *LeadAgent) CloseSession(sessionID string) {
	l.removeSession(strings.TrimSpace(sessionID))
}

func (l *LeadAgent) Shutdown() {
	l.mu.Lock()
	sessions := make([]*leadSession, 0, len(l.sessions))
	for id, sess := range l.sessions {
		sessions = append(sessions, sess)
		delete(l.sessions, id)
	}
	l.mu.Unlock()

	for _, sess := range sessions {
		sess.close()
	}
}

func (l *LeadAgent) IsSessionAlive(sessionID string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	sess, ok := l.sessions[strings.TrimSpace(sessionID)]
	return ok && !sess.isClosed()
}

func (l *LeadAgent) IsSessionRunning(sessionID string) bool {
	l.activeMu.Lock()
	defer l.activeMu.Unlock()
	_, ok := l.activeRuns[strings.TrimSpace(sessionID)]
	return ok
}

func (l *LeadAgent) getOrCreateSession(ctx context.Context, req chatapp.Request, workDir string) (*leadSession, string, error) {
	requestedSessionID := strings.TrimSpace(req.SessionID)
	if requestedSessionID != "" {
		l.mu.Lock()
		if sess, ok := l.sessions[requestedSessionID]; ok && !sess.isClosed() {
			l.mu.Unlock()
			return sess, requestedSessionID, nil
		}
		record := l.cloneRecordLocked(requestedSessionID)
		l.mu.Unlock()

		if record == nil {
			return nil, "", core.ErrNotFound
		}
		if strings.TrimSpace(workDir) == "" {
			workDir = record.WorkDir
		}
		sess, err := l.loadSession(ctx, record, workDir)
		if err != nil {
			return nil, "", err
		}
		return sess, requestedSessionID, nil
	}

	sess, sessionID, err := l.createSession(ctx, workDir, req.ProjectID, req.ProjectName, req.ProfileID, req.DriverID)
	if err != nil {
		return nil, "", err
	}
	return sess, sessionID, nil
}

func (l *LeadAgent) createSession(ctx context.Context, workDir string, projectID int64, projectName, profileID, driverID string) (*leadSession, string, error) {
	scope := fmt.Sprintf("lead-chat-%d", time.Now().UnixNano())

	client, bridge, events, profile, driver, err := l.launchClient(ctx, workDir, scope, "", profileID, driverID)
	if err != nil {
		return nil, "", err
	}

	initCtx, initCancel := context.WithTimeout(ctx, 30*time.Second)
	defer initCancel()

	acpSessionID, err := client.NewSession(initCtx, acpproto.NewSessionRequest{
		Cwd:        workDir,
		McpServers: []acpproto.McpServer{},
	})
	if err != nil {
		_ = client.Close(context.Background())
		return nil, "", fmt.Errorf("create lead session: %w", err)
	}

	publicID := strings.TrimSpace(string(acpSessionID))
	if publicID == "" {
		_ = client.Close(context.Background())
		return nil, "", errors.New("create lead session returned empty session id")
	}

	sess := &leadSession{
		client:    client,
		sessionID: acpSessionID,
		bridge:    bridge,
		events:    events,
		workDir:   workDir,
		scope:     scope,
	}
	bridge.SetSessionID(publicID)

	l.mu.Lock()
	if old, ok := l.sessions[publicID]; ok {
		go old.close()
	}
	l.sessions[publicID] = sess
	now := time.Now().UTC()
	record := l.catalog[publicID]
	if record == nil {
		record = &persistedLeadSession{
			SessionID: publicID,
			CreatedAt: now,
		}
		l.catalog[publicID] = record
	}
	record.Scope = scope
	record.WorkDir = workDir
	record.ProjectID = projectID
	record.ProjectName = strings.TrimSpace(projectName)
	record.ProfileID = profile.ID
	record.ProfileName = strings.TrimSpace(profile.Name)
	record.DriverID = strings.TrimSpace(driver.ID)
	record.AvailableCommands = nil
	record.ConfigOptions = nil
	record.UpdatedAt = now
	_ = l.saveCatalogLocked()
	l.mu.Unlock()
	events.SetUpdateCallback(func(update acpclient.SessionUpdate) {
		l.captureSessionState(publicID, update)
	})

	slog.Info("runtime lead session created", "session_id", publicID, "profile", profile.ID, "driver", driver.ID)
	return sess, publicID, nil
}

func (l *LeadAgent) loadSession(ctx context.Context, record *persistedLeadSession, workDir string) (*leadSession, error) {
	if record == nil || strings.TrimSpace(record.SessionID) == "" {
		return nil, core.ErrNotFound
	}
	if strings.TrimSpace(record.Scope) == "" {
		return nil, fmt.Errorf("session %s has no persisted scope", record.SessionID)
	}
	if strings.TrimSpace(workDir) == "" {
		workDir = record.WorkDir
	}
	if workDir == "" {
		var err error
		workDir, err = resolveLeadWorkDir("")
		if err != nil {
			return nil, err
		}
	}

	client, bridge, events, _, _, err := l.launchClient(ctx, workDir, record.Scope, record.SessionID, record.ProfileID, record.DriverID)
	if err != nil {
		return nil, err
	}
	events.SetUpdateCallback(func(update acpclient.SessionUpdate) {
		l.captureSessionState(record.SessionID, update)
	})

	events.SetSuppress(true)
	initCtx, initCancel := context.WithTimeout(ctx, 30*time.Second)
	defer initCancel()

	loadedID, err := client.LoadSession(initCtx, acpproto.LoadSessionRequest{
		SessionId:  acpproto.SessionId(record.SessionID),
		Cwd:        workDir,
		McpServers: []acpproto.McpServer{},
	})
	events.SetSuppress(false)
	if err != nil {
		_ = client.Close(context.Background())
		return nil, fmt.Errorf("load lead session %s: %w", record.SessionID, err)
	}
	if strings.TrimSpace(string(loadedID)) == "" {
		loadedID = acpproto.SessionId(record.SessionID)
	}

	sess := &leadSession{
		client:    client,
		sessionID: loadedID,
		bridge:    bridge,
		events:    events,
		workDir:   workDir,
		scope:     record.Scope,
	}

	l.mu.Lock()
	if old, ok := l.sessions[record.SessionID]; ok {
		go old.close()
	}
	l.sessions[record.SessionID] = sess
	stored := l.catalog[record.SessionID]
	if stored != nil {
		stored.WorkDir = workDir
		stored.UpdatedAt = time.Now().UTC()
		_ = l.saveCatalogLocked()
	}
	l.mu.Unlock()

	slog.Info("runtime lead session loaded", "session_id", record.SessionID, "scope", record.Scope)
	return sess, nil
}

func (l *LeadAgent) launchClient(ctx context.Context, workDir, scope, publicSessionID, requestedProfileID, requestedDriverID string) (ChatACPClient, *eventbridge.EventBridge, *suppressibleEventHandler, *core.AgentProfile, *core.AgentDriver, error) {
	if l.cfg.Registry == nil {
		return nil, nil, nil, nil, nil, errors.New("agent registry is not configured")
	}

	profileID := strings.TrimSpace(requestedProfileID)
	if profileID == "" {
		profileID = l.cfg.ProfileID
	}
	profile, driver, err := l.cfg.Registry.ResolveByID(ctx, profileID)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("resolve lead profile %q: %w", profileID, err)
	}
	driverID := strings.TrimSpace(requestedDriverID)
	if driverID != "" && !strings.EqualFold(driver.ID, driverID) {
		overrideDriver, driverErr := l.cfg.Registry.GetDriver(ctx, driverID)
		if driverErr != nil {
			return nil, nil, nil, nil, nil, fmt.Errorf("resolve lead driver %q: %w", driverID, driverErr)
		}
		clonedProfile := *profile
		clonedProfile.DriverID = overrideDriver.ID
		profile = &clonedProfile
		driver = overrideDriver
	}

	launchCfg := acpclient.LaunchConfig{
		Command: driver.LaunchCommand,
		Args:    driver.LaunchArgs,
		WorkDir: workDir,
		Env:     cloneEnv(driver.Env),
	}

	bridge := eventbridge.New(l.cfg.Bus, core.EventChatOutput, eventbridge.Scope{
		SessionID: publicSessionID,
	})
	events := &suppressibleEventHandler{inner: bridge}

	sb := l.cfg.Sandbox
	if sb == nil {
		sb = v2sandbox.NoopSandbox{}
	}
	sandboxedLaunch, sbErr := sb.Prepare(ctx, v2sandbox.PrepareInput{
		Profile: profile,
		Driver:  driver,
		Launch:  launchCfg,
		Scope:   scope,
	})
	if sbErr != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("prepare sandbox: %w", sbErr)
	}
	launchCfg = sandboxedLaunch

	client, err := l.cfg.NewClient(launchCfg, &acpclient.NopHandler{}, acpclient.WithEventHandler(events))
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("launch lead agent: %w", err)
	}

	caps := profile.EffectiveCapabilities()
	initCaps := acpclient.ClientCapabilities{
		FSRead:   caps.FSRead,
		FSWrite:  caps.FSWrite,
		Terminal: caps.Terminal,
	}

	initCtx, initCancel := context.WithTimeout(ctx, 30*time.Second)
	defer initCancel()

	if err := client.Initialize(initCtx, initCaps); err != nil {
		_ = client.Close(context.Background())
		return nil, nil, nil, nil, nil, fmt.Errorf("initialize lead agent: %w", err)
	}

	return client, bridge, events, profile, driver, nil
}

func (l *LeadAgent) removeSession(sessionID string) {
	if sessionID == "" {
		return
	}
	l.mu.Lock()
	sess, ok := l.sessions[sessionID]
	if ok {
		delete(l.sessions, sessionID)
	}
	l.mu.Unlock()
	if sess != nil {
		sess.close()
	}
}

func (l *LeadAgent) resetSessionIdle(sessionID string, sess *leadSession) {
	sess.resetIdleTimer(l.cfg.IdleTTL, func() {
		l.removeSession(sessionID)
	})
}

func (l *LeadAgent) appendMessage(sessionID, role, content string) {
	content = strings.TrimSpace(content)
	if sessionID == "" || content == "" {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	record := l.catalog[sessionID]
	if record == nil {
		now := time.Now().UTC()
		record = &persistedLeadSession{
			SessionID: sessionID,
			CreatedAt: now,
			UpdatedAt: now,
		}
		l.catalog[sessionID] = record
	}
	record.Messages = append(record.Messages, chatapp.Message{
		Role:    role,
		Content: content,
		Time:    time.Now().UTC(),
	})
	if record.Title == "" && role == "user" {
		record.Title = buildLeadTitle(content)
	}
	record.UpdatedAt = time.Now().UTC()
	_ = l.saveCatalogLocked()
}

func (l *LeadAgent) cloneRecordLocked(sessionID string) *persistedLeadSession {
	record := l.catalog[sessionID]
	if record == nil {
		return nil
	}
	cloned := *record
	cloned.Messages = append([]chatapp.Message(nil), record.Messages...)
	return &cloned
}

func (l *LeadAgent) saveCatalogLocked() error {
	return saveLeadCatalog(l.catalogPath, l.catalog)
}

func (s *leadSession) stopIdleTimer() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.idleTimer != nil {
		s.idleTimer.Stop()
		s.idleTimer = nil
	}
}

func (s *leadSession) resetIdleTimer(d time.Duration, onExpire func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	if s.idleTimer != nil {
		s.idleTimer.Stop()
	}
	s.idleTimer = time.AfterFunc(d, onExpire)
}

func (s *leadSession) isClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

func (s *leadSession) close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	if s.idleTimer != nil {
		s.idleTimer.Stop()
		s.idleTimer = nil
	}
	client := s.client
	s.mu.Unlock()

	if client != nil {
		closeCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = client.Close(closeCtx)
	}
}

func cloneEnv(in map[string]string) map[string]string {
	if in == nil {
		return map[string]string{}
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func resolveLeadWorkDir(workDir string) (string, error) {
	if strings.TrimSpace(workDir) == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("resolve working directory: %w", err)
		}
		workDir = cwd
	}
	abs, err := filepath.Abs(workDir)
	if err != nil {
		return "", fmt.Errorf("resolve working directory %q: %w", workDir, err)
	}
	return abs, nil
}

func buildLeadTitle(message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return "新会话"
	}
	runes := []rune(message)
	if len(runes) > 24 {
		return string(runes[:24])
	}
	return message
}

func (l *LeadAgent) runningSessionSet() map[string]bool {
	l.activeMu.Lock()
	defer l.activeMu.Unlock()
	out := make(map[string]bool, len(l.activeRuns))
	for sessionID := range l.activeRuns {
		out[sessionID] = true
	}
	return out
}

func buildSessionSummary(record *persistedLeadSession, live, running bool) chatapp.SessionSummary {
	status := "closed"
	if running {
		status = "running"
	} else if live {
		status = "alive"
	}
	return chatapp.SessionSummary{
		SessionID:    record.SessionID,
		Title:        record.Title,
		WorkDir:      record.WorkDir,
		WSPath:       buildChatWSPath(record.SessionID),
		ProjectID:    record.ProjectID,
		ProjectName:  record.ProjectName,
		ProfileID:    record.ProfileID,
		ProfileName:  record.ProfileName,
		DriverID:     record.DriverID,
		CreatedAt:    record.CreatedAt,
		UpdatedAt:    record.UpdatedAt,
		Status:       status,
		MessageCount: len(record.Messages),
	}
}

func (l *LeadAgent) captureSessionState(sessionID string, update acpclient.SessionUpdate) {
	id := strings.TrimSpace(sessionID)
	if id == "" {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	record := l.catalog[id]
	if record == nil {
		return
	}

	changed := false
	switch strings.TrimSpace(update.Type) {
	case "available_commands_update":
		record.AvailableCommands = toChatAvailableCommands(update.Commands)
		changed = true
	case "config_option_update", "config_options_update":
		record.ConfigOptions = toChatConfigOptions(update.ConfigOptions)
		changed = true
	}
	if !changed {
		return
	}
	record.UpdatedAt = time.Now().UTC()
	_ = l.saveCatalogLocked()
}

func toChatAvailableCommands(items []acpproto.AvailableCommand) []chatapp.AvailableCommand {
	if items == nil {
		return nil
	}
	out := make([]chatapp.AvailableCommand, 0, len(items))
	for _, item := range items {
		cmd := chatapp.AvailableCommand{
			Name:        strings.TrimSpace(item.Name),
			Description: strings.TrimSpace(item.Description),
		}
		if item.Input != nil && item.Input.Unstructured != nil {
			cmd.Input = &chatapp.AvailableCommandInput{
				Hint: strings.TrimSpace(item.Input.Unstructured.Hint),
			}
		}
		out = append(out, cmd)
	}
	return out
}

func toChatConfigOptions(items []acpproto.SessionConfigOptionSelect) []chatapp.ConfigOption {
	if items == nil {
		return nil
	}
	out := make([]chatapp.ConfigOption, 0, len(items))
	for _, item := range items {
		option := chatapp.ConfigOption{
			ID:           strings.TrimSpace(string(item.Id)),
			Name:         strings.TrimSpace(item.Name),
			Type:         strings.TrimSpace(item.Type),
			CurrentValue: strings.TrimSpace(string(item.CurrentValue)),
		}
		if item.Description != nil {
			option.Description = strings.TrimSpace(*item.Description)
		}
		if item.Category != nil {
			option.Category = normalizeConfigCategory(item.Category)
		}
		if item.Options.Ungrouped != nil {
			for _, value := range *item.Options.Ungrouped {
				option.Options = append(option.Options, chatapp.ConfigOptionValue{
					Value:       strings.TrimSpace(string(value.Value)),
					Name:        strings.TrimSpace(value.Name),
					Description: derefTrim(value.Description),
				})
			}
		}
		if item.Options.Grouped != nil {
			for _, group := range *item.Options.Grouped {
				for _, value := range group.Options {
					option.Options = append(option.Options, chatapp.ConfigOptionValue{
						Value:       strings.TrimSpace(string(value.Value)),
						Name:        strings.TrimSpace(value.Name),
						Description: derefTrim(value.Description),
						GroupID:     strings.TrimSpace(string(group.Group)),
						GroupName:   strings.TrimSpace(group.Name),
					})
				}
			}
		}
		out = append(out, option)
	}
	return out
}

func derefTrim(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func normalizeConfigCategory(category *acpproto.SessionConfigOptionCategory) string {
	if category == nil || category.Other == nil {
		return ""
	}
	return strings.TrimSpace(string(*category.Other))
}

func buildChatWSPath(sessionID string) string {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return ""
	}
	query := url.Values{}
	query.Set("types", string(core.EventChatOutput))
	query.Set("session_id", sessionID)
	return "/api/ws?" + query.Encode()
}
