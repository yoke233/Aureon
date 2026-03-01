package core

import "context"

// Tracker mirrors TaskItem state into external systems.
type Tracker interface {
	Plugin
	CreateTask(ctx context.Context, item *TaskItem) (externalID string, err error)
	UpdateStatus(ctx context.Context, externalID string, status TaskItemStatus) error
	SyncDependencies(ctx context.Context, item *TaskItem, allItems []TaskItem) error
	OnExternalComplete(ctx context.Context, externalID string) error
}
