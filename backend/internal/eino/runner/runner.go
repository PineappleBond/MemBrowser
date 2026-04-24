package runner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/PineappleBond/MemBrowser/backend/internal/eino/prompts"
	"github.com/PineappleBond/MemBrowser/backend/pkg/token"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/middlewares/reduction"
	"github.com/cloudwego/eino/adk/prebuilt/deep"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

// ModelProvider 模型提供者接口
type ModelProvider interface {
	GetModel(tier string) (model.BaseChatModel, error)
}

// RunnerConfig Agent Runner 配置
type RunnerConfig struct {
	ModelProvider   ModelProvider
	Tier            string
	SystemPrompt    string
	Tools           []tool.BaseTool
	MaxIteration    int
	CheckPointStore adk.CheckPointStore
	MessageQueue    *MessageQueue
	TokenCounter    *token.Counter
	MaxTokens       int
	ReductionEnabled bool
}

// Runner 封装 adk.Runner，提供简化的 Run/Resume 接口
type Runner struct {
	runner *adk.Runner
}

// NewRunner 创建 Runner 实例
func NewRunner(cfg RunnerConfig) (*Runner, error) {
	chatModel, err := cfg.ModelProvider.GetModel(cfg.Tier)
	if err != nil {
		return nil, fmt.Errorf("get model: %w", err)
	}

	instruction := cfg.SystemPrompt
	if instruction == "" {
		instruction = prompts.SystemPrompt
	}

	maxIter := cfg.MaxIteration
	if maxIter == 0 {
		maxIter = 20
	}

	// 中间件链: ContextInjection → Reduction
	var handlers []adk.ChatModelAgentMiddleware

	// 1. ContextInjection (消息注入 + token 检查)
	if cfg.MessageQueue != nil {
		var tokenCheck *TokenCheckConfig
		if cfg.TokenCounter != nil {
			tokenCheck = &TokenCheckConfig{Counter: cfg.TokenCounter, MaxTokens: cfg.MaxTokens}
		}
		handlers = append(handlers, NewContextInjectionMiddleware(
			cfg.MessageQueue, "", tokenCheck,
		))
	}

	// 2. Reduction (大 DOM 数据裁剪)
	if cfg.ReductionEnabled {
		reductionMW, err := reduction.New(context.Background(), &reduction.Config{
			SkipTruncation:            true,
			MaxTokensForClear:         100000,
			ClearRetentionSuffixLimit: 2,
			RootDir:                   filepath.Join(os.TempDir(), "membrowser-reduction"),
			ReadFileToolName:          "",
		})
		if err != nil {
			return nil, fmt.Errorf("reduction init: %w", err)
		}
		handlers = append(handlers, reductionMW)
	}

	deepAgent, err := deep.New(context.Background(), &deep.Config{
		Name:        "membrowser",
		Description: "Web 自动化 Agent",
		ChatModel:   chatModel,
		Instruction: instruction,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools:               cfg.Tools,
				UnknownToolsHandler: unknownToolsHandler,
			},
			EmitInternalEvents: true,
		},
		MaxIteration:      maxIter,
		Handlers:          handlers,
		WithoutWriteTodos: true,
	})
	if err != nil {
		return nil, fmt.Errorf("create deep agent: %w", err)
	}

	runner := adk.NewRunner(context.Background(), adk.RunnerConfig{
		Agent:           deepAgent,
		EnableStreaming: true,
		CheckPointStore: cfg.CheckPointStore,
	})

	return &Runner{runner: runner}, nil
}

// Run 启动新的 Agent 执行，自动注入 checkpointID。
func (r *Runner) Run(ctx context.Context, messages []*schema.Message, checkpointID string, opts ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	opts = append([]adk.AgentRunOption{adk.WithCheckPointID(checkpointID)}, opts...)
	return r.runner.Run(ctx, messages, opts...)
}

// Resume 从检查点恢复执行，自动注入 checkpointID。
func (r *Runner) Resume(ctx context.Context, checkpointID string, opts ...adk.AgentRunOption) (*adk.AsyncIterator[*adk.AgentEvent], error) {
	opts = append([]adk.AgentRunOption{adk.WithCheckPointID(checkpointID)}, opts...)
	return r.runner.Resume(ctx, checkpointID, opts...)
}

// ResumeWithParams 从检查点恢复执行，带恢复参数，自动注入 checkpointID。
func (r *Runner) ResumeWithParams(ctx context.Context, checkpointID string, params *adk.ResumeParams, opts ...adk.AgentRunOption) (*adk.AsyncIterator[*adk.AgentEvent], error) {
	opts = append([]adk.AgentRunOption{adk.WithCheckPointID(checkpointID)}, opts...)
	return r.runner.ResumeWithParams(ctx, checkpointID, params, opts...)
}

func unknownToolsHandler(ctx context.Context, name, input string) (string, error) {
	return "", fmt.Errorf("unknown tool: %s", name)
}
