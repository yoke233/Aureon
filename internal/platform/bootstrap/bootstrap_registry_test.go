package bootstrap

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/yoke233/zhanggui/internal/adapters/store/sqlite"
	"github.com/yoke233/zhanggui/internal/core"
	"github.com/yoke233/zhanggui/internal/platform/config"
)

func TestSeedRegistrySeedsOnlyCEOOnEmptyStore(t *testing.T) {
	t.Parallel()

	store := newBootstrapRegistryTestStore(t)
	cfg := bootstrapRegistrySeedConfig()
	seedRegistry(context.Background(), store, cfg, nil)

	profiles, err := store.ListProfiles(context.Background())
	if err != nil {
		t.Fatalf("ListProfiles() error = %v", err)
	}
	if len(profiles) != 1 {
		t.Fatalf("profiles len = %d, want 1", len(profiles))
	}
	if profiles[0].ID != "ceo" {
		t.Fatalf("profiles[0].ID = %q, want ceo", profiles[0].ID)
	}
	if profiles[0].ManagerProfileID != "" {
		t.Fatalf("profiles[0].ManagerProfileID = %q, want empty", profiles[0].ManagerProfileID)
	}
}

func TestSeedRegistryDoesNotOverwriteExistingProfiles(t *testing.T) {
	t.Parallel()

	store := newBootstrapRegistryTestStore(t)
	if err := store.UpsertProfile(context.Background(), &core.AgentProfile{
		ID:          "custom",
		Name:        "Custom",
		DriverID:    "claude-acp",
		LLMConfigID: "system",
		Role:        core.RoleLead,
		Driver: core.DriverConfig{
			CapabilitiesMax: core.DriverCapabilities{FSRead: true, FSWrite: true, Terminal: true},
		},
	}); err != nil {
		t.Fatalf("UpsertProfile(custom) error = %v", err)
	}

	cfg := bootstrapRegistrySeedConfig()
	seedRegistry(context.Background(), store, cfg, nil)

	profiles, err := store.ListProfiles(context.Background())
	if err != nil {
		t.Fatalf("ListProfiles() error = %v", err)
	}
	if len(profiles) != 1 {
		t.Fatalf("profiles len = %d, want 1", len(profiles))
	}
	if profiles[0].ID != "custom" {
		t.Fatalf("profiles[0].ID = %q, want custom", profiles[0].ID)
	}
}

func newBootstrapRegistryTestStore(t *testing.T) *sqlite.Store {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "bootstrap-registry-test.db")
	store, err := sqlite.New(dbPath)
	if err != nil {
		t.Fatalf("sqlite.New() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func bootstrapRegistrySeedConfig() *config.Config {
	return &config.Config{
		Runtime: config.RuntimeConfig{
			Agents: config.RuntimeAgentsConfig{
				Drivers: []config.RuntimeDriverConfig{{
					ID:            "codex-cli",
					LaunchCommand: "codex",
					CapabilitiesMax: config.CapabilitiesConfig{
						FSRead:   true,
						FSWrite:  true,
						Terminal: true,
					},
				}},
				Profiles: []config.RuntimeProfileConfig{{
					ID:          "ceo",
					Name:        "CEO Orchestrator",
					Driver:      "codex-cli",
					LLMConfigID: "system",
					Role:        string(core.RoleLead),
					Session: config.RuntimeSessionConfig{
						Reuse:    true,
						MaxTurns: 16,
						IdleTTL:  config.Duration{Duration: 30 * time.Minute},
					},
				}, {
					ID:               "lead",
					Name:             "Lead Agent",
					Driver:           "codex-cli",
					LLMConfigID:      "system",
					Role:             string(core.RoleLead),
					ManagerProfileID: "ceo",
				}},
			},
		},
	}
}
