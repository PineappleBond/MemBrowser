# MemBrowser V0.1 实现计划

> **日期**: 2026-04-23
> **设计文档**: `docs/plans/2026-04-23-membrowser-v01-design.md`
> **参考项目**: `/Users/leichujun/go/src/github.com/PineappleBond/eino-demo-dev/`

---

## 总览

| 阶段 | 内容 | 依赖 |
|------|------|------|
| Phase 0 | 项目脚手架 + 基础设施 | 无 |
| Phase 1 | 后端基础 (HTTP + WS + Config + DB) | Phase 0 |
| Phase 2 | WebSocket 协议 (seq 回放 + 心跳 + 多标签页) | Phase 1 |
| Phase 3 | Eino Agent + Tools | Phase 2 |
| Phase 4 | 记忆系统 | Phase 3 |
| Phase 5 | Chrome Extension | Phase 2 |
| Phase 6 | 集成测试 + 联调 | Phase 4 + 5 |

---

## Phase 0: 项目脚手架 + 基础设施

### 任务

1. 初始化 Go module: `github.com/PineappleBond/MemBrowser/backend`

2. 创建目录结构:

```
backend/
├── cmd/server/main.go           # 入口 (手动组装依赖，不用 uber/fx)
├── internal/
│   ├── config/config.go         # 配置 (viper/env)
│   ├── db/db.go                 # SQLite3 + GORM (WAL 模式)
│   ├── handler/                 # HTTP handlers
│   ├── model/                   # GORM models
│   ├── service/                 # 业务逻辑
│   ├── ws/                      # WebSocket 服务端
│   ├── eino/                    # Agent + Tools
│   │   ├── runner/
│   │   ├── tools/
│   │   └── prompts/
│   └── types/                   # 请求/响应类型
├── configs/
│   ├── config.yaml              # 默认配置模板
│   └── .env.example             # 环境变量示例
├── go.mod
└── Makefile

extension/
├── manifest.json
├── src/
│   ├── background/              # Service Worker (标签页事件监听)
│   ├── sidepanel/               # Side Panel UI (WS 客户端在这里)
│   ├── content/                 # Content Script (DOM 采集 + 指令执行)
│   └── shared/                  # 共享类型
├── package.json
└── tsconfig.json
```

3. 安装核心依赖:
   - `gorm.io/gorm` + `gorm.io/driver/sqlite` (modernc.org/sqlite, 无 CGO)
   - `github.com/gin-gonic/gin` + `github.com/gin-contrib/cors`
   - `nhooyr.io/websocket` (不用 gorilla/websocket，已归档)
   - `github.com/spf13/viper`
   - `github.com/google/uuid`
   - Eino 相关包 (参考 eino-demo-dev 的 downloads/ 目录)
   - 测试: `github.com/stretchr/testify`

4. 配置模板:

```yaml
# configs/config.yaml
port: 8080
db_path: ./data/membrowser.db
auth_key: change-me

# 分级模型 (OpenAI 兼容)
haiku:
  base_url: https://api.openai.com/v1
  api_key: ${MEMBROWSER_HAIKU_API_KEY}
  model: gpt-4o-mini

sonnet:
  base_url: https://api.openai.com/v1
  api_key: ${MEMBROWSER_SONNET_API_KEY}
  model: gpt-4o

opus:
  base_url: https://api.openai.com/v1
  api_key: ${MEMBROWSER_OPUS_API_KEY}
  model: o1
```

```bash
# configs/.env.example
MEMBROWSER_PORT=8080
MEMBROWSER_DB_PATH=./data/membrowser.db
MEMBROWSER_AUTH_KEY=change-me
MEMBROWSER_HAIKU_BASE_URL=https://api.openai.com/v1
MEMBROWSER_HAIKU_API_KEY=sk-xxx
MEMBROWSER_HAIKU_MODEL=gpt-4o-mini
MEMBROWSER_SONNET_BASE_URL=https://api.openai.com/v1
MEMBROWSER_SONNET_API_KEY=sk-xxx
MEMBROWSER_SONNET_MODEL=gpt-4o
```

5. 编写 Makefile (build, run, test)

### 验证

- `go build ./cmd/server/` 编译通过
- `make run` 启动后 `GET /health` 返回 200

---

## Phase 1: 后端基础

### 1.1 配置系统

用 viper + env 实现，环境变量前缀 `MEMBROWSER_`:

