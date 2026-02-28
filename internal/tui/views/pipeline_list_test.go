package views

import (
	"strings"
	"testing"

	"github.com/user/ai-workflow/internal/core"
)

func TestRenderPipelineListShowsCurrentStage(t *testing.T) {
	out := RenderPipelineList([]core.Pipeline{
		{
			ID:           "p-1",
			Name:         "demo-pipe",
			Status:       core.StatusRunning,
			CurrentStage: core.StageImplement,
		},
	}, 0, map[string]func(string) string{
		"running": func(s string) string { return s },
	})

	if !strings.Contains(out, "implement") {
		t.Fatalf("expected current stage in list output, got: %s", out)
	}
}
