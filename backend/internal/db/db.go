package db

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/PineappleBond/MemBrowser/backend/internal/config"
	"github.com/PineappleBond/MemBrowser/backend/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// NewDB 创建数据库连接
func NewDB(cfg *config.Config) (*gorm.DB, error) {
	// 确保目录存在
	dir := filepath.Dir(cfg.DBPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	db, err := gorm.Open(sqlite.Open(cfg.DBPath+"?_journal_mode=WAL&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	// AutoMigrate
	if err := db.AutoMigrate(
		&model.Memory{},
		&model.Session{},
		&model.Tab{},
		&model.Update{},
		&model.Checkpoint{},
	); err != nil {
		return nil, fmt.Errorf("auto migrate: %w", err)
	}

	return db, nil
}