```go
// backend/internal/config/config.go
type Config struct {
    Port     int         `mapstructure:"PORT"`
    DBPath   string      `mapstructure:"DB_PATH"`
    AuthKey  string      `mapstructure:"AUTH_KEY"`
    Haiku    ModelConfig `mapstructure:"HAIKU"`
    Sonnet   ModelConfig `mapstructure:"SONNET"`
    Opus     ModelConfig `mapstructure:"OPUS"`
}

type ModelConfig struct {
    BaseURL string `mapstructure:"BASE_URL"`
    APIKey  string `mapstructure:"API_KEY"`
    Model   string `mapstructure:"MODEL"`
}
```

### 1.2 数据库 (SQLite3 + GORM)

```go
// backend/internal/db/db.go
func NewDB(cfg *config.Config) (*gorm.DB, error) {
    db, err := gorm.Open(sqlite.Open(cfg.DBPath+"?_journal_mode=WAL&_busy_timeout=5000"), &gorm.Config{})
    db.AutoMigrate(
        &model.Memory{},
        &model.Session{},
        &model.Tab{},
        &model.Update{},
    )
    return db, nil
}
```

GORM Models:

```go
// backend/internal/model/session.go
type Session struct {
    ID          string    `gorm:"primaryKey"`
    ActiveTabID int       // 当前活跃标签页
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

type Tab struct {
    ID        string    `gorm:"primaryKey"` // UUID
    SessionID string    `gorm:"index"`
    ChromeTabID int     // Chrome 标签页 ID
    URL       string
    Title     string
    Active    bool
    CreatedAt time.Time
}

// backend/internal/model/update.go
type Update struct {
    ID        string         `gorm:"primaryKey"`
    Seq       int64          `gorm:"uniqueIndex"`
    Type      string
    Payload   model.JSONMap
    CreatedAt time.Time
}

// backend/internal/model/memory.go
type Memory struct {
    ID             string    `gorm:"primaryKey"`
    SessionID      string    `gorm:"index"`
    PageURL        string
    PageURLPattern string    `gorm:"index"`
    PageTitle      string
    PageFeatures   string
    DOMSnapshotPath string   // 存文件路径，不存 BLOB
    ScreenshotPath  string   // 存文件路径，不存 BLOB
    ActionType     string
    ActionTarget   string
    ActionSelector string
    ActionValue    string
    Result         string
    FailReason     string
    Source         string
    CreatedAt      time.Time
}
```

**关键**: DOMSnapshot 和 Screenshot 存文件系统，数据库只存路径。

### 1.3 Seq 机制 (无 Redis，统一 atomic)

```go
// backend/internal/ws/seq.go
type SeqManager struct {
    db     *gorm.DB
    maxSeq int64  // 全部用 atomic 操作
}

func (s *SeqManager) Init() error {
    // 启动时从 SQLite 恢复
    var maxSeq int64
    s.db.Model(&model.Update{}).Select("COALESCE(MAX(seq), 0)").Scan(&maxSeq)
    atomic.StoreInt64(&s.maxSeq, maxSeq)
    return nil
}

func (s *SeqManager) Next() int64 {
    return atomic.AddInt64(&s.maxSeq, 1)
}

func (s *SeqManager) Max() int64 {
    return atomic.LoadInt64(&s.maxSeq)
}
```

### 1.4 HTTP 路由 + CORS

```go
// backend/internal/handler/router.go
func NewRouter(h *Handler) *gin.Engine {
    r := gin.Default()
    r.Use(cors.New(cors.Config{
        AllowOrigins:     []string{"*"}, // Extension 跨域
        AllowMethods:     []string{"GET", "POST", "PUT", "DELETE"},
        AllowHeaders:     []string{"Authorization", "Content-Type"},
    }))

    r.GET("/health", h.Health)

    v1 := r.Group("/api/v1")
    {
        // 任务管理
        v1.POST("/tasks", h.StartTask)           // 启动任务
        v1.POST("/tasks/:id/stop", h.StopTask)    // 停止任务

        // Extension 上报
        v1.POST("/page/state", h.UploadPageState)
        v1.POST("/action/result", h.UploadActionResult)
        v1.POST("/teach", h.UploadTeachData)

        // 标签页事件
        v1.POST("/tabs/opened", h.TabOpened)
        v1.POST("/tabs/closed", h.TabClosed)
        v1.POST("/tabs/activated", h.TabActivated)

        // 轮询降级
        v1.GET("/updates", h.PollUpdates)        // ?last_seq=N&limit=500
    }

    return r
}
```

