package runner

import (
	"context"

	"github.com/PineappleBond/MemBrowser/backend/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// DBCheckPointStore 基于数据库的检查点存储
type DBCheckPointStore struct {
	db *gorm.DB
}

func NewDBCheckPointStore(db *gorm.DB) *DBCheckPointStore {
	return &DBCheckPointStore{db: db}
}

func (s *DBCheckPointStore) Get(ctx context.Context, checkpointID string) ([]byte, bool, error) {
	var cp model.Checkpoint
	err := s.db.WithContext(ctx).
		Where("task_id = ?", checkpointID).
		Order("updated_at desc").
		First(&cp).Error
	if err == gorm.ErrRecordNotFound {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return cp.State, true, nil
}

func (s *DBCheckPointStore) Set(ctx context.Context, checkpointID string, data []byte) error {
	var cp model.Checkpoint
	err := s.db.WithContext(ctx).
		Where("task_id = ?", checkpointID).
		First(&cp).Error

	if err == gorm.ErrRecordNotFound {
		cp = model.Checkpoint{
			ID:     uuid.New().String(),
			TaskID: checkpointID,
			State:  data,
		}
		return s.db.WithContext(ctx).Create(&cp).Error
	}
	if err != nil {
		return err
	}
	cp.State = data
	return s.db.WithContext(ctx).Save(&cp).Error
}
