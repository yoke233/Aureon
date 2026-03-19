package probe

import membus "github.com/yoke233/zhanggui/internal/adapters/events/memory"

func NewMemBus() *membus.Bus {
	return membus.NewBus()
}
