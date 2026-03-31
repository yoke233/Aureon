package workitemapp

import (
	"context"
	"errors"

	"github.com/yoke233/zhanggui/internal/core"
)

type LegacyBackfillStore interface {
	core.WorkItemStore
	core.DeliverableStore
	core.ActionStore
	core.RunStore
	core.JournalStore
}

func BackfillLegacyWorkItems(ctx context.Context, store LegacyBackfillStore, registry core.AgentRegistry) error {
	items, err := store.ListWorkItems(ctx, core.WorkItemFilter{Limit: 1000})
	if err != nil {
		return err
	}
	for _, item := range items {
		if item == nil {
			continue
		}
		changed, manual, err := backfillLegacyWorkItem(ctx, store, registry, item)
		if err != nil {
			return err
		}
		if !changed && !manual {
			continue
		}
		if err := store.UpdateWorkItem(ctx, item); err != nil {
			return err
		}
		summary := "backfilled work item responsibility fields"
		if manual {
			summary = "marked work item for manual migration"
		}
		if _, err := store.AppendJournal(ctx, &core.JournalEntry{
			WorkItemID: item.ID,
			Kind:       core.JournalBackfill,
			Source:     core.JournalSourceSystem,
			Summary:    summary,
		}); err != nil {
			return err
		}
	}
	return nil
}

func backfillLegacyWorkItem(ctx context.Context, store LegacyBackfillStore, registry core.AgentRegistry, item *core.WorkItem) (changed bool, manual bool, err error) {
	legacyAssigned := legacyAssignedProfile(item.Metadata)
	if item.ExecutorProfileID == "" && legacyAssigned != "" {
		item.ExecutorProfileID = legacyAssigned
		changed = true
	}
	if item.ActiveProfileID == "" && item.ExecutorProfileID != "" {
		item.ActiveProfileID = item.ExecutorProfileID
		changed = true
	}
	if item.ReviewerProfileID == "" && item.ExecutorProfileID != "" && registry != nil {
		reviewer, reviewerErr := DefaultReviewerProfileID(ctx, item.ExecutorProfileID, registry)
		if reviewerErr != nil {
			return false, false, reviewerErr
		}
		if reviewer != "" {
			item.ReviewerProfileID = reviewer
			changed = true
		}
	}
	if item.ActiveProfileID != "" && len(item.EscalationPath) == 0 && registry != nil {
		path, pathErr := BuildEscalationPath(ctx, item.ActiveProfileID, registry)
		if pathErr != nil {
			return false, false, pathErr
		}
		item.EscalationPath = path
		changed = true
	}
	if item.SponsorProfileID == "" {
		sponsor := resolveSponsorProfileID(item.EscalationPath, item.ReviewerProfileID)
		if sponsor != "" {
			item.SponsorProfileID = sponsor
			changed = true
		}
	}
	if item.CreatedByProfileID == "" && legacyAssigned != "" {
		item.CreatedByProfileID = item.SponsorProfileID
		if item.CreatedByProfileID == "" {
			item.CreatedByProfileID = item.ReviewerProfileID
		}
		changed = changed || item.CreatedByProfileID != ""
	}
	if item.RootWorkItemID == nil {
		rootID := item.ID
		if item.ParentWorkItemID != nil {
			parent, getErr := store.GetWorkItem(ctx, *item.ParentWorkItemID)
			if getErr != nil && !errors.Is(getErr, core.ErrNotFound) {
				return false, false, getErr
			}
			if getErr == nil {
				if parent.RootWorkItemID != nil {
					rootID = *parent.RootWorkItemID
				} else {
					rootID = parent.ID
				}
			}
		}
		item.RootWorkItemID = &rootID
		changed = true
	}
	if item.FinalDeliverableID == nil {
		deliverables, listErr := store.ListDeliverablesByWorkItem(ctx, item.ID)
		if listErr != nil {
			return false, false, listErr
		}
		if len(deliverables) > 0 {
			finalID := deliverables[0].ID
			item.FinalDeliverableID = &finalID
			changed = true
		} else {
			finalID, created, createErr := backfillDeliverableFromRuns(ctx, store, item.ID)
			if createErr != nil {
				return false, false, createErr
			}
			if created {
				item.FinalDeliverableID = &finalID
				changed = true
			}
		}
	}

	if item.ExecutorProfileID == "" && item.ActiveProfileID == "" {
		item.Metadata = cloneMetadata(item.Metadata)
		item.Metadata["manual_migration_required"] = true
		return changed, true, nil
	}
	return changed, false, nil
}

func backfillDeliverableFromRuns(ctx context.Context, store LegacyBackfillStore, workItemID int64) (int64, bool, error) {
	actions, err := store.ListActionsByWorkItem(ctx, workItemID)
	if err != nil {
		return 0, false, err
	}
	var latest *core.Run
	for _, action := range actions {
		run, runErr := store.GetLatestRunWithResult(ctx, action.ID)
		if runErr != nil {
			if errors.Is(runErr, core.ErrNotFound) {
				continue
			}
			return 0, false, runErr
		}
		if latest == nil || run.CreatedAt.After(latest.CreatedAt) {
			latest = run
		}
	}
	if latest == nil {
		return 0, false, nil
	}
	deliverable := core.RunResultToDeliverable(latest)
	if deliverable == nil {
		return 0, false, nil
	}
	deliverable.WorkItemID = &workItemID
	id, err := store.CreateDeliverable(ctx, deliverable)
	if err != nil {
		return 0, false, err
	}
	return id, true, nil
}

func legacyAssignedProfile(metadata map[string]any) string {
	ceo, ok := metadata["ceo"].(map[string]any)
	if !ok {
		return ""
	}
	value, _ := ceo["assigned_profile"].(string)
	return value
}
