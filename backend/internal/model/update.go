package model

import "time"

// Update 更新记录
type Update struct {
	ID        string  `gorm:"primaryKey"`
	SessionID string  `gorm:"index"`
	Seq       int64   `gorm:"uniqueIndex"`
	Type      string
	Payload   JSONMap
	CreatedAt time.Time
}
