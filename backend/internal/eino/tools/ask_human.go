package tools

import (
	"context"
	"encoding/gob"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

func init() {
	gob.Register(map[string]any{})
}

type AskHumanInput struct {
	Message           string `json:"message" jsonschema:"description=需要用户帮助的问题"`
	HighlightSelector string `json:"highlight_selector,omitempty" jsonschema:"description=需要高亮的CSS选择器"`
}

type AskHumanOutput struct {
	Result string `json:"result,omitempty"`
}

func NewAskHumanTool() (tool.InvokableTool, error) {
	return utils.InferTool("ask_human", "请求人类帮助，暂停 Agent 等待用户操作",
		func(ctx context.Context, input AskHumanInput) (AskHumanOutput, error) {
			wasInterrupted, _, _ := tool.GetInterruptState[any](ctx)
			if wasInterrupted {
				isTarget, hasData, data := tool.GetResumeContext[string](ctx)
				if isTarget && hasData {
					return AskHumanOutput{Result: data}, nil
				}
				return AskHumanOutput{}, tool.Interrupt(ctx, nil)
			}
			return AskHumanOutput{}, tool.Interrupt(ctx, map[string]any{
				"type":               "ask_human",
				"message":            input.Message,
				"highlight_selector": input.HighlightSelector,
			})
		})
}
