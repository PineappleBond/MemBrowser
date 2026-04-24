package tools

import (
	"encoding/json"
	"time"

	"github.com/PineappleBond/MemBrowser/backend/internal/ws"
)

// WSManager WS 管理器接口，避免循环依赖
type WSManager interface {
	PushUpdate(u ws.Update) error
	SessionID() string
}

// Waiter 响应等待器接口
type Waiter interface {
	Register(id string) chan json.RawMessage
	WaitChan(ch chan json.RawMessage, id string, timeout time.Duration) (json.RawMessage, error)
	Wait(id string, timeout time.Duration) (json.RawMessage, error)
}
