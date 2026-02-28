package agentclaude

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/user/ai-workflow/internal/core"
)

type ClaudeAgent struct {
	binary string
}

func New(binary string) *ClaudeAgent {
	return &ClaudeAgent{binary: binary}
}

func (a *ClaudeAgent) Name() string {
	return "claude"
}

func (a *ClaudeAgent) Init(_ context.Context) error {
	return nil
}

func (a *ClaudeAgent) Close() error {
	return nil
}

func (a *ClaudeAgent) BuildCommand(opts core.ExecOpts) ([]string, error) {
	prompt := opts.Prompt
	if opts.AppendContext != "" {
		prompt = opts.AppendContext + "\n\n" + prompt
	}
	args := []string{a.binary, "-p", prompt, "--output-format", "stream-json"}
	if opts.MaxTurns > 0 {
		args = append(args, "--max-turns", fmt.Sprintf("%d", opts.MaxTurns))
	}
	if len(opts.AllowedTools) > 0 {
		args = append(args, "--allowedTools", fmt.Sprintf(`"%s"`, strings.Join(opts.AllowedTools, ",")))
	}
	return args, nil
}

func (a *ClaudeAgent) NewStreamParser(r io.Reader) core.StreamParser {
	return NewClaudeStreamParser(r)
}
