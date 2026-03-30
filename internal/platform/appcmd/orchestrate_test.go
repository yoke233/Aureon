package appcmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestParseOrchestrateArgsTaskCreate(t *testing.T) {
	t.Parallel()

	opts, err := parseOrchestrateArgs([]string{
		"task", "create",
		"--title", "CEO bootstrap",
		"--project-id", "12",
		"--json",
	})
	if err != nil {
		t.Fatalf("parseOrchestrateArgs() error = %v", err)
	}
	if opts.Action != "task.create" {
		t.Fatalf("Action = %q, want task.create", opts.Action)
	}
	if opts.Title != "CEO bootstrap" {
		t.Fatalf("Title = %q, want CEO bootstrap", opts.Title)
	}
	if opts.ProjectID == nil || *opts.ProjectID != 12 {
		t.Fatalf("ProjectID = %v, want 12", opts.ProjectID)
	}
	if !opts.JSON {
		t.Fatal("expected JSON to be true")
	}
}

func TestParseOrchestrateArgsRejectsUnknownFlag(t *testing.T) {
	t.Parallel()

	_, err := parseOrchestrateArgs([]string{"task", "create", "--nope"})
	if err == nil {
		t.Fatal("expected unknown flag error")
	}
	if !strings.Contains(err.Error(), "unknown flag") {
		t.Fatalf("unexpected error = %v", err)
	}
}

func TestRunOrchestrateToWriterEmitsStableJSON(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	err := runOrchestrateToWriter(&out, []string{
		"task", "create",
		"--title", "CEO bootstrap",
		"--project-id", "12",
		"--json",
	})
	if err != nil {
		t.Fatalf("runOrchestrateToWriter() error = %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if payload["ok"] != true {
		t.Fatalf("ok = %v, want true", payload["ok"])
	}
	if payload["action"] != "task.create" {
		t.Fatalf("action = %v, want task.create", payload["action"])
	}
	if payload["implemented"] != false {
		t.Fatalf("implemented = %v, want false", payload["implemented"])
	}
	if payload["title"] != "CEO bootstrap" {
		t.Fatalf("title = %v, want CEO bootstrap", payload["title"])
	}
}
