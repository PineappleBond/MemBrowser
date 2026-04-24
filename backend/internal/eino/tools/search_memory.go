package tools

import (
	"context"
	"fmt"

	"github.com/PineappleBond/MemBrowser/backend/internal/service"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

func NewSearchMemoryTool(memStore *service.MemoryStore) (tool.InvokableTool, error) {
	return utils.InferTool("search_memory", "在记忆库中查找相似页面的成功操作",
		func(ctx context.Context, input SearchMemoryInput) (SearchMemoryOutput, error) {
			pageURLPattern := service.NormalizeURL(input.PageURL)

			mem, err := memStore.Search(pageURLPattern, input.ActionType, "")
			if err != nil {
				return SearchMemoryOutput{Found: false}, nil
			}

			return SearchMemoryOutput{
				Found:  true,
				Memory: fmt.Sprintf("action=%s target=%s selector=%s value=%s result=%s",
					mem.ActionType, mem.ActionTarget, mem.ActionSelector, mem.ActionValue, mem.Result),
			}, nil
		})
}
