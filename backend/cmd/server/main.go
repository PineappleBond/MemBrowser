package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/PineappleBond/MemBrowser/backend/internal/config"
	"github.com/PineappleBond/MemBrowser/backend/internal/db"
	"github.com/PineappleBond/MemBrowser/backend/internal/eino"
	"github.com/PineappleBond/MemBrowser/backend/internal/eino/runner"
	"github.com/PineappleBond/MemBrowser/backend/internal/eino/tools"
	"github.com/PineappleBond/MemBrowser/backend/internal/handler"
	"github.com/PineappleBond/MemBrowser/backend/internal/service"
	"github.com/PineappleBond/MemBrowser/backend/internal/ws"
	"github.com/PineappleBond/MemBrowser/backend/pkg/logger"
)

func main() {
	// 加载配置
	cfg := config.Load()

	// 初始化日志
	log := logger.New(cfg.LogLevel)
	slog.SetDefault(log)

	// 初始化数据库
	database, err := db.NewDB(cfg)
	if err != nil {
		log.Error("failed to init database", "error", err)
		os.Exit(1)
	}
	log.Info("database initialized", "path", cfg.DBPath)

	// 初始化 SeqManager
	seqMgr := ws.NewSeqManager(database)
	if err := seqMgr.Init(); err != nil {
		log.Error("failed to init seq manager", "error", err)
		os.Exit(1)
	}

	// 确保 AuthKey 已设置
	if cfg.AuthKey == "" {
		log.Warn("MEMBROWSER_AUTH_KEY not set, generating random key")
		cfg.AuthKey = fmt.Sprintf("membrowser-%s", generateRandomKey())
	}

	// 初始化核心组件
	waiter := ws.NewResponseWaiter()
	wsMgr := ws.NewManager(database, seqMgr, waiter, cfg.AuthKey)
	tabMgr := service.NewTabManager(database)

	// 初始化 Eino 组件
	modelProvider := eino.NewModelProvider(cfg)
	memStore := service.NewMemoryStore(database, cfg.DataDir)
	toolRegistry := tools.NewRegistry(tools.RegistryDeps{
		WSManager: wsMgr,
		Waiter:    waiter,
		TabMgr:    tabMgr,
		MemStore:  memStore,
		SessionID: wsMgr.SessionID(),
		Log:       log,
	})
	runSessionMgr := runner.NewRunSessionManager()
	msgQueue := runner.NewMessageQueue()

	// 初始化 Agent Runner (CheckPointStore 由 Runner 持有，实际每次 run 用 adk.WithCallbacks 覆盖)
	agentRunner, err := runner.NewRunner(runner.RunnerConfig{
		ModelProvider: modelProvider,
		Tier:          "sonnet",
		Tools:         toolRegistry.GetTools(),
		CheckPointStore: runner.NewDBCheckPointStore(database),
	})
	if err != nil {
		log.Error("failed to create agent runner", "error", err)
		os.Exit(1)
	}

	// 初始化 TaskService
	taskSvc := service.NewTaskService(database, wsMgr, seqMgr, runSessionMgr, msgQueue, log, agentRunner)

	// 初始化 Handler 和 Router
	h := handler.NewHandler(taskSvc, wsMgr, tabMgr, waiter, memStore, database, seqMgr, log)
	router := handler.NewRouter(h)

	// 启动 HTTP 服务器
	addr := fmt.Sprintf(":%d", cfg.Port)
	log.Info("starting server", "addr", addr)

	srv := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	// 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down server")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Error("server forced to shutdown", "error", err)
	}
}

func generateRandomKey() string {
	b := make([]byte, 16)
	for i := range b {
		b[i] = alphabet[byte(time.Now().UnixNano())%byte(len(alphabet))]
	}
	return string(b)
}

var alphabet = []byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
