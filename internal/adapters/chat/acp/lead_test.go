package acp

import (
	"context"
	"testing"

	acpproto "github.com/coder/acp-go-sdk"
	"github.com/yoke233/ai-workflow/internal/adapters/agent/acpclient"
	membus "github.com/yoke233/ai-workflow/internal/adapters/events/memory"
	v2sandbox "github.com/yoke233/ai-workflow/internal/adapters/sandbox"
	chatapp "github.com/yoke233/ai-workflow/internal/application/chat"
	"github.com/yoke233/ai-workflow/internal/core"
)

type fakeLeadRegistry struct {
	profile       *core.AgentProfile
	driver        *core.AgentDriver
	lastResolveID string
}

func (f *fakeLeadRegistry) GetDriver(context.Context, string) (*core.AgentDriver, error) {
	return nil, core.ErrDriverNotFound
}
func (f *fakeLeadRegistry) ListDrivers(context.Context) ([]*core.AgentDriver, error) {
	return nil, nil
}
func (f *fakeLeadRegistry) CreateDriver(context.Context, *core.AgentDriver) error { return nil }
func (f *fakeLeadRegistry) UpdateDriver(context.Context, *core.AgentDriver) error { return nil }
func (f *fakeLeadRegistry) DeleteDriver(context.Context, string) error            { return nil }
func (f *fakeLeadRegistry) GetProfile(context.Context, string) (*core.AgentProfile, error) {
	return nil, core.ErrProfileNotFound
}
func (f *fakeLeadRegistry) ListProfiles(context.Context) ([]*core.AgentProfile, error) {
	return nil, nil
}
func (f *fakeLeadRegistry) CreateProfile(context.Context, *core.AgentProfile) error { return nil }
func (f *fakeLeadRegistry) UpdateProfile(context.Context, *core.AgentProfile) error { return nil }
func (f *fakeLeadRegistry) DeleteProfile(context.Context, string) error             { return nil }
func (f *fakeLeadRegistry) ResolveForStep(context.Context, *core.Step) (*core.AgentProfile, *core.AgentDriver, error) {
	return f.profile, f.driver, nil
}
func (f *fakeLeadRegistry) ResolveByID(_ context.Context, id string) (*core.AgentProfile, *core.AgentDriver, error) {
	f.lastResolveID = id
	return f.profile, f.driver, nil
}

type fakeChatACPClient struct {
	newSessionID  acpproto.SessionId
	loadSessionID acpproto.SessionId
	promptReply   string

	initializeCalls int
	newCalls        int
	loadCalls       int
	promptCalls     int

	lastLoad acpproto.LoadSessionRequest
}

func (f *fakeChatACPClient) Initialize(context.Context, acpclient.ClientCapabilities) error {
	f.initializeCalls++
	return nil
}
func (f *fakeChatACPClient) NewSession(context.Context, acpproto.NewSessionRequest) (acpproto.SessionId, error) {
	f.newCalls++
	return f.newSessionID, nil
}
func (f *fakeChatACPClient) LoadSession(_ context.Context, req acpproto.LoadSessionRequest) (acpproto.SessionId, error) {
	f.loadCalls++
	f.lastLoad = req
	return f.loadSessionID, nil
}
func (f *fakeChatACPClient) Prompt(context.Context, acpproto.PromptRequest) (*acpclient.PromptResult, error) {
	f.promptCalls++
	return &acpclient.PromptResult{Text: f.promptReply}, nil
}
func (f *fakeChatACPClient) Cancel(context.Context, acpproto.CancelNotification) error { return nil }
func (f *fakeChatACPClient) Close(context.Context) error                               { return nil }