### 1.5 依赖组装 (手动，不用 uber/fx)

```go
// backend/cmd/server/main.go
func main() {
    cfg := config.Load()
    db := db.NewDB(cfg)
    seqMgr := ws.NewSeqManager(db)
    seqMgr.Init()

    memStore := service.NewMemoryStore(db)
    tabMgr := service.NewTabManager(db)
    waiter := ws.NewResponseWaiter()
    wsMgr := ws.NewManager(seqMgr, waiter)
    modelProvider := eino.NewModelProvider(cfg)
    toolRegistry := eino.NewToolsRegistry(wsMgr, memStore, tabMgr, waiter)
    taskService := service.NewTaskService(modelProvider, toolRegistry, wsMgr, seqMgr)
    handler := handler.NewHandler(taskService, wsMgr, tabMgr, waiter)

    router := handler.NewRouter(handler)
    router.Run(fmt.Sprintf(":%d", cfg.Port))
}
```

### 验证

- 配置从环境变量正确加载
- SQLite WAL 模式启用
- `POST /api/v1/tasks` 接收任务返回 200 + task_id
- `GET /api/v1/updates?last_seq=0&limit=500` 返回空数组
- CORS 头正确

---

## Phase 2: WebSocket 协议

### 2.1 WS 协议定义

```go
// backend/internal/ws/protocol.go
const (
    FrameConnected   = "connected"
    FramePing        = "ping"
    FramePong        = "pong"
    FrameUpdate      = "update"
    FrameUpdateBatch = "update_batch"
    FrameQuery       = "page.query"      // 独立 frame，不走 Update
    FrameExecute     = "action.execute"   // 独立 frame，不走 Update
    FrameNeedHelp    = "need_help"        // 独立 frame，不走 Update
)

type Frame struct {
    Type    string          `json:"type"`
    ID      string          `json:"id"`       // request_id，用于请求-响应配对
    Payload json.RawMessage `json:"payload"`
}

type Update struct {
    Seq     int64           `json:"seq"`     // >0 持久化, =0 临时
    Type    string          `json:"type"`
    ID      string          `json:"id"`
    Payload json.RawMessage `json:"payload"`
}
```

**关键**: `page.query` / `action.execute` / `need_help` 是独立 frame，不走 Update 通道 (不持久化)。`task.started` / `task.step` / `task.completed` 等走 Update 通道 (持久化，可回放)。

### 2.2 WS 连接管理

WS 客户端放在 Side Panel 页面 (非 Service Worker)，因为 MV3 Service Worker 会被 Chrome 休眠:

```
Side Panel (WS 客户端) ←→ 后端 WS 服务端
Background SW (标签页事件监听，通过 Side Panel 转发)
Content Script (DOM 采集 + 指令执行，通过 Background SW 转发)
```

```go
// backend/internal/ws/manager.go
type Manager struct {
    conn      *websocket.Conn  // coder/websocket
    mu        sync.RWMutex
    seqMgr    *SeqManager
    waiter    *ResponseWaiter
    sendCh    chan *Frame
    onMessage func(Frame)      // 回调注入，避免循环依赖
}

func (m *Manager) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // 1. 验证 auth key
    // 2. Accept WebSocket upgrade (coder/websocket)
    // 3. 发送 connected frame { max_seq, session_id }
    // 4. 如果 last_seq < max_seq，回放 (limit 500)
    // 5. 启动 readLoop + writeLoop
}

func (m *Manager) PushFrame(f *Frame)           // 推送独立 frame
func (m *Manager) PushUpdate(u Update)           // 推送 update (持久化 + 推送)
func (m *Manager) SetOnMessage(fn func(Frame))   // 注入回调
```

### 2.3 心跳 (统一: 后端发 ping)

参考 eino-demo，但方向改为后端发 ping (因为 MV3 Service Worker 不可靠):

- 后端每 30s 发 ping frame
- Side Panel 收到后回 pong
- 60s 未收到 pong → 断开，等待重连

### 2.4 重连回放

- Side Panel 连接时带 `?last_seq=N`
- 后端查 `updates WHERE seq > N ORDER BY seq ASC LIMIT 500`
- 通过 `update_batch` frame 推送

### 2.5 HTTP 轮询降级

```go
// GET /api/v1/updates?last_seq=N&limit=500
func (h *Handler) PollUpdates(c *gin.Context) {
    lastSeq := c.QueryInt64("last_seq")
    limit := c.DefaultQueryInt("limit", 500)
    if limit > 1000 { limit = 1000 }
    updates := h.updateService.GetSince(lastSeq, limit)
    c.JSON(200, gin.H{"updates": updates, "max_seq": h.seqMgr.Max()})
}
```

