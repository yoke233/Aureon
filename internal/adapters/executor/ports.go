package executor

import "github.com/yoke233/zhanggui/internal/core"

// Store is the minimal persistence port required by ACP action execution.
type Store interface {
	core.WorkItemStore
	core.ActionSignalStore
	core.UsageStore
}