func TestLeadAgentRestoresPersistedSession(t *testing.T) {
	registry := &fakeLeadRegistry{
		profile: &core.AgentProfile{
			ID:   "lead",
			Name: "Codex Lead",
			Role: core.RoleLead,
		},
		driver: &core.AgentDriver{
			ID:            "codex-acp",
			LaunchCommand: "fake",
		},
	}

	firstClient := &fakeChatACPClient{
		newSessionID: "acp-session-1",
		promptReply:  "first reply",
	}
	secondClient := &fakeChatACPClient{
		loadSessionID: "acp-session-1",
		promptReply:   "second reply",
	}
	clients := []ChatACPClient{firstClient, secondClient}
	newClient := func(_ acpclient.LaunchConfig, _ acpproto.Client, _opts ...acpclient.Option) (ChatACPClient, error) {
		client := clients[0]
		clients = clients[1:]
		return client, nil
	}

	cfg := LeadAgentConfig{
		Registry:  registry,
		Bus:       membus.NewBus(),
		Sandbox:   v2sandbox.NoopSandbox{},
		DataDir:   t.TempDir(),
		NewClient: newClient,
	}

	agent := NewLeadAgent(cfg)
	firstResp, err := agent.Chat(context.Background(), chatapp.Request{Message: "第一次"})
	if err != nil {
		t.Fatalf("first chat: %v", err)
	}
	if firstResp.SessionID != "acp-session-1" {
		t.Fatalf("first session_id = %q, want acp-session-1", firstResp.SessionID)
	}
	if firstResp.WSPath != "/api/ws?session_id=acp-session-1&types=chat.output" {
		t.Fatalf("first ws_path = %q", firstResp.WSPath)
	}
	agent.Shutdown()

	agent = NewLeadAgent(cfg)
	secondResp, err := agent.Chat(context.Background(), chatapp.Request{
		SessionID: "acp-session-1",
		Message:   "继续聊",
	})
	if err != nil {
		t.Fatalf("second chat: %v", err)
	}
	if secondResp.SessionID != "acp-session-1" {
		t.Fatalf("second session_id = %q, want acp-session-1", secondResp.SessionID)
	}
	if secondClient.loadCalls != 1 {
		t.Fatalf("load calls = %d, want 1", secondClient.loadCalls)
	}
	if string(secondClient.lastLoad.SessionId) != "acp-session-1" {
		t.Fatalf("loaded session = %q, want acp-session-1", secondClient.lastLoad.SessionId)
	}

	detail, err := agent.GetSession(context.Background(), "acp-session-1")
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if len(detail.Messages) != 4 {
		t.Fatalf("message count = %d, want 4", len(detail.Messages))
	}
	if detail.WSPath != "/api/ws?session_id=acp-session-1&types=chat.output" {
		t.Fatalf("detail ws_path = %q", detail.WSPath)
	}
	if detail.ProfileID != "lead" || detail.ProfileName != "Codex Lead" {
		t.Fatalf("unexpected profile info: %+v", detail.SessionSummary)
	}
	if detail.Messages[0].Role != "user" || detail.Messages[1].Role != "assistant" {
		t.Fatalf("unexpected history: %+v", detail.Messages)
	}
}

func TestLeadAgentPersistsProjectAndProfileSelection(t *testing.T) {
	registry := &fakeLeadRegistry{
		profile: &core.AgentProfile{
			ID:   "lead-alt",
			Name: "Claude Lead",
			Role: core.RoleLead,
		},
		driver: &core.AgentDriver{
			ID:            "codex-acp",
			LaunchCommand: "fake",
		},
	}
	client := &fakeChatACPClient{
		newSessionID: "acp-session-2",
		promptReply:  "reply",
	}

	agent := NewLeadAgent(LeadAgentConfig{
		Registry: registry,
		Bus:      membus.NewBus(),
		Sandbox:  v2sandbox.NoopSandbox{},
		DataDir:  t.TempDir(),
		NewClient: func(_ acpclient.LaunchConfig, _ acpproto.Client, _opts ...acpclient.Option) (ChatACPClient, error) {
			return client, nil
		},
		ProfileID: "lead",
	})

	resp, err := agent.Chat(context.Background(), chatapp.Request{
		Message:     "hello",
		ProjectID:   42,
		ProjectName: "Alpha",
		ProfileID:   "lead-alt",
	})
	if err != nil {
		t.Fatalf("chat: %v", err)
	}
	if registry.lastResolveID != "lead-alt" {
		t.Fatalf("resolved profile = %q, want lead-alt", registry.lastResolveID)
	}

	detail, err := agent.GetSession(context.Background(), resp.SessionID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if detail.ProjectID != 42 || detail.ProjectName != "Alpha" {
		t.Fatalf("unexpected project info: %+v", detail.SessionSummary)
	}
	if detail.ProfileID != "lead-alt" || detail.ProfileName != "Claude Lead" {
		t.Fatalf("unexpected profile info: %+v", detail.SessionSummary)
	}
}
