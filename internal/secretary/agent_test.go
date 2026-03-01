package secretary

import (
	"context"
	"io"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/user/ai-workflow/internal/core"
)

type mockAgent struct {
	opts   []core.ExecOpts
	cmd    []string
	parser core.StreamParser
}

func (a *mockAgent) Name() string { return "mock-agent" }

func (a *mockAgent) Init(context.Context) error { return nil }

func (a *mockAgent) Close() error { return nil }

func (a *mockAgent) BuildCommand(opts core.ExecOpts) ([]string, error) {
	a.opts = append(a.opts, opts)
	if len(a.cmd) == 0 {
		return []string{"mock"}, nil
	}
	return a.cmd, nil
}

func (a *mockAgent) NewStreamParser(io.Reader) core.StreamParser {
	return a.parser
}

type fakeRuntime struct {
	lastOpts core.RuntimeOpts
	session  *core.Session
}

func (r *fakeRuntime) Name() string { return "fake-runtime" }

func (r *fakeRuntime) Init(context.Context) error { return nil }

func (r *fakeRuntime) Close() error { return nil }

func (r *fakeRuntime) Kill(string) error { return nil }

func (r *fakeRuntime) Create(_ context.Context, opts core.RuntimeOpts) (*core.Session, error) {
	r.lastOpts = opts
	return r.session, nil
}

type sliceParser struct {
	events []*core.StreamEvent
	index  int
}

func (p *sliceParser) Next() (*core.StreamEvent, error) {
	if p.index >= len(p.events) {
		return nil, io.EOF
	}
	evt := p.events[p.index]
	p.index++
	return evt, nil
}

type nopWriteCloser struct{}

func (nopWriteCloser) Write(data []byte) (int, error) { return len(data), nil }

func (nopWriteCloser) Close() error { return nil }

