package tools

import (
	"context"

	"github.com/PineappleBond/MemBrowser/backend/internal/service"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

func NewSwitchTabTool(tabMgr *service.TabManager) (tool.InvokableTool, error) {
	return utils.InferTool("switch_tab", "切换活跃标签页",
		func(ctx context.Context, input SwitchTabInput) (SwitchTabOutput, error) {
			tabMgr.OnTabActivated(input.TabID)
			return SwitchTabOutput{Success: true}, nil
		})
}
