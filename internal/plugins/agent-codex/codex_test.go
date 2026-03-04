package agentcodex

import (
	"strings"
	"testing"

	"github.com/yoke233/ai-workflow/internal/core"
)

func TestBuildCommand_DefaultSandbox(t *testing.T) {
	a := New("codex", "gpt-5.3-codex", "high")
	cmd, err := a.BuildCommand(core.ExecOpts{
		Prompt:  "fix the bug",
		WorkDir: "/tmp/project",
	})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(cmd, " ")
	for _, want := range []string{
		"exec",
		"-a never",
		"--json",
		"-m gpt-5.3-codex",
		"--sandbox workspace-write",
		"shell_environment_policy.inherit=all",
		"--add-dir /tmp",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in: %s", want, joined)
		}
	}
}

func TestBuildCommand_ReadOnlySandbox(t *testing.T) {
	a := New("codex", "gpt-5.3-codex", "high")
	cmd, err := a.BuildCommand(core.ExecOpts{
		Prompt:  "review code",
		WorkDir: "/tmp/project",
		Env:     map[string]string{"AI_WORKFLOW_SANDBOX": "read-only"},
	})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(cmd, " ")
	if !strings.Contains(joined, "--sandbox read-only") {
		t.Errorf("expected read-only sandbox, got: %s", joined)
	}
	if strings.Contains(joined, "--add-dir") {
		t.Error("read-only sandbox should not have --add-dir")
	}
}

func TestBuildCommand_WithOutputSchema_DisablesShellTool(t *testing.T) {
	a := New("codex", "gpt-5.3-codex", "high")
	cmd, err := a.BuildCommand(core.ExecOpts{
		Prompt:  "emit json",
		WorkDir: "/tmp/project",
		Env: map[string]string{
			"AI_WORKFLOW_CODEX_OUTPUT_SCHEMA": "/tmp/schema.json",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(cmd, " ")
	if !strings.Contains(joined, "--output-schema /tmp/schema.json") {
		t.Error("missing output schema flag")
	}
	if !strings.Contains(joined, "--disable shell_tool") {
		t.Error("missing disable shell_tool flag")
	}
}
