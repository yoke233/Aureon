package web

import (
	"testing"

	"github.com/user/ai-workflow/internal/core"
	storesqlite "github.com/user/ai-workflow/internal/plugins/store-sqlite"
)

func newTestStore(t *testing.T) core.Store {
	t.Helper()

	store, err := storesqlite.New(":memory:")
	if err != nil {
		t.Fatalf("create sqlite store: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})
	return store
}
