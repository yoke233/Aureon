package agentcodex

import (
	"context"
	"io"
	"strings"

	"github.com/yoke233/ai-workflow/internal/core"
)

type CodexAgent struct {
	binary    string
	model     string
	reasoning string
}

func New(binary, model, reasoning string) *CodexAgent {
	return &CodexAgent{binary: binary, model: model, reasoning: reasoning}
}

func (a *CodexAgent) Name() string {
	return "codex"
}

func (a *CodexAgent) Init(_ context.Context) error {
	return nil
}

func (a *CodexAgent) Close() error {
	return nil
}

func (a *CodexAgent) BuildCommand(opts core.ExecOpts) ([]string, error) {
	prompt := opts.Prompt
	if opts.AppendContext != "" {
		prompt = opts.AppendContext + "\n\n" + prompt
	}

	sandboxMode := "workspace-write"
	if opts.Env != nil {
		if v := strings.TrimSpace(opts.Env["AI_WORKFLOW_SANDBOX"]); v != "" {
			sandboxMode = v
		}
	}

	args := []string{
		a.binary, "-a", "never",
		"exec",
		"--json",
		"--color", "never",
		"--sandbox", sandboxMode,
		"-m", a.model,
		"-c", "model_reasoning_effort=" + a.reasoning,
		"-c", "shell_environment_policy.inherit=all",
	}

	// workspace-write needs /tmp for Go build artifacts, test caches, etc.
	if sandboxMode == "workspace-write" {
		args = append(args, "--add-dir", "/tmp")
	}

	if opts.Env != nil {
		if schema := opts.Env["AI_WORKFLOW_CODEX_OUTPUT_SCHEMA"]; strings.TrimSpace(schema) != "" {
			args = append(args, "--disable", "shell_tool", "--output-schema", schema)
		}
	}
	if opts.WorkDir != "" {
		args = append(args, "-C", opts.WorkDir)
	}
	args = append(args, "--", prompt)
	return args, nil
}

func (a *CodexAgent) NewStreamParser(r io.Reader) core.StreamParser {
	return NewCodexStreamParser(r)
}
