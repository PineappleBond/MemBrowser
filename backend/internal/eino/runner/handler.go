package runner

import (
	"context"
	"io"
	"sync/atomic"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"go.opentelemetry.io/otel/trace"
)

// RootRunnerHandlerCallback 回调接口
type RootRunnerHandlerCallback interface {
	OnThinking(ctx context.Context, reasoningContent string)
	OnOutputting(ctx context.Context, content string)
	OnCompleted(ctx context.Context, reasoningContent string, outputContent string, usage *model.TokenUsage)
}

// RootRunnerHandler 实现 callbacks.Handler
type RootRunnerHandler struct {
	callback         RootRunnerHandlerCallback
	tracer           trace.Tracer
	onCompletedTimes *atomic.Int32
	LatestOutput     *model.CallbackOutput
}

func NewRootRunnerHandler(callback RootRunnerHandlerCallback, tracer trace.Tracer) *RootRunnerHandler {
	return &RootRunnerHandler{
		callback:         callback,
		tracer:           tracer,
		onCompletedTimes: &atomic.Int32{},
	}
}

// OnStart 处理模型调用开始
func (h *RootRunnerHandler) OnStart(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
	return ctx
}

// OnEnd 处理模型调用结束
func (h *RootRunnerHandler) OnEnd(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
	conv := model.ConvCallbackOutput(output)
	if conv == nil {
		return ctx
	}
	h.LatestOutput = conv
	if h.callback != nil && conv.Message != nil {
		h.callback.OnCompleted(ctx, conv.Message.ReasoningContent, conv.Message.Content, conv.TokenUsage)
	}
	return ctx
}

// OnError 处理错误
func (h *RootRunnerHandler) OnError(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
	return ctx
}

// OnStartWithStreamInput 处理流式输入开始
func (h *RootRunnerHandler) OnStartWithStreamInput(ctx context.Context, info *callbacks.RunInfo, input *schema.StreamReader[callbacks.CallbackInput]) context.Context {
	return ctx
}

// OnEndWithStreamOutput 处理流式输出结束
func (h *RootRunnerHandler) OnEndWithStreamOutput(ctx context.Context, info *callbacks.RunInfo, output *schema.StreamReader[callbacks.CallbackOutput]) context.Context {
	go func() {
		defer output.Close()
		for {
			chunk, err := output.Recv()
			if err == io.EOF {
				return
			}
			if err != nil {
				return
			}
			conv := model.ConvCallbackOutput(chunk)
			if conv == nil || conv.Message == nil {
				continue
			}
			h.LatestOutput = conv
			if h.callback != nil {
				if conv.Message.ReasoningContent != "" {
					h.callback.OnThinking(ctx, conv.Message.ReasoningContent)
				}
				if conv.Message.Content != "" {
					h.callback.OnOutputting(ctx, conv.Message.Content)
				}
			}
		}
	}()
	return ctx
}