func TestAgentDecomposeBuildsPromptAndParsesTaskPlan(t *testing.T) {
	waitCalled := false
	runtime := &fakeRuntime{
		session: &core.Session{
			ID:     "session-1",
			Stdin:  nopWriteCloser{},
			Stdout: strings.NewReader(""),
			Stderr: strings.NewReader(""),
			Wait: func() error {
				waitCalled = true
				return nil
			},
		},
	}

	output := "```json\n{\n  \"name\": \"oauth-rollout\",\n  \"tasks\": [\n    {\n      \"id\": \"task-1\",\n      \"title\": \"后端接入 OAuth\",\n      \"description\": \"完成 OAuth 登录接口并补充单测。\",\n      \"labels\": [\"backend\", \"auth\"],\n      \"depends_on\": [],\n      \"template\": \"standard\"\n    },\n    {\n      \"id\": \"task-2\",\n      \"title\": \"审计日志落库\",\n      \"description\": \"记录登录审计日志并提供查询接口。\",\n      \"labels\": [\"backend\", \"database\"],\n      \"depends_on\": [\"task-1\"],\n      \"template\": \"full\"\n    }\n  ]\n}\n```"
	agent := &mockAgent{
		cmd: []string{"mock-secretary"},
		parser: &sliceParser{
			events: []*core.StreamEvent{
				{Type: "done", Content: output},
			},
		},
	}

	templatePath := filepath.Join("..", "..", "configs", "prompts", "secretary.tmpl")
	driver, err := NewAgentWithTemplatePath(agent, runtime, templatePath)
	if err != nil {
		t.Fatalf("new secretary agent: %v", err)
	}

	req := Request{
		Conversation:                "用户希望新增 OAuth 登录并补充审计日志。",
		ProjectName:                 "ai-workflow",
		TechStack:                   "Go + SQLite",
		RepoPath:                    "D:/project/ai-workflow",
		OriginalConversationSummary: "用户希望增加 OAuth 登录与审计日志能力。",
		PreviousTaskPlanJSON:        `{"name":"oauth-v1","tasks":[{"id":"task-1","title":"旧任务"}]}`,
		AIReviewSummaryJSON:         `{"rounds":2,"last_decision":"fix","top_issues":["coverage_gap"]}`,
		HumanFeedbackJSON:           `{"category":"coverage_gap","detail":"上一版遗漏了审计日志相关任务","expected_direction":"补齐日志任务并明确依赖"}`,
		WorkDir:                     "D:/project/ai-workflow",
	}

	plan, err := driver.Decompose(context.Background(), req)
	if err != nil {
		t.Fatalf("decompose failed: %v", err)
	}

	if !waitCalled {
		t.Fatal("session.Wait must be called")
	}
	if len(agent.opts) != 1 {
		t.Fatalf("BuildCommand should be called once, got %d", len(agent.opts))
	}
	if agent.opts[0].WorkDir != req.WorkDir {
		t.Fatalf("exec opts workdir mismatch, got %q", agent.opts[0].WorkDir)
	}
	if agent.opts[0].MaxTurns <= 0 {
		t.Fatalf("max turns should be set, got %d", agent.opts[0].MaxTurns)
	}
	if !reflect.DeepEqual(agent.opts[0].AllowedTools, []string{"Read(*)"}) {
		t.Fatalf("allowed tools mismatch: %#v", agent.opts[0].AllowedTools)
	}

	prompt := agent.opts[0].Prompt
	for _, s := range []string{
		"输入 1：原始对话摘要",
		"输入 2：上一版 TaskPlan（完整 JSON）",
		"输入 3：AI review 问题摘要（结构化）",
		"输入 4：人类反馈（标准化 JSON）",
		req.OriginalConversationSummary,
		req.PreviousTaskPlanJSON,
		req.AIReviewSummaryJSON,
		req.HumanFeedbackJSON,
		req.Conversation,
		req.ProjectName,
		req.TechStack,
		req.RepoPath,
	} {
		if !strings.Contains(prompt, s) {
			t.Fatalf("prompt must include %q, got:\n%s", s, prompt)
		}
	}

	if runtime.lastOpts.WorkDir != req.WorkDir {
		t.Fatalf("runtime workdir mismatch, got %q", runtime.lastOpts.WorkDir)
	}
	if !reflect.DeepEqual(runtime.lastOpts.Command, []string{"mock-secretary"}) {
		t.Fatalf("runtime command mismatch: %#v", runtime.lastOpts.Command)
	}

	if plan.Name != "oauth-rollout" {
		t.Fatalf("unexpected plan name: %q", plan.Name)
	}
	if plan.Status != core.PlanDraft {
		t.Fatalf("expected status draft, got %q", plan.Status)
	}
	if plan.FailPolicy != core.FailBlock {
		t.Fatalf("expected fail_policy block, got %q", plan.FailPolicy)
	}
	if len(plan.Tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(plan.Tasks))
	}
	if plan.Tasks[0].ID != "task-1" || plan.Tasks[0].Template != "standard" {
		t.Fatalf("unexpected first task: %#v", plan.Tasks[0])
	}
	if plan.Tasks[1].ID != "task-2" || plan.Tasks[1].Template != "full" {
		t.Fatalf("unexpected second task: %#v", plan.Tasks[1])
	}
	if plan.Tasks[1].Status != core.ItemPending {
		t.Fatalf("expected pending status, got %q", plan.Tasks[1].Status)
	}
}

func TestParseTaskPlanRejectsInvalidTemplate(t *testing.T) {
	_, err := ParseTaskPlan(`{
  "name": "invalid-plan",
  "tasks": [
    {
      "id": "task-1",
      "title": "bad template",
      "description": "this should fail",
      "labels": ["backend"],
      "depends_on": [],
      "template": "unsupported-template"
    }
  ]
}`)
	if err == nil {
		t.Fatal("expected parse error for invalid template")
	}
	if !strings.Contains(err.Error(), "invalid template") {
		t.Fatalf("unexpected error: %v", err)
	}
}
