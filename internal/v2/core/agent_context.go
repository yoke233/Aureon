package core

import "time"

// AgentContext tracks conversational state for an agent within a Flow.
type AgentContext struct {
	ID           int64     `json:"id"`
	AgentID      string    `json:"agent_id"`
	FlowID       int64     `json:"flow_id"`
	SystemPrompt string    `json:"system_prompt,omitempty"`
	SessionID    string    `json:"session_id,omitempty"` // ACP session
	Summary      string    `json:"summary,omitempty"`
	TurnCount    int       `json:"turn_count"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
