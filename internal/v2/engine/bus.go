package engine

import (
	"context"
	"sync"

	"github.com/yoke233/ai-workflow/internal/v2/core"
)

// MemBus is an in-memory channel-based EventBus implementation.
type MemBus struct {
	mu   sync.RWMutex
	subs []*memSub
}

type memSub struct {
	types  map[core.EventType]struct{}
	ch     chan core.Event
	cancel func()
	done   bool
}

// NewMemBus creates a new in-memory EventBus.
func NewMemBus() *MemBus {
	return &MemBus{}
}

// Publish sends an event to all matching subscribers.
func (b *MemBus) Publish(_ context.Context, event core.Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, sub := range b.subs {
		if sub.done {
			continue
		}
		if len(sub.types) > 0 {
			if _, ok := sub.types[event.Type]; !ok {
				continue
			}
		}
		select {
		case sub.ch <- event:
		default:
			// Drop if buffer full — non-blocking.
		}
	}
}

// Subscribe creates a new subscription. If opts.Types is empty, all events are received.
func (b *MemBus) Subscribe(opts core.SubscribeOpts) *core.Subscription {
	bufSize := opts.BufferSize
	if bufSize <= 0 {
		bufSize = 64
	}
	ch := make(chan core.Event, bufSize)

	types := make(map[core.EventType]struct{}, len(opts.Types))
	for _, t := range opts.Types {
		types[t] = struct{}{}
	}

	sub := &memSub{
		types: types,
		ch:    ch,
	}

	b.mu.Lock()
	sub.cancel = func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		sub.done = true
		close(ch)
	}
	b.subs = append(b.subs, sub)
	b.mu.Unlock()

	return &core.Subscription{
		C:      ch,
		Cancel: sub.cancel,
	}
}
