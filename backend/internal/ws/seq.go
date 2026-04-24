package ws

import (
	"sync/atomic"

	"github.com/PineappleBond/MemBrowser/backend/internal/model"
	"gorm.io/gorm"
)

// SeqManager 管理全局递增序列号
type SeqManager struct {
	db     *gorm.DB
	maxSeq int64
}

// NewSeqManager 创建 SeqManager 实例
func NewSeqManager(db *gorm.DB) *SeqManager {
	return &SeqManager{db: db}
}

// Init 从数据库加载当前最大序列号
func (s *SeqManager) Init() error {
	var maxSeq int64
	if err := s.db.Model(&model.Update{}).Select("COALESCE(MAX(seq), 0)").Scan(&maxSeq).Error; err != nil {
		return err
	}
	atomic.StoreInt64(&s.maxSeq, maxSeq)
	return nil
}

// Next 返回下一个递增的序列号
func (s *SeqManager) Next() int64 {
	return atomic.AddInt64(&s.maxSeq, 1)
}

// Max 返回当前最大序列号
func (s *SeqManager) Max() int64 {
	return atomic.LoadInt64(&s.maxSeq)
}
