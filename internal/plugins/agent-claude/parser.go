package agentclaude

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/user/ai-workflow/internal/core"
)

func newScanner(r io.Reader) *bufio.Scanner {
	return bufio.NewScanner(r)
}

type ClaudeStreamParser struct {
	scanner *bufio.Scanner
}

func NewClaudeStreamParser(r io.Reader) *ClaudeStreamParser {
	return &ClaudeStreamParser{scanner: newScanner(r)}
}

func (p *ClaudeStreamParser) Next() (*core.StreamEvent, error) {
	for p.scanner.Scan() {
		line := p.scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var raw map[string]any
		if err := json.Unmarshal(line, &raw); err != nil {
			return &core.StreamEvent{Type: "text", Content: string(line), Timestamp: time.Now()}, nil
		}

		typ, _ := raw["type"].(string)
		switch typ {
		case "assistant":
			msg, _ := raw["message"].(map[string]any)
			contents, _ := msg["content"].([]any)
			for _, c := range contents {
				cm, _ := c.(map[string]any)
				ct, _ := cm["type"].(string)
				switch ct {
				case "text":
					text, _ := cm["text"].(string)
					return &core.StreamEvent{Type: "text", Content: text, Timestamp: time.Now()}, nil
				case "tool_use":
					name, _ := cm["name"].(string)
					input, _ := json.Marshal(cm["input"])
					return &core.StreamEvent{
						Type:      "tool_call",
						ToolName:  name,
						ToolInput: string(input),
						Timestamp: time.Now(),
					}, nil
				}
			}
		case "result":
			return &core.StreamEvent{Type: "done", Content: fmt.Sprint(raw["result"]), Timestamp: time.Now()}, nil
		}
	}
	if err := p.scanner.Err(); err != nil {
		return nil, err
	}
	return nil, io.EOF
}
