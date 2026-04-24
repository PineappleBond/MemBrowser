package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/PineappleBond/MemBrowser/backend/internal/eino/runner"
	"github.com/PineappleBond/MemBrowser/backend/internal/model"
	"github.com/PineappleBond/MemBrowser/backend/internal/ws"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"gorm.io/gorm"
)

const DefaultAgentTimeout = 20 * time.Minute

type TaskService struct {
	db            *gorm.DB
	wsMgr         *ws.Manager
	seqMgr        *ws.SeqManager
	runSessionMgr *runner.RunSessionManager
	msgQueue      *runner.MessageQueue
	log           *slog.Logger
	runner        *runner.Runner
}

func NewTaskService(
	db *gorm.DB,
	wsMgr *ws.Manager,
	seqMgr *ws.SeqManager,
	runSessionMgr *runner.RunSessionManager,
	msgQueue *runner.MessageQueue,
	log *slog.Logger,
	runner *runner.Runner,
) *TaskService {
	return &TaskService{
		db:            db,
		wsMgr:         wsMgr,
		seqMgr:        seqMgr,
		runSessionMgr: runSessionMgr,
		msgQueue:      msgQueue,
		log:           log,
		runner:        runner,
	}
}

// StartTask 提交任务到队列并异步启动 Agent。
func (s *TaskService) StartTask(taskID, taskDesc string) {
	s.msgQueue.Enqueue(taskID, taskDesc)
	go s.runAgent(context.Background(), taskID, taskDesc)
}

func (s *TaskService) StopTask(taskID string) {
	s.runSessionMgr.Stop(taskID)
}

