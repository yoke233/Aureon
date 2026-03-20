package sqlite

import (
	"context"
	"fmt"
	"time"

	"github.com/yoke233/zhanggui/internal/core"
	"gorm.io/gorm"
)

func (s *Store) CreateInitiative(ctx context.Context, initiative *core.Initiative) (int64, error) {
	if s == nil || s.orm == nil {
		return 0, fmt.Errorf("store is not initialized")
	}
	if initiative == nil {
		return 0, fmt.Errorf("initiative is nil")
	}
	if initiative.Status == "" {
		initiative.Status = core.InitiativeDraft
	}
	now := time.Now().UTC()
	model := initiativeModelFromCore(initiative)
	model.CreatedAt = now
	model.UpdatedAt = now
	if err := s.orm.WithContext(ctx).Create(model).Error; err != nil {
		return 0, err
	}
	initiative.ID = model.ID
	initiative.CreatedAt = now
	initiative.UpdatedAt = now
	return model.ID, nil
}

func (s *Store) GetInitiative(ctx context.Context, id int64) (*core.Initiative, error) {
	var model InitiativeModel
	if err := s.orm.WithContext(ctx).First(&model, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, core.ErrNotFound
		}
		return nil, err
	}
	return model.toCore(), nil
}

func (s *Store) ListInitiatives(ctx context.Context, filter core.InitiativeFilter) ([]*core.Initiative, error) {
	query := s.orm.WithContext(ctx).Model(&InitiativeModel{})
	if filter.Status != nil {
		query = query.Where("status = ?", string(*filter.Status))
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}
	var models []InitiativeModel
	if err := query.Order("id DESC").Limit(limit).Offset(offset).Find(&models).Error; err != nil {
		return nil, err
	}
	out := make([]*core.Initiative, 0, len(models))
	for i := range models {
		out = append(out, models[i].toCore())
	}
	return out, nil
}