### 2.6 多标签页管理

```go
// backend/internal/service/tab.go
type TabManager struct {
    db      *gorm.DB
    session *model.Session
    mu      sync.RWMutex
    agentLocked bool  // Agent 执行期间锁定，忽略用户手动切换
}

func (m *TabManager) OnTabOpened(tabID int, url, title string) {
    // 创建 Tab 记录，自动切换 active_tab_id
}

func (m *TabManager) OnTabClosed(chromeTabID int) {
    // 关闭的是活跃标签页 → 切换到最近使用的
    // 所有标签页都关闭 → 暂停 Agent，推送通知
}

func (m *TabManager) OnTabActivated(chromeTabID int) {
    // 只在非 agentLocked 状态下更新 active_tab_id
}

func (m *TabManager) LockAgent()   { m.agentLocked = true }
func (m *TabManager) UnlockAgent() { m.agentLocked = false }
```

**关键语义**:
- Agent 执行操作触发新标签页 → 自动切换 (因为 agentLocked=true)
- 用户手动切换标签页 → 不更新 active_tab_id (因为 agentLocked=true)
- Agent 任务结束后 → agentLocked=false，用户手动切换生效

### 2.7 ResponseWaiter (修复内存泄漏)

```go
// backend/internal/ws/waiter.go
type ResponseWaiter struct {
    pending map[string]chan json.RawMessage
    mu      sync.RWMutex
}

func (w *ResponseWaiter) Wait(id string, timeout time.Duration) (json.RawMessage, error) {
    ch := make(chan json.RawMessage, 1)
    w.mu.Lock()
    w.pending[id] = ch
    w.mu.Unlock()

    defer func() {
        w.mu.Lock()
        delete(w.pending, id)  // 超时或正常完成都清理
        w.mu.Unlock()
    }()

    select {
    case resp := <-ch:
        return resp, nil
    case <-time.After(timeout):
        return nil, errors.New("timeout")
    }
}

func (w *ResponseWaiter) Resolve(id string, data json.RawMessage) {
    w.mu.RLock()
    ch, ok := w.pending[id]
    w.mu.RUnlock()
    if ok {
        select {
        case ch <- data:
        default: // channel 已满 (超时已触发)，忽略
        }
    }
}

func (w *ResponseWaiter) Clear() {
    // Agent 任务取消时，清理所有 pending
    w.mu.Lock()
    for id, ch := range w.pending {
        close(ch)
        delete(w.pending, id)
    }
    w.mu.Unlock()
}
```

### 验证

- WS 连接后收到 `connected` frame
- 心跳: 后端 30s 发 ping，Side Panel 回 pong
- 断连重连后 `last_seq` 之后的 update 被回放 (limit 500)
- HTTP 轮询返回相同数据，支持 limit 参数
- 多标签页: 新标签页自动接管，用户手动切换不干扰 Agent
- ResponseWaiter 超时后 map 被清理

---

## Phase 3: Eino Agent + Tools

### 3.1 模型提供者

```go
// backend/internal/eino/model.go
type ModelProvider struct {
    cfg config.Config
}

func (m *ModelProvider) GetModel(tier string) (*openai.ChatModel, error) {
    var mc config.ModelConfig
    switch tier {
    case "haiku": mc = m.cfg.Haiku
    case "sonnet": mc = m.cfg.Sonnet
    case "opus": mc = m.cfg.Opus
    default: mc = m.cfg.Sonnet
    }
    return openai.NewChatModel(context.Background(), &openai.ChatModelConfig{
        BaseURL: mc.BaseURL,
        APIKey:  mc.APIKey,
        Model:   mc.Model,
    })
}
```

### 3.2 Agent 构建 (按 eino-demo 实际 API)

```go
// backend/internal/eino/runner/runner.go
func NewRunner(cfg RunnerConfig) (*Runner, error) {
    chatModel, err := cfg.ModelProvider.GetModel(cfg.Tier)

    deepAgent, err := deep.New(ctx, &deep.Config{
        Name:        "membrowser",
        Description: "Web 自动化 Agent",
        ChatModel:   chatModel,
        Instruction: cfg.SystemPrompt,
        ToolsConfig: adk.ToolsConfig{
            ToolsNodeConfig: compose.ToolsNodeConfig{
                Tools: cfg.Tools,
            },
        },
        MaxIteration: 20,
    })

    runner := adk.NewRunner(ctx, adk.RunnerConfig{
        Agent:            deepAgent,
        EnableStreaming:  true,
        CheckPointStore:  cfg.CheckPointStore,
    })

    return &Runner{runner: runner}, nil
}
```

