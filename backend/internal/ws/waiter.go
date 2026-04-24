package ws

import (
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/PineappleBond/MemBrowser/backend/pkg/errors"
)

type ResponseWaiter struct {
	pending map[string]chan json.RawMessage
	mu      sync.RWMutex
}

func NewResponseWaiter() *ResponseWaiter {
	return &ResponseWaiter{pending: make(map[string]chan json.RawMessage)}
}

// Register 注册一个等待通道，返回 channel。调用方先 Register，再 PushUpdate，再 WaitChan。
func (w *ResponseWaiter) Register(id string) chan json.RawMessage {
	ch := make(chan json.RawMessage, 1)
	w.mu.Lock()
	w.pending[id] = ch
	w.mu.Unlock()
	return ch
}

// WaitChan 等待已注册的通道返回结果。
func (w *ResponseWaiter) WaitChan(ch chan json.RawMessage, id string, timeout time.Duration) (json.RawMessage, error) {
	defer func() {
		w.mu.Lock()
		delete(w.pending, id)
		w.mu.Unlock()
	}()

	select {
	case resp := <-ch:
		return resp, nil
	case <-time.After(timeout):
		return nil, errors.New(errors.ErrCodeTimeout, "等待 Extension 响应超时")
	}
}

// Wait 注册 + 等待（兼容旧调用）。
func (w *ResponseWaiter) Wait(id string, timeout time.Duration) (json.RawMessage, error) {
	ch := w.Register(id)
	return w.WaitChan(ch, id, timeout)
}

func (w *ResponseWaiter) Resolve(id string, data json.RawMessage) {
	w.mu.RLock()
	ch, ok := w.pending[id]
	w.mu.RUnlock()
	if ok {
		ch <- data
	} else {
		slog.Debug("Resolve: no pending waiter for id, response dropped", "id", id)
	}
}

func (w *ResponseWaiter) Clear() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.pending = make(map[string]chan json.RawMessage)
}
