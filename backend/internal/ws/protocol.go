package ws

import "encoding/json"

const (
	FrameConnected = "connected"
	FrameUpdates   = "updates"
	FramePing      = "ping"
)

const (
	ClientFramePong = "pong"
)

const (
	UpdateTaskStarted   = "task.started"
	UpdateTaskStep      = "task.step"
	UpdateTaskCompleted = "task.completed"
	UpdateTaskFailed    = "task.failed"
	UpdateNeedHelp      = "need_help"
	UpdateThinking      = "thinking"
	UpdateMessageDelta  = "message.delta"
	UpdatePageQuery     = "page.query"
	UpdateActionExecute = "action.execute"
	UpdateEmpty         = "empty"
)

type ServerFrame struct {
	Type    string `json:"type"`
	Payload any    `json:"payload"`
}

type ClientFrame struct {
	Type    string         `json:"type"`
	Payload map[string]any `json:"payload,omitempty"`
}

type Update struct {
	Seq     int64           `json:"seq"`
	Type    string          `json:"type"`
	ID      string          `json:"id"`
	Payload json.RawMessage `json:"payload"`
}

type ConnectedPayload struct {
	SessionID  string `json:"session_id"`
	ServerTime string `json:"server_time"`
	MaxSeq     int64  `json:"max_seq"`
}
