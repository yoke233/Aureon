package llmconfig

import (
	"context"
	"testing"

	"github.com/yoke233/ai-workflow/internal/platform/config"
)

func TestBuildReportHidesConnectionFields(t *testing.T) {
	t.Parallel()

	report := buildReport(config.RuntimeLLMConfig{
		DefaultConfigID: "openai-prod",
		Configs: []config.RuntimeLLMEntryConfig{{
			ID:      "openai-prod",
			Type:    ProviderOpenAIResponse,
			BaseURL: "https://api.example.com/v1",
			APIKey:  "sk-secret",
			Model:   "gpt-4.1-mini",
		}},
	})

	if len(report.Configs) != 1 {
		t.Fatalf("configs len = %d, want 1", len(report.Configs))
	}
	if report.Configs[0].BaseURL != "" || report.Configs[0].APIKey != "" {
		t.Fatalf("connection fields should be hidden, got %+v", report.Configs[0])
	}
	if report.Configs[0].Model != "gpt-4.1-mini" {
		t.Fatalf("model = %q, want gpt-4.1-mini", report.Configs[0].Model)
	}
}

func TestMergeEntriesPreservesExistingConnectionFields(t *testing.T) {
	t.Parallel()

	got := mergeEntries(
		[]config.RuntimeLLMEntryConfig{{
			ID:      "openai-prod",
			Type:    ProviderOpenAIResponse,
			BaseURL: "https://api.example.com/v1",
			APIKey:  "sk-secret",
			Model:   "gpt-4.1-mini",
		}},
		[]config.RuntimeLLMEntryConfig{{
			ID:    "openai-prod",
			Type:  ProviderOpenAIResponse,
			Model: "gpt-4.1",
		}},
	)

	if len(got) != 1 {
		t.Fatalf("configs len = %d, want 1", len(got))
	}
	if got[0].BaseURL != "https://api.example.com/v1" {
		t.Fatalf("BaseURL = %q, want preserved value", got[0].BaseURL)
	}
	if got[0].APIKey != "sk-secret" {
		t.Fatalf("APIKey = %q, want preserved value", got[0].APIKey)
	}
	if got[0].Model != "gpt-4.1" {
		t.Fatalf("Model = %q, want updated value", got[0].Model)
	}
}

func TestNormalizeConfigFillsProviderDefaultBaseURL(t *testing.T) {
	t.Parallel()

	cfg := normalizeConfig(config.RuntimeLLMConfig{
		Configs: []config.RuntimeLLMEntryConfig{{
			ID:    "anthropic-default",
			Type:  ProviderAnthropic,
			Model: "claude-sonnet",
		}},
	})

	if len(cfg.Configs) != 1 {
		t.Fatalf("configs len = %d, want 1", len(cfg.Configs))
	}
	if cfg.Configs[0].BaseURL != "https://api.anthropic.com" {
		t.Fatalf("BaseURL = %q, want anthropic default", cfg.Configs[0].BaseURL)
	}
}

func TestReadOnlyControlServiceUpdateReturnsSanitizedReport(t *testing.T) {
	t.Parallel()

	service := NewReadOnlyControlService(config.RuntimeLLMConfig{
		DefaultConfigID: "openai-prod",
		Configs: []config.RuntimeLLMEntryConfig{{
			ID:      "openai-prod",
			Type:    ProviderOpenAIChatCompletion,
			BaseURL: "https://api.openai.com/v1",
			APIKey:  "sk-secret",
			Model:   "gpt-4.1",
		}},
	})

	report, err := service.Update(context.Background(), UpdateRequest{})
	if err != ErrLLMConfigUnavailable {
		t.Fatalf("Update() error = %v, want %v", err, ErrLLMConfigUnavailable)
	}
	if len(report.Configs) != 1 || report.Configs[0].APIKey != "" || report.Configs[0].BaseURL != "" {
		t.Fatalf("Update() report should be sanitized, got %+v", report.Configs)
	}
}