### 3.3 Tools 定义 (回调注入，避免循环依赖)

```go
// backend/internal/eino/tools/registry.go
type Registry struct {
    tools   []tool.BaseTool
    pushFn  func(Frame)              // 回调注入
    waiter  *ResponseWaiter
    memStore *service.MemoryStore
    tabMgr   *service.TabManager
}

func NewRegistry(pushFn func(Frame), waiter *ResponseWaiter, memStore *service.MemoryStore, tabMgr *service.TabManager) *Registry {
    r := &Registry{pushFn: pushFn, waiter: waiter, memStore: memStore, tabMgr: tabMgr}
    r.buildTools()
    return r
}
```

**同步 Tool (get_page_state, execute_action)**: 用 ResponseWaiter，request_id 配对:

```go
// backend/internal/eino/tools/get_page_state.go
func (t *GetPageStateTool) Run(ctx context.Context, input GetPageStateInput) (GetPageStateOutput, error) {
    requestID := uuid.New().String()
    // 通过回调推送 frame (不是 Update)
    t.pushFn(Frame{Type: "page.query", ID: requestID, Payload: ...})
    // 等待 Extension 通过 HTTP 回调
    resp, err := t.waiter.Wait(requestID, 30*time.Second)
    // 解析响应
}
```

**人类示教 Tool (ask_human)**: 必须用 Eino 的 `tool.Interrupt`:

```go
// backend/internal/eino/tools/ask_human.go
func (t *AskHumanTool) Run(ctx context.Context, input AskHumanInput) (AskHumanOutput, error) {
    wasInterrupted, _, _ := tool.GetInterruptState[any](ctx)
    if wasInterrupted {
        isTarget, hasData, data := tool.GetResumeContext[string](ctx)
        if isTarget && hasData {
            return AskHumanOutput{Result: data}, nil
        }
        return AskHumanOutput{}, tool.Interrupt(ctx, map[string]any{
            "message": input.Message,
            "highlight_selector": input.HighlightSelector,
        })
    }
    return AskHumanOutput{}, tool.Interrupt(ctx, map[string]any{
        "message": input.Message,
        "highlight_selector": input.HighlightSelector,
    })
}
```

**完整 Tool 列表:**

| Tool | 类型 | 等待机制 |
|------|------|---------|
| get_page_state | 同步 | ResponseWaiter (30s) |
| execute_action | 同步 | ResponseWaiter (30s) |
| search_memory | 本地 | 直接返回 |
| save_memory | 本地 | 直接返回 |
| ask_human | 中断 | tool.Interrupt + checkpoint |
| list_tabs | 本地 | 直接返回 |
| switch_tab | 本地 | 直接返回 |
| open_tab | 同步 | ResponseWaiter (30s) |

**注意**: `call_ai_model` 不需要注册为 Tool — Eino DeepAgent 内置模型调用能力。

### 3.4 Agent 事件流 + thinking 推送

```go
// backend/internal/service/task.go
func (s *TaskService) runAgent(taskID string, taskDesc string) {
    iter := s.runner.Run(ctx, messages)
    for {
        event, ok := iter.Next()
        if !ok { break }
        switch event.Type {
        case adk.EventThinking:
            s.wsMgr.PushFrame(&Frame{Type: "thinking", Payload: event.Delta})
        case adk.EventOutputting:
            s.wsMgr.PushUpdate(Update{Type: "task.step", Payload: ...})
        case adk.EventCompleted:
            s.wsMgr.PushUpdate(Update{Type: "task.completed", ...})
        case adk.EventError:
            s.wsMgr.PushUpdate(Update{Type: "task.failed", ...})
        }
    }
}
```

### 3.5 并发控制

```go
// backend/internal/service/task.go
type TaskService struct {
    running atomic.Bool  // 同时只能有一个 Agent
}

func (s *TaskService) StartTask(taskDesc string) (string, error) {
    if !s.running.CompareAndSwap(false, true) {
        return "", errors.New("已有任务在运行")  // HTTP 409
    }
    // ... 启动 Agent
}
```

### 3.6 Agent 超时 (可配置)

