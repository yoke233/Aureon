package core

import (
	"context"
	"io"
	"time"
)

type ExecOpts struct {
	Prompt        string
	WorkDir       string
	AllowedTools  []string
	MaxTurns      int
	Timeout       time.Duration
	Env           map[string]string
	AppendContext string
}

type StreamEvent struct {
	Type      string    `json:"type"`
	Content   string    `json:"content"`
	ToolName  string    `json:"tool_name,omitempty"`
	ToolInput string    `json:"tool_input,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

type StreamParser interface {
	Next() (*StreamEvent, error)
}

type AgentPlugin interface {
	Plugin
	BuildCommand(opts ExecOpts) ([]string, error)
	NewStreamParser(r io.Reader) StreamParser
}

type RuntimeOpts struct {
	WorkDir string
	Env     map[string]string
	Command []string
}

type Session struct {
	ID     string
	Stdin  io.WriteCloser
	Stdout io.Reader
	Stderr io.Reader
	Wait   func() error
}

type RuntimePlugin interface {
	Plugin
	Create(ctx context.Context, opts RuntimeOpts) (*Session, error)
	Kill(sessionID string) error
}
