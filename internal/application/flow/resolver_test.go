package flow

import (
	"context"
	"testing"

	"github.com/yoke233/zhanggui/internal/core"
)

func TestProfileRegistryResolvePrefersConfiguredProfileOverride(t *testing.T) {
	t.Parallel()

	reg := NewProfileRegistry([]*core.AgentProfile{
		{ID: "worker-a", Role: core.RoleWorker, Capabilities: []string{"backend"}},
		{ID: "worker-b", Role: core.RoleWorker, Capabilities: []string{"backend"}},
	})
	action := &core.Action{
		AgentRole:            "worker",
		RequiredCapabilities: []string{"backend"},
		Config:               map[string]any{"preferred_profile_id": "worker-b"},
	}

	got, err := reg.Resolve(context.Background(), action)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got != "worker-b" {
		t.Fatalf("Resolve() = %q, want worker-b", got)
	}
}

func TestProfileRegistryResolveFallsBackWhenPreferredProfileMissing(t *testing.T) {
	t.Parallel()

	reg := NewProfileRegistry([]*core.AgentProfile{
		{ID: "worker-a", Role: core.RoleWorker, Capabilities: []string{"backend"}},
	})
	action := &core.Action{
		AgentRole:            "worker",
		RequiredCapabilities: []string{"backend"},
		Config:               map[string]any{"preferred_profile_id": "missing"},
	}

	got, err := reg.Resolve(context.Background(), action)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got != "worker-a" {
		t.Fatalf("Resolve() = %q, want worker-a", got)
	}
}

func TestProfileRegistryResolveReturnsNoMatchForNilAction(t *testing.T) {
	t.Parallel()

	reg := NewProfileRegistry([]*core.AgentProfile{
		{ID: "worker-a", Role: core.RoleWorker, Capabilities: []string{"backend"}},
	})

	got, err := reg.Resolve(context.Background(), nil)
	if err != core.ErrNoMatchingAgent {
		t.Fatalf("Resolve() error = %v, want %v", err, core.ErrNoMatchingAgent)
	}
	if got != "" {
		t.Fatalf("Resolve() = %q, want empty", got)
	}
}