```go
const DefaultAgentTimeout = 20 * time.Minute  // 默认 20 分钟

func (s *TaskService) runAgentWithTimeout(taskID string, taskDesc string) {
    ctx, cancel := context.WithTimeout(context.Background(), DefaultAgentTimeout)
    defer cancel()
    s.runAgent(ctx, taskID, taskDesc)
}
```

### 3.7 System Prompt

```
你是 MemBrowser，一个 Web 自动化助手。帮用户自动执行 Web 操作。

## 工作流程
1. 用 get_page_state() 观察当前页面
2. 用 search_memory() 查找已知操作路径
3. 有记忆命中 → 用 execute_action() 直接执行
4. 无记忆 → AI 分析 DOM 决定下一步
5. AI 无法确定 → 用 ask_human() 请求帮助
6. 操作成功后 → 用 save_memory() 保存经验

## 规则
- 每次只执行一个操作，然后观察结果
- 不预规划多个步骤
- 失败时记录原因，尝试其他方案
- 连续失败 3 次后必须 ask_human()
- 需要操作特定标签页时，用 list_tabs() 查看，用 switch_tab() 切换
```

### 验证

- Agent 能接收任务并开始执行
- get_page_state() 通过 WS 发送 page.query，Extension 通过 HTTP 回调
- execute_action() 通过 WS 发送 action.execute
- ask_human() 用 tool.Interrupt 暂停，checkpoint 保存状态
- thinking 消息实时推送给 Extension
- 同时提交两个任务，第二个返回 409
- Agent 超时后推送 task.failed

---

## Phase 4: 记忆系统

### 4.1 记忆存储

```go
// backend/internal/service/memory.go
type MemoryStore struct {
    db     *gorm.DB
    dataDir string  // 截图和 DOM 快照存储目录
}

func (s *MemoryStore) Save(m *model.Memory) error
func (s *MemoryStore) Search(pageURL, actionType, actionTarget string) (*model.Memory, error)
func (s *MemoryStore) SaveScreenshot(data []byte) (string, error)  // 存文件，返回路径
func (s *MemoryStore) SaveDOMSnapshot(data string) (string, error)  // 存文件，返回路径
```

### 4.2 URL 标准化

```go
// backend/internal/service/url_normalize.go
func NormalizeURL(rawURL string) string {
    // /product/12345 → /product/{id}
    // /user/abc-def/profile → /user/{id}/profile
}
```

### 4.3 记忆匹配 (用 INSTR 代替 LIKE)

```go
func (s *MemoryStore) Search(pageURLPattern, actionType, actionTarget string) (*model.Memory, error) {
    var m model.Memory
    err := s.db.Where(
        "page_url_pattern = ? AND action_type = ? AND INSTR(action_target, ?) > 0 AND result = ?",
        pageURLPattern, actionType, actionTarget, "success",
    ).Order("created_at DESC").First(&m).Error
    return &m, err
}
```

用 `INSTR()` 代替 `LIKE '%..%'`，避免通配符注入风险。

### 4.4 人类示教流程

```go
// backend/internal/handler/teach.go
func (h *Handler) UploadTeach(c *gin.Context) {
    // 1. 接收示教数据 (element, action, screenshot, session_id)
    // 2. 保存截图到文件系统
    // 3. 构造 Memory 记录 (source=human, result=success)
    // 4. 存入 SQLite
    // 5. 推送 update
    // 6. 调用 waiter.Resolve() 通知 Agent 继续
}
```

### 验证

- 人类示教后记忆正确存入 SQLite + 文件系统
- 相同页面 + 相同操作，search_memory 命中
- 记忆命中后 Agent 直接复用 (0 Token)
- URL 标准化正确

---

## Phase 5: Chrome Extension

### 5.1 Manifest V3

```json
{
  "manifest_version": 3,
  "name": "MemBrowser",
  "permissions": ["sidePanel", "activeTab", "scripting", "tabs"],
  "host_permissions": ["http://localhost:*/*"],
  "side_panel": {
    "default_path": "sidepanel.html",
    "openPanelOnActionClick": true
  },
  "background": { "service_worker": "background.js" },
  "content_scripts": [{
    "matches": ["<all_urls>"],
    "js": ["content.js"],
    "run_at": "document_idle"
  }]
}
```

**权限说明**:
- `tabs`: chrome.tabs.onCreated/onRemoved/onActivated 需要
- `host_permissions`: 访问后端 localhost API
- `sidePanel`: Side Panel API

