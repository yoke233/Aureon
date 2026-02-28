package agentclaude

import (
	"strings"
	"testing"

	"github.com/user/ai-workflow/internal/core"
)

func TestBuildCommand(t *testing.T) {
	a := New("claude")
	cmd, err := a.BuildCommand(core.ExecOpts{
		Prompt:       "implement feature X",
		MaxTurns:     20,
		AllowedTools: []string{"Read(*)", "Write(*)"},
	})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(cmd, " ")
	if !strings.Contains(joined, "--output-format stream-json") {
		t.Error("missing stream-json flag")
	}
	if !strings.Contains(joined, "--max-turns 20") {
		t.Error("missing max-turns")
	}
	if !strings.Contains(joined, `--allowedTools "Read(*),Write(*)"`) {
		t.Errorf("missing allowedTools, got: %s", joined)
	}
}

func TestClaudeStreamParser(t *testing.T) {
	input := `{"type":"assistant","message":{"content":[{"type":"text","text":"hello world"}]}}
{"type":"result","result":"done","duration_ms":1234}
`
	parser := &ClaudeStreamParser{scanner: newScanner(strings.NewReader(input))}

	evt, err := parser.Next()
	if err != nil {
		t.Fatal(err)
	}
	if evt.Type != "text" || evt.Content != "hello world" {
		t.Errorf("unexpected event: %+v", evt)
	}

	evt2, err := parser.Next()
	if err != nil {
		t.Fatal(err)
	}
	if evt2.Type != "done" {
		t.Errorf("expected done, got %s", evt2.Type)
	}
}
