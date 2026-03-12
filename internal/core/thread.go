package core

import (
	"context"
	"time"
)

// ThreadStatus represents the lifecycle state of a Thread.
type ThreadStatus string

const (
	ThreadActive   ThreadStatus = "active"
	ThreadClosed   ThreadStatus = "closed"
	ThreadArchived ThreadStatus = "archived"
)

// Thread is an independent multi-participant discussion container.
// Unlike ChatSession (1:1 direct chat), a Thread supports multiple
// AI agents and multiple human participants in shared discussion.
type Thread struct {
	ID         int64          `json:"id"`
	Title      string         `json:"title"`
	Status     ThreadStatus   `json:"status"`
	OwnerID    string         `json:"owner_id,omitempty"`
	Summary    string         `json:"summary,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
}

// ThreadFilter constrains Thread queries.
type ThreadFilter struct {
	Status *ThreadStatus
	Limit  int
	Offset int
}

// ThreadMessage is a single message within a Thread.
type ThreadMessage struct {
	ID        int64          `json:"id"`
	ThreadID  int64          `json:"thread_id"`
	SenderID  string         `json:"sender_id"`
	Role      string         `json:"role"` // "human" or "agent"
	Content   string         `json:"content"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

// ThreadParticipant represents a participant in a Thread.
type ThreadParticipant struct {
	ID       int64     `json:"id"`
	ThreadID int64     `json:"thread_id"`
	UserID   string    `json:"user_id"`
	Role     string    `json:"role"` // "owner", "member", "agent"
	JoinedAt time.Time `json:"joined_at"`
}

// ThreadStore persists Thread aggregates.
type ThreadStore interface {
	CreateThread(ctx context.Context, thread *Thread) (int64, error)
	GetThread(ctx context.Context, id int64) (*Thread, error)
	ListThreads(ctx context.Context, filter ThreadFilter) ([]*Thread, error)
	UpdateThread(ctx context.Context, thread *Thread) error
	DeleteThread(ctx context.Context, id int64) error

	CreateThreadMessage(ctx context.Context, msg *ThreadMessage) (int64, error)
	ListThreadMessages(ctx context.Context, threadID int64, limit, offset int) ([]*ThreadMessage, error)

	AddThreadParticipant(ctx context.Context, p *ThreadParticipant) (int64, error)
	ListThreadParticipants(ctx context.Context, threadID int64) ([]*ThreadParticipant, error)
	RemoveThreadParticipant(ctx context.Context, threadID int64, userID string) error
}