func (s *TaskService) runAgent(ctx context.Context, taskID string, taskDesc string) {
	ctx, cancel := context.WithTimeout(ctx, DefaultAgentTimeout)

	if !s.runSessionMgr.TryStart(taskID, cancel, nil) {
		cancel()
		s.msgQueue.Enqueue(taskID, taskDesc)
		return
	}

	callbacks := runner.NewRootRunnerCallbacks(runner.RunCallbackConfig{
		TaskID:    taskID,
		SessionID: s.wsMgr.SessionID(),
		DB:        s.db,
		Log:       s.log,
		PushUpdate: func(u ws.Update) error { return s.wsMgr.PushUpdate(u) },
	})

	handler := runner.NewRootRunnerHandler(callbacks, nil)

	done := make(chan struct{})
	go func() {
		// defer 链: 保证所有退出路径都执行清理
		defer func() {
			if r := recover(); r != nil {
				s.log.Error("agent panic", "task_id", taskID, "recover", r)
				callbacks.Stop()
				s.wsMgr.PushUpdate(ws.Update{Type: ws.UpdateTaskFailed, Payload: marshal(map[string]any{
					"error": fmt.Sprintf("agent panic: %v", r),
				})})
			}
			callbacks.Stop()
			s.runSessionMgr.Cleanup(taskID)
			// pending 消息检查: Agent 结束后仍有消息入队，自动重启
			if s.msgQueue.HasPending(taskID) {
				go s.runAgent(context.WithoutCancel(ctx), taskID, "")
			} else {
				s.runSessionMgr.Done(taskID)
			}
		}()
		defer close(done)

		// 推送任务开始
		s.wsMgr.PushUpdate(ws.Update{Type: ws.UpdateTaskStarted, Payload: marshal(map[string]any{
			"task_id": taskID,
		})})

		// 检查 checkpoint + resume
		checkpointID := s.runSessionMgr.GetCheckpointID(taskID)
		var iter *adk.AsyncIterator[*adk.AgentEvent]
		if checkpointID != "" {
			resumeParams, hasResume := s.runSessionMgr.GetResumeParams(taskID)
			if hasResume {
				s.runSessionMgr.ClearResumeParams(taskID)
				var resumeErr error
				iter, resumeErr = s.runner.ResumeWithParams(ctx, checkpointID, &adk.ResumeParams{
					Targets: resumeParams,
				}, adk.WithCallbacks(handler))
				if resumeErr != nil {
					s.log.Warn("ResumeWithParams failed, falling back to Run", "task_id", taskID, "error", resumeErr)
					messages := s.loadMessages(taskID)
					iter = s.runner.Run(ctx, messages, taskID, adk.WithCallbacks(handler))
				}
			} else {
				var resumeErr error
				iter, resumeErr = s.runner.Resume(ctx, checkpointID, adk.WithCallbacks(handler))
				if resumeErr != nil {
					s.log.Warn("Resume failed, falling back to Run", "task_id", taskID, "error", resumeErr)
					messages := s.loadMessages(taskID)
					iter = s.runner.Run(ctx, messages, taskID, adk.WithCallbacks(handler))
				}
			}
		} else {
			messages := s.loadMessages(taskID)
			iter = s.runner.Run(ctx, messages, taskID, adk.WithCallbacks(handler))
		}

		// 事件循环
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			event, ok := iter.Next()
			if !ok {
				callbacks.OnEnd()
				return
			}
			if event == nil {
				continue
			}

			// 错误处理
			if event.Err != nil {
				// 路径 1: Middleware 检测的 TokenOverflow
				var overflowErr *runner.TokenOverflowError
				if errors.As(event.Err, &overflowErr) {
					for _, content := range overflowErr.DrainedMessages {
						s.msgQueue.Enqueue(taskID, content)
					}
					s.log.Warn("token overflow, restarting agent",
						"task_id", taskID,
						"tokens", overflowErr.TokenCount,
						"max", overflowErr.MaxTokens,
					)
					callbacks.Stop()
					s.runSessionMgr.Cleanup(taskID)
					go s.runAgent(context.WithoutCancel(ctx), taskID, "")
					return
				}
				// 路径 2: API 返回的 context overflow
				if isContextOverflowError(event.Err) {
					s.log.Warn("context overflow", "task_id", taskID)
					s.wsMgr.PushUpdate(ws.Update{Type: ws.UpdateTaskFailed, Payload: marshal(map[string]any{
						"error": "对话过长，请重新开始任务",
					})})
					return
				}
				s.wsMgr.PushUpdate(ws.Update{Type: ws.UpdateTaskFailed, Payload: marshal(map[string]any{
					"error": event.Err.Error(),
				})})
				return
			}

			// Interrupt 事件 (ask_human 触发)
			if event.Action != nil && event.Action.Interrupted != nil {
				s.runSessionMgr.SetCheckpointID(taskID, taskID)
				// 持久化 checkpoint ID 到 DB
				s.db.Model(&model.Session{}).Where("id = ?", taskID).
					Update("checkpoint_id", taskID)
				callbacks.OnInterrupted(map[string]any{
					"data": event.Action.Interrupted.Data,
				})
				return // 等待 resume
			}

			// 处理输出事件
			if event.Output != nil && event.Output.MessageOutput != nil {
				msg, err := event.Output.MessageOutput.GetMessage()
				if err == nil && msg != nil {
					// thinking (reasoning content)
					if msg.ReasoningContent != "" {
						s.wsMgr.PushUpdate(ws.Update{Seq: 0, Type: ws.UpdateThinking, Payload: marshal(map[string]any{
							"delta": msg.ReasoningContent,
						})})
					}
					// message delta (streaming content)
					if msg.Content != "" {
						s.wsMgr.PushUpdate(ws.Update{Seq: 0, Type: ws.UpdateMessageDelta, Payload: marshal(map[string]any{
							"delta": msg.Content,
						})})
					}
				}
			}

			// task.completed 通过 OnEnd 回调推送 (callbacks.OnEnd → callbacks.OnCompleted → PushUpdate task.step)
			// 最终完成状态在 iter.Next() 返回 !ok 时推送
		}
	}()

	// select: 等待事件循环完成或超时
	timer := time.NewTimer(DefaultAgentTimeout)
	defer timer.Stop()
	select {
	case <-done:
		// 事件循环正常结束
	case <-timer.C:
		s.log.Warn("agent timeout", "task_id", taskID)
		cancel()
		s.wsMgr.PushUpdate(ws.Update{Type: ws.UpdateTaskFailed, Payload: marshal(map[string]any{
			"error": "Agent 执行超时",
		})})
		<-done // 等待 goroutine defer 链完成，防止竞态
	}
}

func (s *TaskService) resumeAgent(ctx context.Context, taskID string, interruptID string, answerValue string) {
	s.runSessionMgr.Stop(taskID)
	s.runSessionMgr.Cleanup(taskID)

	s.db.Model(&model.Session{}).
		Where("id = ?", taskID).
		Update("checkpoint_id", "")

	s.runSessionMgr.SetResumeParams(taskID, map[string]any{
		interruptID: answerValue,
	})
	s.runSessionMgr.SetCheckpointID(taskID, taskID)

	go s.runAgent(context.WithoutCancel(ctx), taskID, "")
}

func (s *TaskService) loadMessages(taskID string) []*schema.Message {
	drained := s.msgQueue.Drain(taskID)
	messages := make([]*schema.Message, 0, len(drained))
	for _, content := range drained {
		messages = append(messages, schema.UserMessage(content))
	}
	return messages
}

func isContextOverflowError(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "context_length_exceeded") ||
		strings.Contains(msg, "prompt_too_long") ||
		strings.Contains(msg, "maximum context length")
}

func marshal(v any) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}