### 5.2 消息路由层

```
后端 WS → Side Panel (WS 客户端)
                ↓ chrome.runtime.sendMessage
          Background Service Worker
                ↓ chrome.tabs.sendMessage(tabId)
          Content Script (目标标签页)
```

```typescript
// extension/src/sidepanel/ws-client.ts
// WS 客户端在 Side Panel 中 (MV3 Service Worker 会被休眠)
class WSClient {
  private ws: WebSocket;
  private lastSeq: number = 0;

  connect(url: string, authKey: string) {
    this.ws = new WebSocket(`${url}?token=${authKey}&last_seq=${this.lastSeq}`);
    this.ws.onmessage = (e) => this.handleFrame(JSON.parse(e.data));
  }

  private handleFrame(frame: Frame) {
    if (frame.type === 'connected') {
      this.lastSeq = frame.payload.max_seq;
      return;
    }
    if (frame.type === 'ping') {
      this.send({ type: 'pong' });
      return;
    }
    if (frame.type === 'update_batch') {
      frame.payload.forEach(u => this.handleUpdate(u));
      return;
    }
    // 独立 frame: page.query / action.execute / need_help
    // 根据 payload.tab_id 路由到对应标签页
    this.routeToTab(frame);
  }

  private routeToTab(frame: Frame) {
    const tabId = frame.payload.tab_id;
    chrome.runtime.sendMessage({
      type: 'forward_to_tab',
      tabId: tabId,
      frame: frame
    });
  }
}
```

```typescript
// extension/src/background/tab-router.ts
// Background SW: 标签页事件监听 + 消息路由
chrome.tabs.onCreated.addListener((tab) => {
  // 通知 Side Panel (如果打开)
  chrome.runtime.sendMessage({
    type: 'tab_event',
    event: 'opened',
    payload: { tab_id: tab.id, url: tab.url, title: tab.title }
  });
});

chrome.tabs.onRemoved.addListener((tabId) => {
  chrome.runtime.sendMessage({
    type: 'tab_event',
    event: 'closed',
    payload: { tab_id: tabId }
  });
});

chrome.tabs.onActivated.addListener((activeInfo) => {
  chrome.runtime.sendMessage({
    type: 'tab_event',
    event: 'activated',
    payload: { tab_id: activeInfo.tabId }
  });
});

// 路由: Side Panel → Content Script
chrome.runtime.onMessage.addListener((msg, sender, sendResponse) => {
  if (msg.type === 'forward_to_tab') {
    chrome.tabs.sendMessage(msg.tabId, msg.frame).then(sendResponse);
    return true; // 异步响应
  }
});
```

### 5.3 Content Script — DOM 采集

```typescript
// extension/src/content/dom-collector.ts
interface DOMNode {
  tag: string;
  chromeTabId: string;   // 唯一标识 (用于选择器)
  text: string;
  attributes: Record<string, string>;
  interactable: boolean;
  children: DOMNode[];
}

function collectDOM(): DOMNode {
  // 限制最多 500 个可交互元素
  // 只采集 input, button, a, select, textarea
  // 保留 placeholder, aria-label, text content
  // 生成唯一标识用于后续定位
}
```

### 5.4 Content Script — 指令执行

```typescript
// extension/src/content/action-executor.ts
function executeAction(action: {
  request_id: string;
  type: 'click' | 'input' | 'scroll' | 'navigate';
  selector: string;
  value?: string;
}): void {
  const el = document.querySelector(action.selector);
  if (!el) {
    reportResult(action.request_id, false, 'element not found');
    return;
  }
  // 执行操作
  // 通过 HTTP 上报结果到后端 (带 request_id)
  reportResult(action.request_id, true);
}
```

### 5.5 Content Script — 人类示教 UI

```typescript
// extension/src/content/teaching-ui.ts
function showHighlight(selector: string, message: string, timeout: number = 300000) {
  // 1. 高亮目标元素 (amber border + pulse)
  // 2. 显示提示浮层
  // 3. 等待用户点击 (超时 5 分钟)
  // 4. 捕获操作，通过 HTTP 上报到后端
  // 5. 超时则上报 timeout 错误
}
```

### 5.6 Side Panel UI

参考 `docs/prototype/extension-mockup.html`，用 TypeScript 实现:
- 模型选择 (Haiku / Sonnet / Opus)
- 任务输入框 + 提交按钮
- 执行进度列表 (含 thinking 流式显示)
- 人类示教提示
- 连接状态
- 设置 (折叠式): WebSocket URL + Auth Key
- 任务运行中: 提交按钮灰掉，显示停止按钮

