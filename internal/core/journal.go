package core

import (
	"context"
	"fmt"
	"time"
)

// JournalKind identifies the type of activity recorded.
type JournalKind string

const (
	JournalStateChange JournalKind = "state_change"
	JournalToolCall    JournalKind = "tool_call"
	JournalAgentOutput JournalKind = "agent_output"
	JournalUsage       JournalKind = "usage"
	JournalSignal      JournalKind = "signal"
	JournalFeedback    JournalKind = "feedback"
	JournalContext     JournalKind = "context"
	JournalProbe       JournalKind = "probe"
	JournalHumanAction JournalKind = "human_action"
	JournalMergeEvent  JournalKind = "merge_event"
	JournalError       JournalKind = "error"
	JournalSystem      JournalKind = "system"
)

// JournalSource identifies who produced the entry.
type JournalSource string

const (
	JournalSourceAgent  JournalSource = "agent"
	JournalSourceHuman  JournalSource = "human"
	JournalSourceSystem JournalSource = "system"
)

// JournalEntry is a single activity record in the unified journal.
type JournalEntry struct {
	ID             int64          `json:"id"`
	WorkItemID     int64          `json:"work_item_id,omitempty"`
	ActionID       int64          `json:"action_id,omitempty"`
	RunID          int64          `json:"run_id,omitempty"`
	Kind           JournalKind    `json:"kind"`
	Source         JournalSource  `json:"source"`
	Summary        string         `json:"summary,omitempty"`
	Payload        map[string]any `json:"payload,omitempty"`
	Ref            string         `json:"ref,omitempty"`
	Actor          string         `json:"actor,omitempty"`
	SourceActionID int64          `json:"source_action_id,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
}

// JournalFilter constrains journal queries.
type JournalFilter struct {
	WorkItemID *int64
	ActionID   *int64
	RunID      *int64
	Kinds      []JournalKind
	Sources    []JournalSource
	Since      *time.Time
	Until      *time.Time
	Limit      int
	Offset     int
}

// JournalStore persists and queries unified activity journal entries.
type JournalStore interface {
	AppendJournal(ctx context.Context, entry *JournalEntry) (int64, error)
	BatchAppendJournal(ctx context.Context, entries []*JournalEntry) error
	ListJournal(ctx context.Context, filter JournalFilter) ([]*JournalEntry, error)
	CountJournal(ctx context.Context, filter JournalFilter) (int, error)
	GetLatestSignal(ctx context.Context, actionID int64, signalTypes ...string) (*JournalEntry, error)
	CountSignals(ctx context.Context, actionID int64, signalTypes ...string) (int, error)
}

// ── Mapping functions: existing types → JournalEntry ──

// ActionSignalToJournalEntry converts an ActionSignal to a JournalEntry.
func ActionSignalToJournalEntry(s *ActionSignal) *JournalEntry {
	if s == nil {
		return nil
	}

	kind := signalTypeToJournalKind(s.Type)
	source := JournalSource(s.Source)

	// Build payload: merge signal's own payload with type and content.
	payload := make(map[string]any, len(s.Payload)+3)
	for k, v := range s.Payload {
		payload[k] = v
	}
	payload["signal_type"] = string(s.Type)
	if s.Content != "" {
		payload["content"] = s.Content
	}

	return &JournalEntry{
		WorkItemID:     s.WorkItemID,
		ActionID:       s.ActionID,
		RunID:          s.RunID,
		Kind:           kind,
		Source:         source,
		Summary:        s.Summary,
		Payload:        payload,
		Actor:          s.Actor,
		SourceActionID: s.SourceActionID,
		CreatedAt:      s.CreatedAt,
	}
}

func signalTypeToJournalKind(t SignalType) JournalKind {
	switch t {
	case SignalComplete, SignalNeedHelp, SignalBlocked, SignalApprove, SignalReject:
		return JournalSignal
	case SignalProgress:
		return JournalAgentOutput
	case SignalUnblock, SignalOverride, SignalInstruction:
		return JournalHumanAction
	case SignalFeedback:
		return JournalFeedback
	case SignalContext:
		return JournalContext
	case SignalProbeRequest, SignalProbeResponse:
		return JournalProbe
	default:
		return JournalSignal
	}
}

// UsageRecordToJournalEntry converts a UsageRecord to a JournalEntry.
func UsageRecordToJournalEntry(r *UsageRecord) *JournalEntry {
	if r == nil {
		return nil
	}
	return &JournalEntry{
		WorkItemID: r.WorkItemID,
		ActionID:   r.ActionID,
		RunID:      r.RunID,
		Kind:       JournalUsage,
		Source:     JournalSourceSystem,
		Summary:    fmt.Sprintf("token usage: %d tokens", r.TotalTokens),
		Payload: map[string]any{
			"agent_id":           r.AgentID,
			"profile_id":         r.ProfileID,
			"model_id":           r.ModelID,
			"input_tokens":       r.InputTokens,
			"output_tokens":      r.OutputTokens,
			"cache_read_tokens":  r.CacheReadTokens,
			"cache_write_tokens": r.CacheWriteTokens,
			"reasoning_tokens":   r.ReasoningTokens,
			"total_tokens":       r.TotalTokens,
			"duration_ms":        r.DurationMs,
		},
		Actor:     r.AgentID,
		CreatedAt: r.CreatedAt,
	}
}
