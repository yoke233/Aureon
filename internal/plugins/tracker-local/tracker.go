package trackerlocal

import (
	"context"

	"github.com/user/ai-workflow/internal/core"
)

// LocalTracker is a no-op tracker implementation that keeps all operations local.
type LocalTracker struct{}

func New() *LocalTracker {
	return &LocalTracker{}
}

func (t *LocalTracker) Name() string {
	return "local"
}

func (t *LocalTracker) Init(context.Context) error {
	return nil
}

func (t *LocalTracker) Close() error {
	return nil
}

func (t *LocalTracker) CreateTask(_ context.Context, item *core.TaskItem) (string, error) {
	if item == nil {
		return "", nil
	}
	if item.ExternalID != "" {
		return item.ExternalID, nil
	}
	return item.ID, nil
}

func (t *LocalTracker) UpdateStatus(context.Context, string, core.TaskItemStatus) error {
	return nil
}

func (t *LocalTracker) SyncDependencies(context.Context, *core.TaskItem, []core.TaskItem) error {
	return nil
}

func (t *LocalTracker) OnExternalComplete(context.Context, string) error {
	return nil
}

var _ core.Tracker = (*LocalTracker)(nil)

