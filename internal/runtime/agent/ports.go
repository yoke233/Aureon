package agentruntime

import "github.com/yoke233/zhanggui/internal/core"

// AgentContextStore is the minimal persistence port required by ACP session reuse.
type AgentContextStore interface {
	core.AgentContextStore
}

// RunRouteStore is the minimal persistence port required by remote executor workers.
type RunRouteStore interface {
	core.RunStore
	core.AgentContextStore
}

// ThreadSessionStore is the minimal persistence port required by thread agent sessions.
type ThreadSessionStore interface {
	core.ThreadStore
	core.WorkItemStore
	core.ProjectStore
	core.ResourceSpaceStore
}
