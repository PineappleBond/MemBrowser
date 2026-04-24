package model

import "time"

// Memory 记忆
type Memory struct {
	ID              string    `gorm:"primaryKey"`
	SessionID       string    `gorm:"index"`
	PageURL         string
	PageURLPattern  string    `gorm:"index"`
	PageTitle       string
	PageFeatures    string
	DOMSnapshotPath string
	ScreenshotPath  string
	ActionType      string
	ActionTarget    string
	ActionSelector  string
	ActionValue     string
	Result          string
	FailReason      string
	Source          string
	CreatedAt       time.Time
}
