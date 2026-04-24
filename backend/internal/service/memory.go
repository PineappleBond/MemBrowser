package service

import (
	"os"
	"path/filepath"

	"github.com/PineappleBond/MemBrowser/backend/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type MemoryStore struct {
	db      *gorm.DB
	dataDir string
}

func NewMemoryStore(db *gorm.DB, dataDir string) *MemoryStore {
	return &MemoryStore{db: db, dataDir: dataDir}
}

func (s *MemoryStore) Save(m *model.Memory) error {
	if m.ID == "" {
		m.ID = uuid.New().String()
	}
	return s.db.Create(m).Error
}

func (s *MemoryStore) Search(pageURLPattern, actionType, actionTarget string) (*model.Memory, error) {
	var m model.Memory
	err := s.db.Where(
		"page_url_pattern = ? AND action_type = ? AND INSTR(action_target, ?) > 0 AND result = ?",
		pageURLPattern, actionType, actionTarget, "success",
	).Order("created_at DESC").First(&m).Error
	return &m, err
}

func (s *MemoryStore) SaveScreenshot(data []byte) (string, error) {
	dir := filepath.Join(s.dataDir, "screenshots")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, uuid.New().String()+".png")
	return path, os.WriteFile(path, data, 0644)
}

func (s *MemoryStore) SaveDOMSnapshot(data string) (string, error) {
	dir := filepath.Join(s.dataDir, "dom_snapshots")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, uuid.New().String()+".html")
	return path, os.WriteFile(path, []byte(data), 0644)
}