### 验证

- Side Panel 打开后 WS 连接成功
- 后端发 page.query，Side Panel 路由到对应标签页的 Content Script
- Content Script 返回 DOM 数据，通过 HTTP 上报
- 多标签页: 新标签页自动接管
- 人类示教: 高亮 → 用户点击 → 超时 5 分钟
- Side Panel 关闭后 WS 断开，重连后 seq 回放恢复状态

---

## Phase 6: 集成测试 + 联调

### 6.1 测试框架

```go
// 测试依赖
// github.com/stretchr/testify
// net/http/httptest
```

### 6.2 单元测试 (L1)

| 测试对象 | 测试用例 |
|---------|---------|
| SeqManager | 并发安全、Init 恢复、Max 一致性 |
| ResponseWaiter | 超时清理、正常返回、并发调用、Clear |
| URL Normalize | 纯数字、UUID、多级路径、无变化 |
| MemoryStore.Search | 精确匹配、模糊匹配、无结果、INSTR 转义 |
| TabManager | 创建、关闭、活跃切换、Agent 锁定、全部关闭 |
| Content Script DOM | 空页面、复杂页面、iframe、动态加载 |
| Content Script Action | click/input/scroll/navigate、元素不存在 |

### 6.3 集成测试 (L2)

| 场景 | 验证点 |
|------|--------|
| 完整任务流程 | 任务 → Agent → WS → Extension → HTTP 回调 → 完成 |
| 记忆复现 | 第一次 AI 推理，第二次记忆命中 (0 Token) |
| 人类示教 | Agent 失败 → ask_human → tool.Interrupt → 用户操作 → 恢复 |
| 多标签页 | 新标签页打开 → 自动切换 → 继续执行 |
| WS 断连重连 | 断连 → 重连 → seq 回放 → 状态恢复 |
| 并发任务拒绝 | 第一个任务运行中 → 第二个返回 409 |
| Agent 超时 | 20 分钟超时 → task.failed 推送 |

### 6.4 首次用户体验测试 (L3)

完整流程:
1. 下载后端可执行文件
2. 复制 `.env.example` 为 `.env`，填入 API Key
3. `make run` 启动后端
4. Chrome 加载未打包 Extension (chrome://extensions 开发者模式)
5. 配置 Side Panel 连接后端 (localhost:8080)
6. 输入第一个任务
7. 看到 Agent 执行 + thinking 推送
8. 任务完成，显示执行统计

### 6.5 异常测试 (L3)

| 场景 | 验证点 |
|------|--------|
| 用户关闭活跃标签页 | 自动切换到最近标签页 |
| 所有标签页关闭 | Agent 暂停，通知用户 |
| AI API 限流 | 重试或降级 |
| DOM 采集返回空 | Agent 报错并尝试其他方案 |
| iframe 内操作 | Content Script 能访问 iframe |

### 验证

- 所有 L1 单元测试通过
- 所有 L2 集成测试通过
- L3 首次用户体验流程跑通
- L3 异常测试覆盖

---

## 关键技术决策

### 1. 不用 uber/fx，手动组装

V0.1 单用户桌面应用，复杂度不需要 fx。`main()` 中手动 `New()` 即可。

### 2. WS 客户端在 Side Panel

MV3 Service Worker 会被 Chrome 休眠，WS 客户端放在 Side Panel 页面更可靠。Side Panel 关闭时降级为 HTTP 轮询。

### 3. ask_human 用 tool.Interrupt

Eino 原生的 checkpoint/resume 机制，比自建 ResponseWaiter 更可靠。Agent 暂停期间上下文持久化到 SQLite。

### 4. 多标签页 Agent 锁定

Agent 执行期间锁定 active_tab_id，用户手动切换不干扰。只有 Agent 操作触发的新标签页才自动切换。

### 5. DOMSnapshot/Screenshot 存文件

SQLite 只存路径，大文件存文件系统。限制 DOM 采集最多 500 个可交互元素。

### 6. INSTR 代替 LIKE

记忆匹配用 `INSTR()` 代替 `LIKE '%..%'`，避免通配符注入。

### 7. coder/websocket

不用已归档的 gorilla/websocket，与 eino-demo 保持一致。

### 8. 心跳方向: 后端发 ping

统一为后端每 30s 发 ping，Side Panel 回 pong。适配 MV3 Service Worker 不可靠的场景。
