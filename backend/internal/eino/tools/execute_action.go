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

func NewExecuteActionTool(wsMgr WSManager, waiter Waiter) (tool.InvokableTool, error) {
	return utils.InferTool("execute_action", "执行页面操作（click/input/scroll/navigate）",
		func(ctx context.Context, input ExecuteActionInput) (ExecuteActionOutput, error) {
			requestID := uuid.New().String()

			// 先注册 waiter，再推送 WS 消息，避免竞态
			ch := waiter.Register(requestID)

			payload, _ := json.Marshal(map[string]any{
				"request_id": requestID,
				"action":     input.Action,
				"selector":   input.Selector,
				"value":      input.Value,
				"url":        input.URL,
			})

			wsMgr.PushUpdate(ws.Update{
				Seq:     0,
				Type:    ws.UpdateActionExecute,
				Payload: payload,
			})

			resp, err := waiter.WaitChan(ch, requestID, 30*time.Second)
			if err != nil {
				return ExecuteActionOutput{}, err
			}

			var result ExecuteActionOutput
			if err := json.Unmarshal(resp, &result); err != nil {
				return ExecuteActionOutput{}, err
			}
			return result, nil
		})
}
