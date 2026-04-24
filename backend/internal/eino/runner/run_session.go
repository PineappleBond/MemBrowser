package runner

import (
	"sync"
	"time"
)

type AgentRunSession struct {
	CancelFunc   func()
	TaskID       string
	CheckpointID string
	ResumeParams map[string]any
	IsRunning    bool
	StopFunc     func()
	done         chan struct{}
	doneOnce     sync.Once
	ParentID     string
}

type RunSessionManager struct {
	mu       sync.RWMutex
	sessions map[string]*AgentRunSession
}

func NewRunSessionManager() *RunSessionManager {
	return &RunSessionManager{sessions: make(map[string]*AgentRunSession)}
}

func (m *RunSessionManager) createSession(taskID string, cancel func(), stopFunc func(), parentID string) {
	s := &AgentRunSession{
		CancelFunc: cancel,
		TaskID:     taskID,
		IsRunning:  true,
		StopFunc:   stopFunc,
		done:       make(chan struct{}),
		ParentID:   parentID,
	}
	m.sessions[taskID] = s
}

func (m *RunSessionManager) TryStart(taskID string, cancel func(), stopFunc func(), parentID ...string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	pid := ""
	if len(parentID) > 0 {
		pid = parentID[0]
	}
	if existing, ok := m.sessions[taskID]; ok {
		if existing.IsRunning {
			return false
		}
		m.createSession(taskID, cancel, stopFunc, pid)
		m.sessions[taskID].CheckpointID = existing.CheckpointID
		m.sessions[taskID].ResumeParams = existing.ResumeParams
		return true
	}
	m.createSession(taskID, cancel, stopFunc, pid)
	return true
}

func (m *RunSessionManager) Stop(taskID string) {
	// 1. RLock 收集子会话 IDs
	m.mu.RLock()
	var childIDs []string
	for id, s := range m.sessions {
		if s.ParentID == taskID && s.IsRunning {
			childIDs = append(childIDs, id)
		}
	}
	m.mu.RUnlock()

	// 2. 递归停止子会话
	for _, childID := range childIDs {
		m.Stop(childID)
	}

	// 3. Lock 内读取、设置、提取 -> Unlock -> 锁外执行
	m.mu.Lock()
	s, ok := m.sessions[taskID]
	if !ok || !s.IsRunning {
		m.mu.Unlock()
		return
	}
	s.IsRunning = false
	cancelFn := s.CancelFunc
	stopFn := s.StopFunc
	doneCh := s.done
	m.mu.Unlock()

	if cancelFn != nil {
		cancelFn()
	}
	if stopFn != nil {
		stopFn()
	}
	select {
	case <-doneCh:
	case <-time.After(2 * time.Second):
	}
}

// Done 标记会话完成，使用 sync.Once 防止双重 close panic。
func (m *RunSessionManager) Done(taskID string) {
	m.mu.RLock()
	s, ok := m.sessions[taskID]
	m.mu.RUnlock()
	if ok {
		s.doneOnce.Do(func() { close(s.done) })
	}
}

func (m *RunSessionManager) Cleanup(taskID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, taskID)
}

func (m *RunSessionManager) SetResumeParams(taskID string, params map[string]any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[taskID]
	if !ok {
		s = &AgentRunSession{IsRunning: false, done: make(chan struct{})}
		s.doneOnce.Do(func() { close(s.done) })
		m.sessions[taskID] = s
	}
	s.ResumeParams = params
}

func (m *RunSessionManager) GetResumeParams(taskID string) (map[string]any, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[taskID]
	if !ok || s.ResumeParams == nil {
		return nil, false
	}
	return s.ResumeParams, true
}

func (m *RunSessionManager) ClearResumeParams(taskID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.sessions[taskID]; ok {
		s.ResumeParams = nil
	}
}

func (m *RunSessionManager) SetCheckpointID(taskID string, id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.sessions[taskID]; ok {
		s.CheckpointID = id
	}
}

func (m *RunSessionManager) GetCheckpointID(taskID string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if s, ok := m.sessions[taskID]; ok {
		return s.CheckpointID
	}
	return ""
}
