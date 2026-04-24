package ws

import (
	"encoding/json"

	"github.com/PineappleBond/MemBrowser/backend/internal/model"
)

func FillGaps(lastSeq int64, updates []model.Update) []Update {
	result := make([]Update, 0, len(updates))
	expectedSeq := lastSeq + 1
	for _, u := range updates {
		for u.Seq > expectedSeq {
			result = append(result, Update{
				Seq:     expectedSeq,
				Type:    UpdateEmpty,
				Payload: json.RawMessage(`{}`),
			})
			expectedSeq++
		}

		var payload json.RawMessage
		if u.Payload != nil {
			payload, _ = json.Marshal(u.Payload)
		} else {
			payload = json.RawMessage(`{}`)
		}

		result = append(result, Update{
			Seq:     u.Seq,
			Type:    u.Type,
			ID:      u.ID,
			Payload: payload,
		})
		expectedSeq = u.Seq + 1
	}
	return result
}
