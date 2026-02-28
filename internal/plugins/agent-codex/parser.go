package agentcodex

import (
	"bufio"
	"io"
	"time"

	"github.com/user/ai-workflow/internal/core"
)

type CodexStreamParser struct {
	scanner *bufio.Scanner
}

func NewCodexStreamParser(r io.Reader) *CodexStreamParser {
	return &CodexStreamParser{scanner: bufio.NewScanner(r)}
}

func (p *CodexStreamParser) Next() (*core.StreamEvent, error) {
	if p.scanner.Scan() {
		line := p.scanner.Text()
		if line == "" {
			return p.Next()
		}
		return &core.StreamEvent{Type: "text", Content: line, Timestamp: time.Now()}, nil
	}
	if err := p.scanner.Err(); err != nil {
		return nil, err
	}
	return nil, io.EOF
}
