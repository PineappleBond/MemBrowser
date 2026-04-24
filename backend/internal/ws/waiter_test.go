package ws

import (
	"encoding/json"
	"testing"
	"time"
)

func TestResponseWaiter_Wait_Timeout(t *testing.T) {
	w := NewResponseWaiter()
	_, err := w.Wait("nonexistent", 100*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

func TestResponseWaiter_Wait_Resolve(t *testing.T) {
	w := NewResponseWaiter()
	data := json.RawMessage(`{"url":"http://example.com","title":"test"}`)

	go func() {
		time.Sleep(50 * time.Millisecond)
		w.Resolve("req-1", data)
	}()

	resp, err := w.Wait("req-1", 2*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(resp) != string(data) {
		t.Errorf("resp = %s, want %s", resp, data)
	}
}

func TestResponseWaiter_RegisterFirst_NoRace(t *testing.T) {
	w := NewResponseWaiter()
	data := json.RawMessage(`{"success":true}`)

	// 先注册，再推送，再等待 — 模拟工具的 register-first 模式
	ch := w.Register("req-2")

	// 模拟 Extension 响应（在 WaitChan 之前 Resolve）
	go func() {
		w.Resolve("req-2", data)
	}()

	resp, err := w.WaitChan(ch, "req-2", 2*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(resp) != string(data) {
		t.Errorf("resp = %s, want %s", resp, data)
	}
}

func TestResponseWaiter_Resolve_UnknownID(t *testing.T) {
	w := NewResponseWaiter()
	// 不应 panic，只打日志
	w.Resolve("unknown-id", json.RawMessage(`{}`))
}
