package runner

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/PineappleBond/MemBrowser/backend/pkg/token"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

// TokenOverflowError 表示中间件检测到的 token 溢出。
// DrainedMessages 保存已被排空的消息，用于溢出恢复时重新入队。
type TokenOverflowError struct {
	TokenCount      int
	MaxTokens       int
	DrainedMessages []string
}

func (e *TokenOverflowError) Error() string {
	return fmt.Sprintf("token overflow: %d > %d", e.TokenCount, e.MaxTokens)
}

// TokenCheckConfig token 溢出检测配置
type TokenCheckConfig struct {
	Counter   *token.Counter
	MaxTokens int
}

type contextInjectionMiddleware struct {
	*adk.BaseChatModelAgentMiddleware
	queue      *MessageQueue
	taskID     string
	tokenCheck *TokenCheckConfig
}

// NewContextInjectionMiddleware 创建上下文注入中间件。
func NewContextInjectionMiddleware(queue *MessageQueue, taskID string, tokenCheck *TokenCheckConfig) *contextInjectionMiddleware {
	return &contextInjectionMiddleware{
		BaseChatModelAgentMiddleware: &adk.BaseChatModelAgentMiddleware{},
		queue:                        queue,
		taskID:                       taskID,
		tokenCheck:                   tokenCheck,
	}
}

func (m *contextInjectionMiddleware) BeforeModelRewriteState(ctx context.Context, state *adk.ChatModelAgentState, mc *adk.ModelContext) (context.Context, *adk.ChatModelAgentState, error) {
	drained := m.queue.Drain(m.taskID)
	for _, content := range drained {
		state.Messages = append(state.Messages, schema.UserMessage(content))
	}

	if m.tokenCheck != nil && m.tokenCheck.Counter != nil {
		totalTokens := m.countTokens(state.Messages)
		if totalTokens > m.tokenCheck.MaxTokens {
			slog.Warn("token overflow detected in context injection",
				"task_id", m.taskID,
				"tokens", totalTokens,
				"max", m.tokenCheck.MaxTokens,
			)
			return ctx, state, &TokenOverflowError{
				TokenCount:      totalTokens,
				MaxTokens:       m.tokenCheck.MaxTokens,
				DrainedMessages: drained,
			}
		}
	}
	return ctx, state, nil
}

func (m *contextInjectionMiddleware) countTokens(messages []*schema.Message) int {
	total := 0
	for _, msg := range messages {
		total += m.tokenCheck.Counter.Count(msg.Content)
	}
	return total
}
