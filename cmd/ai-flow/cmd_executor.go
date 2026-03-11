package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/nats-io/nats.go"
	"github.com/yoke233/ai-workflow/internal/config"
	"github.com/yoke233/ai-workflow/internal/teamleader"
	v2engine "github.com/yoke233/ai-workflow/internal/v2/engine"
	v2sqlite "github.com/yoke233/ai-workflow/internal/v2/store/sqlite"
)

// cmdExecutor runs a remote executor worker that connects to NATS and processes
// ACP prompt messages. This is the `ai-flow executor` subcommand.
//
// Usage:
//
//	ai-flow executor --nats-url nats://localhost:4222 [--agents claude,codex] [--max-concurrent 2]
func cmdExecutor(args []string) error {
	natsURL := ""
	agentTypes := ""
	maxConcurrent := 2

	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--nats-url" && i+1 < len(args):
			i++
			natsURL = strings.TrimSpace(args[i])
		case strings.HasPrefix(args[i], "--nats-url="):
			natsURL = strings.TrimSpace(strings.TrimPrefix(args[i], "--nats-url="))
		case args[i] == "--agents" && i+1 < len(args):
			i++
			agentTypes = strings.TrimSpace(args[i])
		case strings.HasPrefix(args[i], "--agents="):
			agentTypes = strings.TrimSpace(strings.TrimPrefix(args[i], "--agents="))
		case args[i] == "--max-concurrent" && i+1 < len(args):
			i++
			n := 0
			if _, err := fmt.Sscanf(args[i], "%d", &n); err == nil && n > 0 {
				maxConcurrent = n
			}
		default:
			return fmt.Errorf("unknown flag: %s\nusage: ai-flow executor --nats-url <url> [--agents claude,codex] [--max-concurrent 2]", args[i])
		}
	}

	if natsURL == "" {
		natsURL = os.Getenv("AI_WORKFLOW_NATS_URL")
	}
	if natsURL == "" {
		return fmt.Errorf("--nats-url is required (or set AI_WORKFLOW_NATS_URL)")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Load config for agent registry.
	cfg, err := loadBootstrapConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Open the v2 store for agent profile/driver resolution.
	dbPath := expandStorePath(cfg.Store.Path)
	v2DBPath := strings.TrimSuffix(dbPath, ".db") + "_v2.db"
	store, err := v2sqlite.New(v2DBPath)
	if err != nil {
		return fmt.Errorf("open v2 store: %w", err)
	}
	defer store.Close()

	// Seed registry.
	seedV2Registry(context.Background(), store, cfg, nil)

	// Connect to NATS.
	nc, err := nats.Connect(natsURL,
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2),
	)
	if err != nil {
		return fmt.Errorf("connect to NATS at %s: %w", natsURL, err)
	}
	defer nc.Drain()

	slog.Info("executor: connected to NATS", "url", natsURL)

	var agents []string
	if agentTypes != "" {
		for _, a := range strings.Split(agentTypes, ",") {
			if t := strings.TrimSpace(a); t != "" {
				agents = append(agents, t)
			}
		}
	}

	streamPrefix := "aiworkflow"
	if cfg.V2.SessionManager.NATS.StreamPrefix != "" {
		streamPrefix = cfg.V2.SessionManager.NATS.StreamPrefix
	}

	worker, err := v2engine.NewExecutorWorker(v2engine.ExecutorWorkerConfig{
		NATSConn:       nc,
		StreamPrefix:   streamPrefix,
		AgentTypes:     agents,
		Store:          store,
		Registry:       store,
		DefaultWorkDir: resolveDefaultWorkDir(cfg),
		MaxConcurrent:  maxConcurrent,
		MCPEnv:         buildExecutorMCPEnv(cfg),
	})
	if err != nil {
		return fmt.Errorf("create executor worker: %w", err)
	}

	slog.Info("executor: starting worker", "agents", agents, "max_concurrent", maxConcurrent)

	err = worker.Start(ctx)
	worker.Stop()

	if ctx.Err() != nil {
		slog.Info("executor: shutting down")
		return nil
	}
	return err
}

func resolveDefaultWorkDir(cfg *config.Config) string {
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return cwd
}

func buildExecutorMCPEnv(cfg *config.Config) teamleader.MCPEnvConfig {
	return teamleader.MCPEnvConfig{
		DBPath: expandStorePath(cfg.Store.Path),
	}
}
