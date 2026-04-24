package ws

import (
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/PineappleBond/MemBrowser/backend/internal/model"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"
)

type Manager struct {
	db        *gorm.DB
	upgrader  websocket.Upgrader
	seqMgr    *SeqManager
	waiter    *ResponseWaiter
	conns     map[string]*ConnState
	mu        sync.RWMutex
	authKey   string
	sessionID string
}

func NewManager(db *gorm.DB, seqMgr *SeqManager, waiter *ResponseWaiter, authKey string) *Manager {
	return &Manager{
		db:        db,
		upgrader:  websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }},
		seqMgr:    seqMgr,
		waiter:    waiter,
		conns:     make(map[string]*ConnState),
		authKey:   authKey,
		sessionID: uuid.New().String(),
	}
}

func (m *Manager) SessionID() string { return m.sessionID }

type ConnState struct {
	conn      *websocket.Conn
	sendCh    chan []byte
	sessionID string
	lastSeq   int64
	mu        sync.Mutex
}

func (m *Manager) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	lastSeq, _ := strconv.ParseInt(r.URL.Query().Get("last_seq"), 10, 64)

	if token != m.authKey {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := m.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	sessionID := uuid.New().String()
	connState := &ConnState{
		conn:      conn,
		sendCh:    make(chan []byte, 512),
		sessionID: sessionID,
		lastSeq:   lastSeq,
	}

	// V0.1 单用户: 固定 key 管理唯一连接
	const connKey = "main"
	m.kickOldConn(connKey)

	m.mu.Lock()
	m.conns[connKey] = connState
	m.mu.Unlock()

	m.sendConnected(connState)

	// 回放 missed updates (只要本地 maxSeq 大于 lastSeq 就回放)
	if lastSeq < m.seqMgr.Max() {
		m.replayUpdates(connState, lastSeq)
	}

	go m.writeLoop(connState)
	go m.readLoop(connState)
}

func (m *Manager) kickOldConn(key string) {
	m.mu.Lock()
	old, ok := m.conns[key]
	if ok {
		delete(m.conns, key)
	}
	m.mu.Unlock()
	if ok {
		old.conn.Close()
	}
}

// sendConnected 使用 blocking send，确保 connected frame 不被丢弃。
func (m *Manager) sendConnected(cs *ConnState) {
	payload := ConnectedPayload{
		SessionID:  cs.sessionID,
		ServerTime: time.Now().UTC().Format(time.RFC3339),
		MaxSeq:     m.seqMgr.Max(),
	}
	frame := ServerFrame{Type: FrameConnected, Payload: payload}
	data, _ := json.Marshal(frame)
	cs.sendCh <- data
}

func (m *Manager) PushUpdate(u Update) error {
	if u.Seq == 0 {
		frame := ServerFrame{Type: FrameUpdates, Payload: []Update{u}}
		m.broadcast(frame)
		return nil
	}
	seq := m.seqMgr.Next()
	u.Seq = seq

	var payload model.JSONMap
	if len(u.Payload) > 0 {
		if err := json.Unmarshal(u.Payload, &payload); err != nil {
			payload = model.JSONMap{}
		}
	} else {
		payload = model.JSONMap{}
	}

	m.db.Create(&model.Update{
		SessionID: m.sessionID,
		Seq:       seq,
		Type:      u.Type,
		ID:        u.ID,
		Payload:   payload,
	})

	frame := ServerFrame{Type: FrameUpdates, Payload: []Update{u}}
	m.broadcast(frame)
	return nil
}

func (m *Manager) broadcast(frame ServerFrame) {
	data, _ := json.Marshal(frame)
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, cs := range m.conns {
		select {
		case cs.sendCh <- data:
		default:
		}
	}
}
