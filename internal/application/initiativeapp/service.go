package initiativeapp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yoke233/zhanggui/internal/core"
)

type Store interface {
	core.InitiativeStore
	core.WorkItemStore
	core.ThreadStore
}

type Tx interface {
	InTx(ctx context.Context, fn func(ctx context.Context, store Store) error) error
}

type Scheduler interface {
	Submit(ctx context.Context, workItemID int64) error
}

type Config struct {
	Store     Store
	Tx        Tx
	Scheduler Scheduler
}

type Service struct {
	store     Store
	tx        Tx
	scheduler Scheduler
}

func New(cfg Config) *Service {
	return &Service{store: cfg.Store, tx: cfg.Tx, scheduler: cfg.Scheduler}
}

type CreateInitiativeInput struct {
	Title       string
	Description string
	CreatedBy   string
	Metadata    map[string]any
}

type UpdateInitiativeInput struct {
	ID          int64
	Title       *string
	Description *string
	Metadata    map[string]any
}

type AddInitiativeItemInput struct {
	InitiativeID int64
	WorkItemID   int64
	Role         string
}

type UpdateInitiativeItemInput struct {
	InitiativeID int64
	WorkItemID   int64
	Role         string
}

type LinkThreadInput struct {
	InitiativeID int64
	ThreadID     int64
	RelationType string
}

type InitiativeDetail struct {
	Initiative *core.Initiative             `json:"initiative"`
	Items      []*core.InitiativeItem       `json:"items"`
	WorkItems  []*core.WorkItem             `json:"work_items"`
	Threads    []*core.ThreadInitiativeLink `json:"threads"`
	Progress   core.InitiativeProgress      `json:"progress"`
}

func (s *Service) CreateInitiative(ctx context.Context, input CreateInitiativeInput) (*core.Initiative, error) {
	title := strings.TrimSpace(input.Title)
	if title == "" {
		return nil, fmt.Errorf("title is required")
	}
	initiative := &core.Initiative{
		Title:       title,
		Description: strings.TrimSpace(input.Description),
		Status:      core.InitiativeDraft,
		CreatedBy:   strings.TrimSpace(input.CreatedBy),
		Metadata:    cloneMetadata(input.Metadata),
	}
	id, err := s.store.CreateInitiative(ctx, initiative)
	if err != nil {
		return nil, err
	}
	initiative.ID = id
	return initiative, nil
}

func (s *Service) ListInitiatives(ctx context.Context, filter core.InitiativeFilter) ([]*core.Initiative, error) {
	return s.store.ListInitiatives(ctx, filter)
}

func (s *Service) GetInitiativeDetail(ctx context.Context, initiativeID int64) (*InitiativeDetail, error) {
	initiative, err := s.refreshDerivedStatus(ctx, initiativeID)
	if err != nil {
		return nil, err
	}
	items, err := s.store.ListInitiativeItems(ctx, initiativeID)
	if err != nil {
		return nil, err
	}
	workItems := make([]*core.WorkItem, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		workItem, err := s.store.GetWorkItem(ctx, item.WorkItemID)
		if err != nil {
			if errors.Is(err, core.ErrNotFound) {
				continue
			}
			return nil, err
		}
		workItems = append(workItems, workItem)
	}
	threads, err := s.store.ListThreadsByInitiative(ctx, initiativeID)
	if err != nil {
		return nil, err
	}
	return &InitiativeDetail{
		Initiative: initiative,
		Items:      items,
		WorkItems:  workItems,
		Threads:    threads,
		Progress:   computeInitiativeProgress(workItems),
	}, nil
}

