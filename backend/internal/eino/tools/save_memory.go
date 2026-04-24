package tools

import (
	"context"

	"github.com/PineappleBond/MemBrowser/backend/internal/model"
	"github.com/PineappleBond/MemBrowser/backend/internal/service"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

func NewSaveMemoryTool(memStore *service.MemoryStore, sessionID string) (tool.InvokableTool, error) {
	return utils.InferTool("save_memory", "将成功操作存入记忆库",
		func(ctx context.Context, input SaveMemoryInput) (SaveMemoryOutput, error) {
			pageURLPattern := service.NormalizeURL(input.PageURL)

			mem := &model.Memory{
				SessionID:      sessionID,
				PageURL:        input.PageURL,
				PageURLPattern: pageURLPattern,
				ActionType:     input.ActionType,
				ActionTarget:   input.ActionTarget,
				Result:         input.Result,
				Source:         "ai",
			}

			if err := memStore.Save(mem); err != nil {
				return SaveMemoryOutput{Success: false}, err
			}
			return SaveMemoryOutput{Success: true}, nil
		})
}
