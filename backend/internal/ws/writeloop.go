package ws

import (
	"encoding/json"
	"time"

	"github.com/gorilla/websocket"
)

func (m *Manager) writeLoop(cs *ConnState) {
	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()
	defer cs.conn.Close()

	for {
		select {
		case data, ok := <-cs.sendCh:
			if !ok {
				return
			}
			cs.mu.Lock()
			err := cs.conn.WriteMessage(websocket.TextMessage, data)
			cs.mu.Unlock()
			if err != nil {
				return
			}
		case <-pingTicker.C:
			ping, _ := json.Marshal(ServerFrame{Type: FramePing, Payload: map[string]any{}})
			cs.mu.Lock()
			err := cs.conn.WriteMessage(websocket.TextMessage, ping)
			cs.mu.Unlock()
			if err != nil {
				return
			}
		}
	}
}