func (s *Service) UpdateInitiative(ctx context.Context, input UpdateInitiativeInput) (*core.Initiative, error) {
	initiative, err := s.store.GetInitiative(ctx, input.ID)
	if err != nil {
		return nil, err
	}
	if initiative.Status != core.InitiativeDraft {
		return nil, fmt.Errorf("initiative %d is not editable in status %s", initiative.ID, initiative.Status)
	}
	if input.Title != nil {
		title := strings.TrimSpace(*input.Title)
		if title == "" {
			return nil, fmt.Errorf("title is required")
		}
		initiative.Title = title
	}
	if input.Description != nil {
		initiative.Description = strings.TrimSpace(*input.Description)
	}
	if input.Metadata != nil {
		initiative.Metadata = cloneMetadata(input.Metadata)
	}
	if err := s.store.UpdateInitiative(ctx, initiative); err != nil {
		return nil, err
	}
	return initiative, nil
}

func (s *Service) DeleteInitiative(ctx context.Context, initiativeID int64) error {
	initiative, err := s.store.GetInitiative(ctx, initiativeID)
	if err != nil {
		return err
	}
	if initiative.Status != core.InitiativeDraft {
		return fmt.Errorf("initiative %d can only be deleted in draft status", initiativeID)
	}
	run := func(ctx context.Context, store Store) error {
		if err := store.DeleteThreadInitiativeLinksByInitiative(ctx, initiativeID); err != nil {
			return err
		}
		if err := store.DeleteInitiativeItemsByInitiative(ctx, initiativeID); err != nil {
			return err
		}
		return store.DeleteInitiative(ctx, initiativeID)
	}
	if s.tx != nil {
		return s.tx.InTx(ctx, run)
	}
	return run(ctx, s.store)
}

func (s *Service) AddWorkItem(ctx context.Context, input AddInitiativeItemInput) (*core.InitiativeItem, error) {
	initiative, err := s.store.GetInitiative(ctx, input.InitiativeID)
	if err != nil {
		return nil, err
	}
	if initiative.Status != core.InitiativeDraft {
		return nil, fmt.Errorf("initiative %d is not editable in status %s", initiative.ID, initiative.Status)
	}
	if _, err := s.store.GetWorkItem(ctx, input.WorkItemID); err != nil {
		return nil, err
	}
	item := &core.InitiativeItem{
		InitiativeID: input.InitiativeID,
		WorkItemID:   input.WorkItemID,
		Role:         strings.TrimSpace(input.Role),
	}
	id, err := s.store.CreateInitiativeItem(ctx, item)
	if err != nil {
		return nil, err
	}
	item.ID = id
	return item, nil
}

