package model

import "time"

// Checkpoint 检查点
type Checkpoint struct {
	ID        string    `gorm:"primaryKey"`
	TaskID    string    `gorm:"index"`
	NodeKey   string    `gorm:"type:varchar(128);not null"`
	State     []byte    `gorm:"type:blob;not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
}
