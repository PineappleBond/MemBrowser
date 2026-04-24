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

func NewGetPageStateTool(wsMgr WSManager, waiter Waiter) (tool.InvokableTool, error) {
	return utils.InferTool("get_page_state", "获取当前页面状态（DOM、URL、标题）",
		func(ctx context.Context, input GetPageStateInput) (GetPageStateOutput, error) {
			requestID := uuid.New().String()

			// 先注册 waiter，再推送 WS 消息，避免竞态
			ch := waiter.Register(requestID)

			payload, _ := json.Marshal(map[string]any{
				"request_id":         requestID,
				"include_screenshot": input.IncludeScreenshot,
			})

			wsMgr.PushUpdate(ws.Update{
				Seq:     0,
				Type:    ws.UpdatePageQuery,
				Payload: payload,
			})

			resp, err := waiter.WaitChan(ch, requestID, 30*time.Second)
			if err != nil {
				return GetPageStateOutput{}, err
			}

			var result GetPageStateOutput
			if err := json.Unmarshal(resp, &result); err != nil {
				return GetPageStateOutput{}, err
			}
			return result, nil
		})
}
