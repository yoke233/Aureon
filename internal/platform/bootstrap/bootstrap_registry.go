package bootstrap

import (
	"context"
	"log/slog"

	"github.com/yoke233/zhanggui/internal/adapters/agent/acpclient"
	"github.com/yoke233/zhanggui/internal/adapters/store/sqlite"
	"github.com/yoke233/zhanggui/internal/core"
	"github.com/yoke233/zhanggui/internal/platform/config"
	"github.com/yoke233/zhanggui/internal/platform/configruntime"
)

// seedRegistry seeds agent profiles into the SQLite store from TOML config.
// Uses upsert so TOML always acts as the source of truth for configured agents,
// while runtime additions via API are also persisted.
func seedRegistry(ctx context.Context, store *sqlite.Store, cfg *config.Config, _ *acpclient.RoleResolver) {
	if cfg == nil || store == nil {
		return
	}

	currentProfiles, err := store.ListProfiles(ctx)
	if err != nil {
		slog.Warn("registry: list profiles before bootstrap failed", "error", err)
		return
	}
	if len(currentProfiles) > 0 {
		slog.Info("registry: bootstrap skipped because store already has profiles", "profiles", len(currentProfiles))
		return
	}

	profiles := bootstrapProfiles(configruntime.BuildAgents(cfg))
	if len(profiles) == 0 {
		slog.Warn("registry: no agent config to seed")
		return
	}

	for _, p := range profiles {
		if err := store.UpsertProfile(ctx, p); err != nil {
			slog.Warn("registry: seed profile failed", "id", p.ID, "error", err)
		}
	}
	slog.Info("registry: seeded from config", "profiles", len(profiles))
}

func bootstrapProfiles(profiles []*core.AgentProfile) []*core.AgentProfile {
	if len(profiles) == 0 {
		return nil
	}
	for _, profile := range profiles {
		if profile != nil && profile.ID == "ceo" {
			return []*core.AgentProfile{profile}
		}
	}
	for _, profile := range profiles {
		if profile != nil {
			slog.Warn("registry: ceo profile missing from bootstrap config; seeding first available profile", "id", profile.ID)
			return []*core.AgentProfile{profile}
		}
	}
	return nil
}

func SeedRegistry(ctx context.Context, store *sqlite.Store, cfg *config.Config) {
	seedRegistry(ctx, store, cfg, nil)
}
