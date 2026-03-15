package sqlite

import (
	"context"

	"github.com/yoke233/ai-workflow/internal/core"
	"gorm.io/gorm"
)

func (s *Store) CreateThreadAttachment(ctx context.Context, att *core.ThreadAttachment) (int64, error) {
	model := threadAttachmentModelFromCore(att)
	if err := s.orm.WithContext(ctx).Create(model).Error; err != nil {
		return 0, err
	}
	return model.ID, nil
}

func (s *Store) GetThreadAttachment(ctx context.Context, id int64) (*core.ThreadAttachment, error) {
	var model ThreadAttachmentModel
	if err := s.orm.WithContext(ctx).Where("id = ?", id).First(&model).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, core.ErrNotFound
		}
		return nil, err
	}
	return model.toCore(), nil
}

func (s *Store) ListThreadAttachments(ctx context.Context, threadID int64) ([]*core.ThreadAttachment, error) {
	var models []ThreadAttachmentModel
	if err := s.orm.WithContext(ctx).Where("thread_id = ?", threadID).Order("created_at ASC").Find(&models).Error; err != nil {
		return nil, err
	}
	out := make([]*core.ThreadAttachment, len(models))
	for i := range models {
		out[i] = models[i].toCore()
	}
	return out, nil
}

func (s *Store) DeleteThreadAttachment(ctx context.Context, id int64) error {
	result := s.orm.WithContext(ctx).Where("id = ?", id).Delete(&ThreadAttachmentModel{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return core.ErrNotFound
	}
	return nil
}

func (s *Store) DeleteThreadAttachmentsByThread(ctx context.Context, threadID int64) error {
	return s.orm.WithContext(ctx).Where("thread_id = ?", threadID).Delete(&ThreadAttachmentModel{}).Error
}
