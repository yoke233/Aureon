package specnoop

import (
	"context"
	"testing"

	"github.com/user/ai-workflow/internal/core"
)

func TestSpecPluginInterface_CompileGuard(t *testing.T) {
	var _ core.SpecPlugin = (*NoopSpec)(nil)
}

func TestNoopSpec_IsInitializedFalse(t *testing.T) {
	plugin := New()
	if err := plugin.Init(context.Background()); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	if plugin.IsInitialized() {
		t.Fatal("noop spec should report IsInitialized() == false")
	}
}

func TestNoopSpec_GetContext_ReturnsEmptyContext(t *testing.T) {
	plugin := New()
	got, err := plugin.GetContext(context.Background(), core.SpecContextRequest{
		ProjectID: "proj-1",
		PlanID:    "plan-1",
		Query:     "oauth",
	})
	if err != nil {
		t.Fatalf("GetContext() error = %v", err)
	}
	if got.Summary != "" {
		t.Fatalf("GetContext() summary = %q, want empty", got.Summary)
	}
	if len(got.References) != 0 {
		t.Fatalf("GetContext() references = %v, want empty", got.References)
	}
}
