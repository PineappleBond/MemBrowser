package ws

import (
	"encoding/json"

	"github.com/PineappleBond/MemBrowser/backend/internal/model"
)

func (m *Manager) replayUpdates(cs *ConnState, lastSeq int64) {
	var updates []model.Update
	m.db.Where("seq > ? AND seq <= ?", lastSeq, m.seqMgr.Max()).
		Order("seq ASC").Limit(500).Find(&updates)

	result := FillGaps(lastSeq, updates)

	frame := ServerFrame{Type: FrameUpdates, Payload: result}
	data, _ := json.Marshal(frame)
	select {
	case cs.sendCh <- data:
	default:
	}
}
