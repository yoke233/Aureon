package agentcodex

import (
	"context"
	"io"

	"github.com/user/ai-workflow/internal/core"
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

	args := []string{
		a.binary, "exec", prompt,
		"--sandbox", "workspace-write",
		"-a", "never",
		"-m", a.model,
		"-c", "model_reasoning_effort=" + a.reasoning,
	}
	if opts.WorkDir != "" {
		args = append(args, "-C", opts.WorkDir)
	}
	return args, nil
}

func (a *CodexAgent) NewStreamParser(r io.Reader) core.StreamParser {
	return NewCodexStreamParser(r)
}
