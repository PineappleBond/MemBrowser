package ws

import (
	"encoding/json"
	"log/slog"
	"time"
)

func (m *Manager) readLoop(cs *ConnState) {
	defer func() {
		m.mu.Lock()
		delete(m.conns, "main")
		m.mu.Unlock()
		cs.conn.Close()
	}()

	deadline := 60 * time.Second
	cs.conn.SetReadDeadline(time.Now().Add(deadline))

	for {
		_, msg, err := cs.conn.ReadMessage()
		if err != nil {
			break
		}

		var frame ClientFrame
		if err := json.Unmarshal(msg, &frame); err != nil {
			slog.Warn("invalid ws message", "error", err)
			continue
		}

		if frame.Type == ClientFramePong {
			cs.conn.SetReadDeadline(time.Now().Add(deadline))
			continue
		}
	}
}
