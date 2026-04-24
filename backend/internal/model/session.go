package model

import "time"

// Session 会话
type Session struct {
	ID           string `gorm:"primaryKey"`
	ActiveTabID  int
	CheckpointID string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Tab 标签页
type Tab struct {
	ID          string `gorm:"primaryKey"`
	SessionID   string `gorm:"index"`
	ChromeTabID int
	URL         string
	Title       string
	Active      bool
	CreatedAt   time.Time
}
