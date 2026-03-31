package workitemapp

import (
	"context"
	"errors"

	"github.com/yoke233/zhanggui/internal/core"
)

func (s *Service) AdoptDeliverable(ctx context.Context, workItemID, deliverableID int64) (*core.WorkItem, error) {
	workItem, err := s.store.GetWorkItem(ctx, workItemID)
	if err != nil {
		if errors.Is(err, core.ErrNotFound) {
			return nil, newError(CodeWorkItemNotFound, "work item not found", err)
		}
		return nil, err
	}

	deliverable, err := s.store.GetDeliverable(ctx, deliverableID)
	if err != nil {
		if errors.Is(err, core.ErrNotFound) {
			return nil, newError(CodeDeliverableNotFound, "deliverable not found", err)
		}
		return nil, err
	}

	workItem.FinalDeliverableID = &deliverable.ID
	if err := s.store.UpdateWorkItem(ctx, workItem); err != nil {
		if errors.Is(err, core.ErrNotFound) {
			return nil, newError(CodeWorkItemNotFound, "work item not found", err)
		}
		return nil, err
	}
	return workItem, nil
}

func (s *Service) ListDeliverables(ctx context.Context, workItemID int64) ([]*core.Deliverable, error) {
	workItem, err := s.store.GetWorkItem(ctx, workItemID)
	if err != nil {
		if errors.Is(err, core.ErrNotFound) {
			return nil, newError(CodeWorkItemNotFound, "work item not found", err)
		}
		return nil, err
	}

	items, err := s.store.ListDeliverablesByWorkItem(ctx, workItemID)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 && workItem.FinalDeliverableID == nil {
		return []*core.Deliverable{}, nil
	}

	result := make([]*core.Deliverable, 0, len(items)+1)
	seen := make(map[int64]struct{}, len(items)+1)
	appendUnique := func(item *core.Deliverable) {
		if item == nil {
			return
		}
		if _, exists := seen[item.ID]; exists {
			return
		}
		seen[item.ID] = struct{}{}
		result = append(result, item)
	}

	if workItem.FinalDeliverableID != nil {
		finalDeliverable, err := s.store.GetDeliverable(ctx, *workItem.FinalDeliverableID)
		if err != nil {
			if errors.Is(err, core.ErrNotFound) {
				return nil, newError(CodeDeliverableNotFound, "deliverable not found", err)
			}
			return nil, err
		}
		appendUnique(finalDeliverable)
	}
	for _, item := range items {
		appendUnique(item)
	}
	return result, nil
}
