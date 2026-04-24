package tools

import (
	"context"

	"github.com/PineappleBond/MemBrowser/backend/internal/service"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

func NewListTabsTool(tabMgr *service.TabManager) (tool.InvokableTool, error) {
	return utils.InferTool("list_tabs", "列出当前 Session 的所有标签页",
		func(ctx context.Context, input ListTabsInput) (ListTabsOutput, error) {
			tabs := tabMgr.ListTabs()
			result := make([]TabInfo, len(tabs))
			for i, tab := range tabs {
				result[i] = TabInfo{
					ID:     tab.ChromeTabID,
					URL:    tab.URL,
					Title:  tab.Title,
					Active: tab.Active,
				}
			}
			return ListTabsOutput{Tabs: result}, nil
		})
}
