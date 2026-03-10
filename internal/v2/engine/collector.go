package engine

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/yoke233/ai-workflow/internal/v2/core"
	"github.com/yoke233/ai-workflow/internal/v2/llm"
)

// LLMCollector extracts structured metadata from agent markdown output
// by calling a small LLM to produce JSON (Structured Outputs / JSON schema).
type LLMCollector struct {
	// Complete is the LLM completion function injected by the caller.
	// prompt is the fully assembled extraction prompt.
	// tools carries the JSON schema used to define the expected JSON output.
	// Returns the raw JSON output.
	Complete func(ctx context.Context, prompt string, tools []llm.ToolDef) (json.RawMessage, error)
}

// NewLLMCollector creates a Collector backed by an LLM completion function.
// Typical usage: NewLLMCollector(llmClient.Complete)
func NewLLMCollector(complete func(ctx context.Context, prompt string, tools []llm.ToolDef) (json.RawMessage, error)) *LLMCollector {
	return &LLMCollector{Complete: complete}
}

// Extract implements the Collector interface.
func (c *LLMCollector) Extract(ctx context.Context, stepType core.StepType, markdown string) (map[string]any, error) {
	if c.Complete == nil {
		return nil, fmt.Errorf("LLMCollector.Complete is not set")
	}

	prompt := buildExtractionPrompt(stepType, markdown)
	tools := extractionTools(stepType)

	raw, err := c.Complete(ctx, prompt, tools)
	if err != nil {
		return nil, fmt.Errorf("llm extract for %s: %w", stepType, err)
	}

	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("parse extraction result: %w", err)
	}
	return result, nil
}

// buildExtractionPrompt creates the system+user prompt for metadata extraction.
func buildExtractionPrompt(stepType core.StepType, markdown string) string {
	var instruction string
	switch stepType {
	case core.StepGate:
		instruction = `You are a metadata extractor. Analyze the following gate review output and extract:
- verdict: "pass" or "reject"
 - reason: a short, human-readable reason (empty string if unclear)
 - reject_targets: list of upstream step IDs to reset when verdict is "reject" (optional; omit if unclear)
Return ONLY a JSON object matching the provided JSON schema.`
	case core.StepComposite:
		instruction = `You are a metadata extractor. Analyze the following composite step output and extract:
- sub_tasks: list of sub-task names/descriptions identified
Return ONLY a JSON object matching the provided JSON schema.`
	default: // exec
		instruction = `You are a metadata extractor. Analyze the following execution output and extract:
- summary: a one-sentence summary of what was accomplished
- files_changed: list of file paths that were modified (empty list if unclear)
- tests_passed: boolean indicating whether tests passed (null if not mentioned)
Return ONLY a JSON object matching the provided JSON schema.`
	}
	return fmt.Sprintf("%s\n\n---\n\n%s", instruction, markdown)
}

// extractionTools returns the tool definitions for a given step type.
func extractionTools(stepType core.StepType) []llm.ToolDef {
	switch stepType {
	case core.StepGate:
		return []llm.ToolDef{{
			Name:        "extract_gate_metadata",
			Description: "Extract structured metadata from a gate review output.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"verdict": map[string]any{
						"type":        "string",
						"enum":        []string{"pass", "reject"},
						"description": "Whether the gate passed or was rejected.",
					},
					"reason": map[string]any{
						"type":        "string",
						"description": "Short, human-readable reason for the verdict.",
					},
					"reject_targets": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "integer"},
						"description": "Upstream step IDs to reset when verdict is reject.",
					},
				},
				"required": []string{"verdict", "reason"},
			},
		}}
	case core.StepComposite:
		return []llm.ToolDef{{
			Name:        "extract_composite_metadata",
			Description: "Extract structured metadata from a composite step output.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"sub_tasks": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "List of sub-task names or descriptions.",
					},
				},
				"required": []string{"sub_tasks"},
			},
		}}
	default: // exec
		return []llm.ToolDef{{
			Name:        "extract_exec_metadata",
			Description: "Extract structured metadata from an execution output.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"summary": map[string]any{
						"type":        "string",
						"description": "One-sentence summary of what was accomplished.",
					},
					"files_changed": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "File paths that were modified.",
					},
					"tests_passed": map[string]any{
						"type":        "boolean",
						"description": "Whether tests passed. Omit if not mentioned.",
					},
				},
				"required": []string{"summary"},
			},
		}}
	}
}

// OpenAICompleter is a backward-compatible wrapper around llm.Client.
// Deprecated: Use llm.New() directly instead.
type OpenAICompleter = llm.Client

// OpenAICompleterConfig is a backward-compatible alias for llm.Config.
// Deprecated: Use llm.Config directly instead.
type OpenAICompleterConfig = llm.Config

// NewOpenAICompleter is a backward-compatible wrapper around llm.New.
// Deprecated: Use llm.New() directly instead.
func NewOpenAICompleter(cfg OpenAICompleterConfig) (*OpenAICompleter, error) {
	return llm.New(cfg)
}

// ToolDef is a backward-compatible alias for llm.ToolDef.
// Deprecated: Use llm.ToolDef directly instead.
type ToolDef = llm.ToolDef