func (s *Service) UpdateWorkItemRole(ctx context.Context, input UpdateInitiativeItemInput) (*core.InitiativeItem, error) {
	initiative, err := s.store.GetInitiative(ctx, input.InitiativeID)
	if err != nil {
		return nil, err
	}
	if initiative.Status != core.InitiativeDraft {
		return nil, fmt.Errorf("initiative %d is not editable in status %s", initiative.ID, initiative.Status)
	}
	item := &core.InitiativeItem{
		InitiativeID: input.InitiativeID,
		WorkItemID:   input.WorkItemID,
		Role:         strings.TrimSpace(input.Role),
	}
	if err := s.store.UpdateInitiativeItem(ctx, item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) RemoveWorkItem(ctx context.Context, initiativeID int64, workItemID int64) error {
	initiative, err := s.store.GetInitiative(ctx, initiativeID)
	if err != nil {
		return err
	}
	if initiative.Status != core.InitiativeDraft {
		return fmt.Errorf("initiative %d is not editable in status %s", initiative.ID, initiative.Status)
	}
	return s.store.DeleteInitiativeItem(ctx, initiativeID, workItemID)
}

func (s *Service) Propose(ctx context.Context, initiativeID int64) (*core.Initiative, error) {
	initiative, err := s.store.GetInitiative(ctx, initiativeID)
	if err != nil {
		return nil, err
	}
	if !core.CanTransitionInitiativeStatus(initiative.Status, core.InitiativeProposed) {
		return nil, fmt.Errorf("initiative %d cannot transition from %s to proposed", initiative.ID, initiative.Status)
	}
	initiative.Status = core.InitiativeProposed
	if err := s.store.UpdateInitiative(ctx, initiative); err != nil {
		return nil, err
	}
	return initiative, nil
}

func (s *Service) Reject(ctx context.Context, initiativeID int64, reviewNote string) (*core.Initiative, error) {
	initiative, err := s.store.GetInitiative(ctx, initiativeID)
	if err != nil {
		return nil, err
	}
	if !core.CanTransitionInitiativeStatus(initiative.Status, core.InitiativeDraft) {
		return nil, fmt.Errorf("initiative %d cannot transition from %s to draft", initiative.ID, initiative.Status)
	}
	initiative.Status = core.InitiativeDraft
	initiative.ReviewNote = strings.TrimSpace(reviewNote)
	if err := s.store.UpdateInitiative(ctx, initiative); err != nil {
		return nil, err
	}
	return initiative, nil
}

func (s *Service) Approve(ctx context.Context, initiativeID int64, approvedBy string) (*core.Initiative, error) {
	initiative, err := s.store.GetInitiative(ctx, initiativeID)
	if err != nil {
		return nil, err
	}
	if !core.CanTransitionInitiativeStatus(initiative.Status, core.InitiativeApproved) {
		return nil, fmt.Errorf("initiative %d cannot transition from %s to approved", initiative.ID, initiative.Status)
	}
	items, err := s.store.ListInitiativeItems(ctx, initiativeID)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	initiative.ApprovedBy = ptr(strings.TrimSpace(approvedBy))
	initiative.ApprovedAt = &now
	initiative.Status = core.InitiativeExecuting
	queueAfterCommit := make([]int64, 0, len(items))
	run := func(ctx context.Context, store Store) error {
		if err := store.UpdateInitiative(ctx, initiative); err != nil {
			return err
		}
		for _, item := range items {
			if item == nil {
				continue
			}
			workItem, err := store.GetWorkItem(ctx, item.WorkItemID)
			if err != nil {
				return err
			}
			queueRoot := len(workItem.DependsOn) == 0 && (workItem.Status == core.WorkItemOpen || workItem.Status == core.WorkItemAccepted)
			holdDependent := len(workItem.DependsOn) > 0 && workItem.Status == core.WorkItemOpen
			if s.scheduler != nil {
				if holdDependent {
					if err := store.UpdateWorkItemStatus(ctx, item.WorkItemID, core.WorkItemAccepted); err != nil {
						return err
					}
				}
				if queueRoot {
					queueAfterCommit = append(queueAfterCommit, item.WorkItemID)
				}
				continue
			}
			if queueRoot {
				if err := store.PrepareWorkItemRun(ctx, item.WorkItemID, core.WorkItemQueued); err != nil {
					return err
				}
				continue
			}
			if !holdDependent {
				continue
			}
			if err := store.UpdateWorkItemStatus(ctx, item.WorkItemID, core.WorkItemAccepted); err != nil {
				return err
			}
		}
		return nil
	}
	if s.tx != nil {
		if err := s.tx.InTx(ctx, run); err != nil {
			return nil, err
		}
	} else {
		if err := run(ctx, s.store); err != nil {
			return nil, err
		}
	}
	for _, workItemID := range queueAfterCommit {
		if err := s.scheduler.Submit(ctx, workItemID); err != nil {
			return nil, err
		}
	}
	return initiative, nil
}

func (s *Service) Cancel(ctx context.Context, initiativeID int64) (*core.Initiative, error) {
	initiative, err := s.store.GetInitiative(ctx, initiativeID)
	if err != nil {
		return nil, err
	}
	if !core.CanTransitionInitiativeStatus(initiative.Status, core.InitiativeCancelled) {
		return nil, fmt.Errorf("initiative %d cannot transition from %s to cancelled", initiative.ID, initiative.Status)
	}
	initiative.Status = core.InitiativeCancelled
	if err := s.store.UpdateInitiative(ctx, initiative); err != nil {
		return nil, err
	}
	return initiative, nil
}

func (s *Service) GetProgress(ctx context.Context, initiativeID int64) (core.InitiativeProgress, error) {
	detail, err := s.GetInitiativeDetail(ctx, initiativeID)
	if err != nil {
		return core.InitiativeProgress{}, err
	}
	return detail.Progress, nil
}

func (s *Service) LinkThread(ctx context.Context, input LinkThreadInput) (*core.ThreadInitiativeLink, error) {
	if _, err := s.store.GetInitiative(ctx, input.InitiativeID); err != nil {
		return nil, err
	}
	if _, err := s.store.GetThread(ctx, input.ThreadID); err != nil {
		return nil, err
	}
	link := &core.ThreadInitiativeLink{
		InitiativeID: input.InitiativeID,
		ThreadID:     input.ThreadID,
		RelationType: strings.TrimSpace(input.RelationType),
	}
	id, err := s.store.CreateThreadInitiativeLink(ctx, link)
	if err != nil {
		return nil, err
	}
	link.ID = id
	return link, nil
}

func (s *Service) ListThreads(ctx context.Context, initiativeID int64) ([]*core.ThreadInitiativeLink, error) {
	return s.store.ListThreadsByInitiative(ctx, initiativeID)
}

func (s *Service) UnlinkThread(ctx context.Context, initiativeID int64, threadID int64) error {
	return s.store.DeleteThreadInitiativeLink(ctx, initiativeID, threadID)
}

func (s *Service) refreshDerivedStatus(ctx context.Context, initiativeID int64) (*core.Initiative, error) {
	initiative, err := s.store.GetInitiative(ctx, initiativeID)
	if err != nil {
		return nil, err
	}
	items, err := s.store.ListInitiativeItems(ctx, initiativeID)
	if err != nil {
		return nil, err
	}
	workItems := make([]*core.WorkItem, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		workItem, err := s.store.GetWorkItem(ctx, item.WorkItemID)
		if err != nil {
			if errors.Is(err, core.ErrNotFound) {
				continue
			}
			return nil, err
		}
		workItems = append(workItems, workItem)
	}
	progress := computeInitiativeProgress(workItems)
	nextStatus := initiative.Status
	switch {
	case initiative.Status == core.InitiativeExecuting && progress.Total > 0 && progress.Done == progress.Total:
		nextStatus = core.InitiativeDone
	case initiative.Status == core.InitiativeExecuting && progress.Failed > 0:
		nextStatus = core.InitiativeFailed
	case initiative.Status == core.InitiativeExecuting && progress.Blocked > 0 && progress.Running == 0:
		nextStatus = core.InitiativeBlocked
	case initiative.Status == core.InitiativeBlocked && progress.Total > 0 && progress.Done == progress.Total:
		nextStatus = core.InitiativeDone
	case initiative.Status == core.InitiativeBlocked && progress.Running > 0:
		nextStatus = core.InitiativeExecuting
	}
	if nextStatus != initiative.Status {
		initiative.Status = nextStatus
		if err := s.store.UpdateInitiative(ctx, initiative); err != nil {
			return nil, err
		}
	}
	return initiative, nil
}

func computeInitiativeProgress(workItems []*core.WorkItem) core.InitiativeProgress {
	var progress core.InitiativeProgress
	progress.Total = len(workItems)
	for _, workItem := range workItems {
		if workItem == nil {
			continue
		}
		switch workItem.Status {
		case core.WorkItemOpen, core.WorkItemAccepted:
			progress.Pending++
		case core.WorkItemQueued, core.WorkItemRunning:
			progress.Running++
		case core.WorkItemBlocked:
			progress.Blocked++
		case core.WorkItemCompleted:
			progress.Done++
		case core.WorkItemFailed:
			progress.Failed++
		case core.WorkItemCancelled:
			progress.Cancelled++
		default:
			progress.Pending++
		}
	}
	return progress
}

func cloneMetadata(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func ptr(v string) *string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	value := strings.TrimSpace(v)
	return &value
}
