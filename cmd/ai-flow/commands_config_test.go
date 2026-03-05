package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yoke233/ai-workflow/internal/config"
)

func TestCmdConfigInitCreatesLoadableConfig(t *testing.T) {
	wd := t.TempDir()
	t.Chdir(wd)

	if err := cmdConfigInit(nil); err != nil {
		t.Fatalf("cmdConfigInit() error = %v", err)
	}

	cfgPath := filepath.Join(wd, ".ai-workflow", "config.yaml")
	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read generated config: %v", err)
	}
	if len(raw) == 0 {
		t.Fatal("generated config should not be empty")
	}

	loaded, err := config.LoadGlobal(cfgPath)
	if err != nil {
		t.Fatalf("generated config must be loadable by strict loader: %v", err)
	}
	if loaded.Run.DefaultTemplate == "" {
		t.Fatal("loaded config should preserve run.default_template")
	}
}

func TestCmdConfigInitReturnsErrorWhenConfigExists(t *testing.T) {
	wd := t.TempDir()
	t.Chdir(wd)

	cfgPath := filepath.Join(wd, ".ai-workflow", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatalf("mkdir data dir: %v", err)
	}
	if err := os.WriteFile(cfgPath, []byte("existing: true\n"), 0o644); err != nil {
		t.Fatalf("write existing config: %v", err)
	}

	err := cmdConfigInit(nil)
	if err == nil {
		t.Fatal("expected conflict error when config exists")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected already exists error, got %v", err)
	}
}

func TestCmdConfigInitForceOverwritesConfig(t *testing.T) {
	wd := t.TempDir()
	t.Chdir(wd)

	cfgPath := filepath.Join(wd, ".ai-workflow", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatalf("mkdir data dir: %v", err)
	}
	if err := os.WriteFile(cfgPath, []byte("invalid: [\n"), 0o644); err != nil {
		t.Fatalf("write existing invalid config: %v", err)
	}

	if err := cmdConfigInit([]string{"--force"}); err != nil {
		t.Fatalf("cmdConfigInit(--force) error = %v", err)
	}

	loaded, err := config.LoadGlobal(cfgPath)
	if err != nil {
		t.Fatalf("forced overwritten config should be loadable: %v", err)
	}
	if loaded.Server.Port == 0 {
		t.Fatalf("expected non-zero server port from defaults, got %d", loaded.Server.Port)
	}
}

func TestCmdConfigInitThenLoadBootstrapConfig(t *testing.T) {
	wd := t.TempDir()
	t.Chdir(wd)

	if err := cmdConfigInit(nil); err != nil {
		t.Fatalf("cmdConfigInit() error = %v", err)
	}

	cfg, err := loadBootstrapConfig()
	if err != nil {
		t.Fatalf("loadBootstrapConfig() should load initialized config: %v", err)
	}
	if cfg == nil {
		t.Fatal("loadBootstrapConfig() returned nil config")
	}
}

func TestCLIConfigCommandUsageError(t *testing.T) {
	err := runWithArgs([]string{"config"})
	if err == nil {
		t.Fatal("expected usage error for missing config subcommand")
	}
	if !strings.Contains(err.Error(), "usage: ai-flow config <init> [--force]") {
		t.Fatalf("unexpected usage error: %v", err)
	}
}

func TestCLIConfigInitCommandRoute(t *testing.T) {
	wd := t.TempDir()
	t.Chdir(wd)

	if err := runWithArgs([]string{"config", "init"}); err != nil {
		t.Fatalf("runWithArgs(config init) error = %v", err)
	}

	cfgPath := filepath.Join(wd, ".ai-workflow", "config.yaml")
	if _, err := config.LoadGlobal(cfgPath); err != nil {
		t.Fatalf("generated config from CLI route should be loadable: %v", err)
	}
}
