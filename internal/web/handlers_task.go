package web

import (
	"github.com/go-chi/chi/v5"
	"github.com/yoke233/ai-workflow/internal/core"
)

// Task-level action endpoint has been removed in favor of issue-level actions.
func registerTaskRoutes(_ chi.Router, _ core.Store) {}
