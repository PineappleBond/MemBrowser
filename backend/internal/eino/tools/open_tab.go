package tools

import (
	"context"
	"encoding/json"
	"time"

	"github.com/PineappleBond/MemBrowser/backend/internal/ws"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/google/uuid"
)

func NewOpenTabTool(wsMgr WSManager, waiter Waiter) (tool.InvokableTool, error) {
	return utils.InferTool("open_tab", "打开新标签页",
		func(ctx context.Context, input OpenTabInput) (OpenTabOutput, error) {
			requestID := uuid.New().String()

			// 先注册 waiter，再推送 WS 消息，避免竞态
			ch := waiter.Register(requestID)

			payload, _ := json.Marshal(map[string]any{
				"request_id": requestID,
				"url":        input.URL,
			})

			wsMgr.PushUpdate(ws.Update{
				Seq:     0,
				Type:    "open_tab",
				Payload: payload,
			})

			resp, err := waiter.WaitChan(ch, requestID, 30*time.Second)
			if err != nil {
				return OpenTabOutput{}, err
			}

			var result OpenTabOutput
			if err := json.Unmarshal(resp, &result); err != nil {
				return OpenTabOutput{}, err
			}
			return result, nil
		})
}