func (s *Store) UpdateInitiative(ctx context.Context, initiative *core.Initiative) error {
	if initiative == nil {
		return fmt.Errorf("initiative is nil")
	}
	model := initiativeModelFromCore(initiative)
	model.UpdatedAt = time.Now().UTC()
	result := s.orm.WithContext(ctx).Model(&InitiativeModel{}).
		Where("id = ?", initiative.ID).
		Updates(map[string]any{
			"title":       model.Title,
			"description": model.Description,
			"status":      model.Status,
			"created_by":  model.CreatedBy,
			"approved_by": model.ApprovedBy,
			"approved_at": model.ApprovedAt,
			"review_note": model.ReviewNote,
			"metadata":    model.Metadata,
			"updated_at":  model.UpdatedAt,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return core.ErrNotFound
	}
	initiative.UpdatedAt = model.UpdatedAt
	return nil
}

func (s *Store) DeleteInitiative(ctx context.Context, id int64) error {
	result := s.orm.WithContext(ctx).Delete(&InitiativeModel{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return core.ErrNotFound
	}
	return nil
}

func (s *Store) CreateInitiativeItem(ctx context.Context, item *core.InitiativeItem) (int64, error) {
	if item == nil {
		return 0, fmt.Errorf("initiative item is nil")
	}
	now := time.Now().UTC()
	model := initiativeItemModelFromCore(item)
	model.CreatedAt = now
	if err := s.orm.WithContext(ctx).Create(model).Error; err != nil {
		return 0, err
	}
	item.ID = model.ID
	item.CreatedAt = now
	return model.ID, nil
}

func (s *Store) ListInitiativeItems(ctx context.Context, initiativeID int64) ([]*core.InitiativeItem, error) {
	var models []InitiativeItemModel
	if err := s.orm.WithContext(ctx).
		Where("initiative_id = ?", initiativeID).
		Order("id ASC").
		Find(&models).Error; err != nil {
		return nil, err
	}
	out := make([]*core.InitiativeItem, 0, len(models))
	for i := range models {
		out = append(out, models[i].toCore())
	}
	return out, nil
}

func (s *Store) ListInitiativeItemsByWorkItem(ctx context.Context, workItemID int64) ([]*core.InitiativeItem, error) {
	var models []InitiativeItemModel
	if err := s.orm.WithContext(ctx).
		Where("work_item_id = ?", workItemID).
		Order("id ASC").
		Find(&models).Error; err != nil {
		return nil, err
	}
	out := make([]*core.InitiativeItem, 0, len(models))
	for i := range models {
		out = append(out, models[i].toCore())
	}
	return out, nil
}

func (s *Store) UpdateInitiativeItem(ctx context.Context, item *core.InitiativeItem) error {
	result := s.orm.WithContext(ctx).Model(&InitiativeItemModel{}).
		Where("initiative_id = ? AND work_item_id = ?", item.InitiativeID, item.WorkItemID).
		Updates(map[string]any{"role": item.Role})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return core.ErrNotFound
	}
	return nil
}

func (s *Store) DeleteInitiativeItem(ctx context.Context, initiativeID int64, workItemID int64) error {
	result := s.orm.WithContext(ctx).
		Where("initiative_id = ? AND work_item_id = ?", initiativeID, workItemID).
		Delete(&InitiativeItemModel{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return core.ErrNotFound
	}
	return nil
}

func (s *Store) DeleteInitiativeItemsByInitiative(ctx context.Context, initiativeID int64) error {
	return s.orm.WithContext(ctx).
		Where("initiative_id = ?", initiativeID).
		Delete(&InitiativeItemModel{}).Error
}

func (s *Store) CreateThreadInitiativeLink(ctx context.Context, link *core.ThreadInitiativeLink) (int64, error) {
	if link == nil {
		return 0, fmt.Errorf("thread initiative link is nil")
	}
	if link.RelationType == "" {
		link.RelationType = "source"
	}
	now := time.Now().UTC()
	model := threadInitiativeLinkModelFromCore(link)
	model.CreatedAt = now
	if err := s.orm.WithContext(ctx).Create(model).Error; err != nil {
		return 0, err
	}
	link.ID = model.ID
	link.CreatedAt = now
	return model.ID, nil
}

func (s *Store) ListThreadsByInitiative(ctx context.Context, initiativeID int64) ([]*core.ThreadInitiativeLink, error) {
	var models []ThreadInitiativeLinkModel
	if err := s.orm.WithContext(ctx).
		Where("initiative_id = ?", initiativeID).
		Order("id ASC").
		Find(&models).Error; err != nil {
		return nil, err
	}
	out := make([]*core.ThreadInitiativeLink, 0, len(models))
	for i := range models {
		out = append(out, models[i].toCore())
	}
	return out, nil
}

func (s *Store) ListInitiativesByThread(ctx context.Context, threadID int64) ([]*core.ThreadInitiativeLink, error) {
	var models []ThreadInitiativeLinkModel
	if err := s.orm.WithContext(ctx).
		Where("thread_id = ?", threadID).
		Order("id ASC").
		Find(&models).Error; err != nil {
		return nil, err
	}
	out := make([]*core.ThreadInitiativeLink, 0, len(models))
	for i := range models {
		out = append(out, models[i].toCore())
	}
	return out, nil
}

func (s *Store) DeleteThreadInitiativeLink(ctx context.Context, initiativeID int64, threadID int64) error {
	result := s.orm.WithContext(ctx).
		Where("initiative_id = ? AND thread_id = ?", initiativeID, threadID).
		Delete(&ThreadInitiativeLinkModel{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return core.ErrNotFound
	}
	return nil
}

func (s *Store) DeleteThreadInitiativeLinksByInitiative(ctx context.Context, initiativeID int64) error {
	return s.orm.WithContext(ctx).
		Where("initiative_id = ?", initiativeID).
		Delete(&ThreadInitiativeLinkModel{}).Error
}

func (s *Store) ListDependentWorkItems(ctx context.Context, workItemID int64) ([]*core.WorkItem, error) {
	var models []WorkItemModel
	if err := s.orm.WithContext(ctx).Order("id ASC").Find(&models).Error; err != nil {
		return nil, err
	}
	out := make([]*core.WorkItem, 0)
	for i := range models {
		workItem := models[i].toCore()
		for _, depID := range workItem.DependsOn {
			if depID == workItemID {
				out = append(out, workItem)
				break
			}
		}
	}
	return out, nil
}
