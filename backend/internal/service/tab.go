package service

import (
	"sync"

	"github.com/PineappleBond/MemBrowser/backend/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// TabManager 标签页管理器
type TabManager struct {
	db          *gorm.DB
	session     *model.Session
	mu          sync.RWMutex
	agentLocked bool
}

// NewTabManager 创建 TabManager，复用或新建 Session
func NewTabManager(db *gorm.DB) *TabManager {
	var session model.Session
	if err := db.Order("created_at desc").First(&session).Error; err != nil {
		session = model.Session{ID: uuid.New().String()}
		db.Create(&session)
	}
	return &TabManager{db: db, session: &session}
}

// OnTabOpened 处理标签页打开事件
func (m *TabManager) OnTabOpened(chromeTabID int, url, title string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	tab := model.Tab{
		ID:          uuid.New().String(),
		SessionID:   m.session.ID,
		ChromeTabID: chromeTabID,
		URL:         url,
		Title:       title,
		Active:      false,
	}
	m.db.Create(&tab)

	// 第一个标签页自动激活
	var count int64
	m.db.Model(&model.Tab{}).Where("session_id = ?", m.session.ID).Count(&count)
	if count == 1 {
		m.db.Model(&tab).Update("active", true)
		m.db.Model(m.session).Update("active_tab_id", chromeTabID)
	}
}

// OnTabClosed 处理标签页关闭事件
func (m *TabManager) OnTabClosed(chromeTabID int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.db.Where("session_id = ? AND chrome_tab_id = ?", m.session.ID, chromeTabID).Delete(&model.Tab{})

	// 如果关闭的是激活标签页，切换到最近打开的标签页
	if m.session.ActiveTabID == chromeTabID {
		var next model.Tab
		if err := m.db.Where("session_id = ?", m.session.ID).
			Order("created_at desc").First(&next).Error; err == nil {
			m.db.Model(&next).Update("active", true)
			m.db.Model(m.session).Update("active_tab_id", next.ChromeTabID)
		} else {
			m.db.Model(m.session).Update("active_tab_id", 0)
		}
	}
}

// OnTabActivated 处理标签页激活事件
func (m *TabManager) OnTabActivated(chromeTabID int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 清除旧的激活状态
	m.db.Model(&model.Tab{}).Where("session_id = ?", m.session.ID).Update("active", false)

	// 设置新的激活标签页
	m.db.Model(&model.Tab{}).Where("session_id = ? AND chrome_tab_id = ?", m.session.ID, chromeTabID).
		Update("active", true)
	m.db.Model(m.session).Update("active_tab_id", chromeTabID)
}

// LockAgent 锁定 Agent，禁止自动切换标签页
func (m *TabManager) LockAgent() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.agentLocked = true
}

// UnlockAgent 解锁 Agent，允许自动切换标签页
func (m *TabManager) UnlockAgent() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.agentLocked = false
}

// IsAgentLocked 返回 Agent 是否被锁定
func (m *TabManager) IsAgentLocked() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.agentLocked
}

// ListTabs 列出当前 Session 的所有标签页
func (m *TabManager) ListTabs() []model.Tab {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var tabs []model.Tab
	m.db.Where("session_id = ?", m.session.ID).Order("created_at desc").Find(&tabs)
	return tabs
}

// SessionID 返回当前会话 ID
func (m *TabManager) SessionID() string {
	return m.session.ID
}
