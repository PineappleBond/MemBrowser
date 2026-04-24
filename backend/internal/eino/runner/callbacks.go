package runner

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"

	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/PineappleBond/MemBrowser/backend/internal/model"
	"github.com/PineappleBond/MemBrowser/backend/internal/ws"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type RunCallbackConfig struct {
	TaskID     string
	SessionID  string
	DB         *gorm.DB
	Log        *slog.Logger
	PushUpdate func(ws.Update) error
}

type RootRunnerCallbacks struct {
	cfg                   RunCallbackConfig
	mu                    sync.Mutex
	totalPromptTokens     int
	totalCompletionTokens int
	stopped               bool
}

func NewRootRunnerCallbacks(cfg RunCallbackConfig) *RootRunnerCallbacks {
	return &RootRunnerCallbacks{cfg: cfg}
}

func (c *RootRunnerCallbacks) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stopped {
		return
	}
	c.stopped = true
	c.cfg.Log.Info("agent stopped",
		"task_id", c.cfg.TaskID,
		"prompt_tokens", c.totalPromptTokens,
		"completion_tokens", c.totalCompletionTokens,
	)
}

func (c *RootRunnerCallbacks) OnThinking(ctx context.Context, reasoningContent string) {
	c.cfg.PushUpdate(ws.Update{
		Seq:     0,
		Type:    ws.UpdateThinking,
		Payload: marshal(map[string]any{"delta": reasoningContent}),
	})
}

func (c *RootRunnerCallbacks) OnOutputting(ctx context.Context, content string) {
	c.cfg.PushUpdate(ws.Update{
		Seq:     0,
		Type:    ws.UpdateMessageDelta,
		Payload: marshal(map[string]any{"delta": content}),
	})
}

func (c *RootRunnerCallbacks) OnCompleted(ctx context.Context, reasoningContent string, outputContent string, usage *einomodel.TokenUsage) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if usage != nil {
		c.totalPromptTokens += usage.PromptTokens
		c.totalCompletionTokens += usage.CompletionTokens
	}
	c.cfg.PushUpdate(ws.Update{
		Type:    ws.UpdateTaskStep,
		Payload: marshal(map[string]any{"content": outputContent}),
	})
}

// CheckPointStore 实现
func (c *RootRunnerCallbacks) Get(ctx context.Context, checkpointID string) ([]byte, bool, error) {
	var cp model.Checkpoint
	err := c.cfg.DB.WithContext(ctx).
		Where("task_id = ? AND node_key = ?", c.cfg.TaskID, checkpointID).
		First(&cp).Error
	if err == gorm.ErrRecordNotFound {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return cp.State, true, nil
}

func (c *RootRunnerCallbacks) Set(ctx context.Context, checkpointID string, data []byte) error {
	var cp model.Checkpoint
	err := c.cfg.DB.WithContext(ctx).
		Where("task_id = ? AND node_key = ?", c.cfg.TaskID, checkpointID).
		First(&cp).Error

	cp.TaskID = c.cfg.TaskID
	cp.NodeKey = checkpointID
	cp.State = data

	if err == gorm.ErrRecordNotFound {
		cp.ID = uuid.New().String()
		return c.cfg.DB.WithContext(ctx).Create(&cp).Error
	}
	if err != nil {
		return err
	}
	return c.cfg.DB.WithContext(ctx).Save(&cp).Error
}

func (c *RootRunnerCallbacks) OnError(err error) {
	c.cfg.Log.Error("agent error", "task_id", c.cfg.TaskID, "error", err)
}

func (c *RootRunnerCallbacks) OnEnd() {
	c.Stop()
	// 推送 task.completed
	c.cfg.PushUpdate(ws.Update{
		Type:    ws.UpdateTaskCompleted,
		Payload: marshal(map[string]any{
			"prompt_tokens":     c.totalPromptTokens,
			"completion_tokens": c.totalCompletionTokens,
		}),
	})
}

func (c *RootRunnerCallbacks) OnInterrupted(info map[string]any) {
	question := ""
	if msg, ok := info["message"]; ok {
		question, _ = msg.(string)
	}
	c.cfg.PushUpdate(ws.Update{
		Type:    ws.UpdateNeedHelp,
		Payload: marshal(map[string]any{"question": question, "info": info}),
	})
}

func marshal(v any) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}
