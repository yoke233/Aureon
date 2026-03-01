package core

import "context"

// PluginSlot identifies one of the ten pluggable extension points.
type PluginSlot string

const (
	SlotAgent      PluginSlot = "agent"
	SlotRuntime    PluginSlot = "runtime"
	SlotWorkspace  PluginSlot = "workspace"
	SlotSpec       PluginSlot = "spec"
	SlotReviewGate PluginSlot = "review_gate"
	SlotTracker    PluginSlot = "tracker"
	SlotSCM        PluginSlot = "scm"
	SlotNotifier   PluginSlot = "notifier"
	SlotStore      PluginSlot = "store"
	SlotTerminal   PluginSlot = "terminal"
)

// Plugin is the common interface every pluggable component must satisfy.
type Plugin interface {
	Name() string
	Init(ctx context.Context) error
	Close() error
}

// PluginModule describes a registerable plugin implementation.
type PluginModule struct {
	Name    string
	Slot    PluginSlot
	Factory func(cfg map[string]any) (Plugin, error)
}
