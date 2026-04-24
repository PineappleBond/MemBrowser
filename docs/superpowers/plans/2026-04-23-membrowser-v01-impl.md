# MemBrowser V0.1 实现计划

> **日期**: 2026-04-23
> **设计文档**: `docs/plans/2026-04-23-membrowser-v01-design.md`
> **参考项目**: `/Users/leichujun/go/src/github.com/PineappleBond/eino-demo-dev/`
> **WebSocket 协议参考**: `/Users/leichujun/go/src/github.com/PineappleBond/eino-demo-dev/docs/API.md`

---

## 总览

| 阶段 | 内容 | 依赖 |
|------|------|------|
| Phase 0 | 项目脚手架 + 基础设施 | 无 |
| Phase 1 | 后端基础 (HTTP + WS + Config + DB + 错误处理 + 日志) | Phase 0 |
| Phase 2 | WebSocket 协议 (seq 回放 + gap filling + 心跳 + 多标签页) | Phase 1 |
| Phase 3 | Eino Agent + Callback 体系 + Tools | Phase 2 |
| Phase 4 | 记忆系统 | Phase 3 |
| Phase 5 | Chrome Extension (含 applyUpdates) | Phase 2 |
| Phase 6 | 集成测试 + 联调 | Phase 4 + 5 |

### 提交规范

所有提交遵循 CLAUDE.md 第 5 节: `<type>(<scope>): <subject>`，简体中文祈使句。

### 自循环验证

每个 Phase 完成后执行 CLAUDE.md 第 7 节自循环验证: 子代理 Review (≥2 角度) → L1 单元测试 → L2 集成测试 → L3 E2E。

### 与 CLAUDE.md 的已知偏差

| 偏差项 | CLAUDE.md/API.md 要求 | 本计划选择 | 原因 |
|--------|----------------------|-----------|------|
| 依赖注入 | uber/fx via `internal/di/container.go` | 手动组装 main.go | V0.1 单用户桌面应用，复杂度不需要 fx |
| ErrorCode | 14 个值与 OpenMAIC 完全一致 | 14 个，参考命名风格，扩展 MemBrowser 特有码，命名不与 OpenMAIC 一一对应 | MemBrowser 不与 OpenMAIC 互操作 |
| import path | `github.com/PineappleBond/classroom-backend/` | `github.com/PineappleBond/MemBrowser/backend/` | CLAUDE.md 为模板残留，以实际项目为准 |
| env 前缀 | `CLASSROOM_` | `MEMBROWSER_` | 同上 |
| PollUpdates last_seq=0 | API.md: 只返回最后一条 | 返回所有更新 (受 limit 限制) | 桌面应用首次连接需要更多历史 |
| WS 连接管理 | API.md: 多连接按 user_id 管理 | V0.1 单用户，固定 key `"main"` 管理唯一连接 | 单用户桌面应用无需多连接 |
| CORS | gin-cors `AllowOrigins` 精确匹配 | `AllowOriginFunc` 前缀匹配 | Extension origin 含随机 ID，无法精确列出 |
| 心跳方向 | API.md Flow 3: 服务端发 ping，客户端回 pong | 遵循 API.md | 无偏差 |
| Update.ID 字段 | API.md: Update 无 id 字段 | 增加 `id` 字段 (request_id 配对) | MemBrowser 扩展，用于请求-响应配对 |
| response 包 | CLAUDE.md: Response 在 pkg/errors 中 | 独立 pkg/response 包 | 避免循环依赖 |

---

## Phase 0: 项目脚手架 + 基础设施

### 任务

1. 初始化 Go module: `github.com/PineappleBond/MemBrowser/backend`

2. 创建目录结构:

```
backend/
├── cmd/server/main.go           # 入口 (手动组装依赖)
├── internal/
│   ├── config/config.go         # 配置 (viper/env)
│   ├── db/db.go                 # SQLite3 + GORM (WAL 模式)
│   ├── handler/                 # HTTP handlers
│   ├── model/                   # GORM models + JSONMap 类型
│   ├── service/                 # 业务逻辑
│   ├── ws/                      # WebSocket 服务端
│   ├── eino/                    # Agent + Callback + Tools
│   │   ├── runner/
│   │   │   ├── runner.go        # DeepAgent 构建
│   │   │   ├── handler.go       # RootRunnerHandler (callbacks.Handler)
│   │   │   ├── callbacks.go     # RootRunnerCallbacks (DB + WS)
│   │   │   ├── run_session.go   # RunSessionManager
│   │   │   ├── message_queue.go # MessageQueue
│   │   │   └── context_injection.go # ContextInjectionMiddleware
│   │   ├── tools/
│   │   │   ├── registry.go      # Tool 注册 (utils.InferTool)
│   │   │   ├── get_page_state.go
│   │   │   ├── execute_action.go
│   │   │   ├── search_memory.go
│   │   │   ├── save_memory.go
│   │   │   ├── ask_human.go     # tool.Interrupt 模式
│   │   │   ├── list_tabs.go
│   │   │   ├── switch_tab.go
│   │   │   └── open_tab.go
│   │   └── prompts/
│   │       └── system.go
│   ├── types/                   # 请求/响应类型
│   └── middleware/              # HTTP 中间件 (CORS、日志、错误处理)
├── pkg/
│   ├── errors/errors.go         # 统一错误码 (ErrorCode)
│   ├── response/response.go    # 统一响应信封 (OK/Fail)
│   ├── logger/logger.go         # slog 封装
│   └── token/counter.go         # Token 计数 (tiktoken-go)
├── configs/
│   ├── config.yaml              # 默认配置模板
│   └── .env.example             # 仅敏感配置 (API Key)
├── go.mod
└── Makefile

extension/
├── manifest.json
├── src/
│   ├── background/              # Service Worker (标签页事件监听)
│   ├── sidepanel/               # Side Panel UI (WS 客户端 + applyUpdates)
│   │   ├── ws-client.ts         # WS 连接 + applyUpdates 流程
│   │   ├── update-store.ts      # chrome.storage.local 持久化
│   │   └── poll-fallback.ts     # HTTP 轮询降级
│   ├── content/                 # Content Script (DOM 采集 + 指令执行)
│   └── shared/                  # 共享类型 (types.ts)
├── package.json
└── tsconfig.json
```

3. 安装核心依赖:
   - `gorm.io/gorm` + `gorm.io/driver/sqlite` (modernc.org/sqlite, 无 CGO)
   - `github.com/gin-gonic/gin` + `github.com/gin-contrib/cors`
   - `github.com/gorilla/websocket` (遵循 CLAUDE.md 指定)
   - `github.com/spf13/viper`
   - `github.com/google/uuid`
   - `github.com/sashabaranov/go-openai` (OpenAI 兼容客户端)
   - Eino 相关包 (参考 eino-demo-dev 的 downloads/ 目录):
     - `github.com/cloudwego/eino`
     - `github.com/cloudwego/eino-ext`
     - `github.com/cloudwego/eino-contrib`
   - Token 计数: `github.com/tiktoken-go/tokenizer`
   - 测试: `github.com/stretchr/testify`

4. Eino API 签名验证 (安装依赖后立即执行):
   - 确认 `deep.New(ctx, &deep.Config{})` 所有字段与 eino-demo 一致
   - 确认 `adk.NewRunner` 返回类型为 `*adk.AsyncIterator[*adk.AgentEvent]` (非 `*adk.Iterator`)
   - 确认 `utils.InferTool`、`tool.Interrupt`、`tool.GetResumeContext`、`tool.GetInterruptState` 签名存在
   - 确认 `reduction.New` 的 `ReadFileToolName=""` 不 panic
   - 如有差异，先更新 Plan 再编码

5. 配置模板:

`config.yaml` 存默认值，`.env.example` 只列敏感配置 (API Key):

```yaml
# configs/config.yaml
# api_key 不在 yaml 中写入，由环境变量 MEMBROWSER_<TIER>_API_KEY 覆盖
# auth_key 同理，由 MEMBROWSER_AUTH_KEY 覆盖
port: 8080
db_path: ./data/membrowser.db
data_dir: ./data
log_level: debug

haiku:
  base_url: https://api.openai.com/v1
  model: gpt-4o-mini

sonnet:
  base_url: https://api.openai.com/v1
  model: gpt-4o

opus:
  base_url: https://api.openai.com/v1
  model: o1
```

```bash
# configs/.env.example — 仅敏感配置
MEMBROWSER_HAIKU_API_KEY=sk-xxx
MEMBROWSER_SONNET_API_KEY=sk-xxx
MEMBROWSER_OPUS_API_KEY=sk-xxx
MEMBROWSER_AUTH_KEY=change-me
```

6. 编写 Makefile:

```makefile
.PHONY: build run test lint clean

build:
	go build -o bin/membrowser ./cmd/server/

run: build
	./bin/membrowser

test:
	go test ./... -v -count=1

lint:
	golangci-lint run ./...

clean:
	rm -rf bin/ data/
```

### 验证

- `go build ./cmd/server/` 编译通过
- `make run` 启动后 `GET /health` 返回 200
- `go test ./... -v -count=1` 全部通过 (至少有 config/db 的基础测试)
- `go vet ./...` 无警告
- Eino API 签名全部确认 (step 4)，如有差异已更新 Plan

---

## Phase 1: 后端基础

### 1.1 统一错误处理 (pkg/errors)

```go
// backend/pkg/errors/errors.go
package errors

type ErrorCode string

const (
    // 通用 (参考 OpenMAIC 命名风格)
    ErrCodeBadRequest     ErrorCode = "BAD_REQUEST"
    ErrCodeUnauthorized   ErrorCode = "UNAUTHORIZED"
    ErrCodeNotFound       ErrorCode = "NOT_FOUND"
    ErrCodeConflict       ErrorCode = "CONFLICT"
    ErrCodeTimeout        ErrorCode = "TIMEOUT"
    ErrCodeInternal       ErrorCode = "INTERNAL_ERROR"

    // MemBrowser 特有 (OpenMAIC 无对应)
    ErrCodeAgentFailed     ErrorCode = "AGENT_FAILED"
    ErrCodeWSDisconnected  ErrorCode = "WS_DISCONNECTED"
    ErrCodeDOMEmpty        ErrorCode = "DOM_EMPTY"
    ErrCodeElementNotFound ErrorCode = "ELEMENT_NOT_FOUND"
    ErrCodeTeachTimeout    ErrorCode = "TEACH_TIMEOUT"
    ErrCodeTabClosed       ErrorCode = "TAB_CLOSED"
    ErrCodeAIThrottled     ErrorCode = "AI_THROTTLED"
    ErrCodeTokenOverflow   ErrorCode = "TOKEN_OVERFLOW"
)

type AppError struct {
    Code    ErrorCode `json:"errorCode"`
    Message string    `json:"error"`
    Details string    `json:"details,omitempty"`
}

func (e *AppError) Error() string { return e.Message }
func New(code ErrorCode, msg string) *AppError { ... }
func Newf(code ErrorCode, format string, args ...any) *AppError { ... }
```

统一响应信封 (H10: 独立包，避免 errors 包循环依赖):

```go
// backend/pkg/response/response.go
package response

type Response struct {
    Success   bool            `json:"success"`
    ErrorCode errors.ErrorCode `json:"errorCode,omitempty"`
    Error     string          `json:"error,omitempty"`
    Details   string          `json:"details,omitempty"`
    Data      any             `json:"data,omitempty"`
}

func OK(data any) Response { return Response{Success: true, Data: data} }
func Fail(err *errors.AppError) Response {
    return Response{Success: false, ErrorCode: err.Code, Error: err.Message, Details: err.Details}
}
```

### 1.2 slog 日志 (pkg/logger)

```go
// backend/pkg/logger/logger.go
package logger

func New(level string) *slog.Logger {
    var lvl slog.Level
    switch level {
    case "debug": lvl = slog.LevelDebug
    case "info":  lvl = slog.LevelInfo
    case "warn":  lvl = slog.LevelWarn
    case "error": lvl = slog.LevelError
    default:      lvl = slog.LevelInfo
    }
    return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl}))
}
```

**关键**: main() 中必须调用 `slog.SetDefault(logger.New(cfg.LogLevel))`，否则 `slog.Info()` 等全局调用使用默认 logger，配置的 log level 不生效。

### 1.3 配置系统

```go
// backend/internal/config/config.go
type Config struct {
    Port     int         `mapstructure:"PORT"`
    DBPath   string      `mapstructure:"DB_PATH"`
    DataDir  string      `mapstructure:"DATA_DIR"` // 截图和 DOM 快照存储目录
    AuthKey  string      `mapstructure:"AUTH_KEY"`
    LogLevel string      `mapstructure:"LOG_LEVEL"`
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

### 1.4 数据库 (SQLite3 + GORM)

```go
// backend/internal/db/db.go
func NewDB(cfg *config.Config) (*gorm.DB, error) {
    db, err := gorm.Open(sqlite.Open(cfg.DBPath+"?_journal_mode=WAL&_busy_timeout=5000"), &gorm.Config{})
    if err != nil {
        return nil, fmt.Errorf("open db: %w", err)
    }
    if err := db.AutoMigrate(
        &model.Memory{},
        &model.Session{},
        &model.Tab{},
        &model.Update{},
        &model.Checkpoint{},
    ); err != nil {
        return nil, fmt.Errorf("auto migrate: %w", err)
    }
    return db, nil
}
```

GORM Models:

```go
// backend/internal/model/types.go
// JSONMap: GORM 自定义类型，实现 Scanner/Valuer
type JSONMap map[string]any
// 实现 database/sql.Scanner 和 driver.Valuer 接口

// backend/internal/model/session.go
type Session struct {
    ID           string `gorm:"primaryKey"`
    ActiveTabID  int
    CheckpointID string // 当前 checkpoint ID (清除标记用，不删 state blob)
    CreatedAt    time.Time
    UpdatedAt    time.Time
}

type Tab struct {
    ID          string    `gorm:"primaryKey"` // UUID
    SessionID   string    `gorm:"index"`
    ChromeTabID int
    URL         string
    Title       string
    Active      bool
    CreatedAt   time.Time
}

// backend/internal/model/update.go
type Update struct {
    ID        string  `gorm:"primaryKey"`
    SessionID string  `gorm:"index"`    // V0.1 单 session，保留维度字段
    Seq       int64   `gorm:"uniqueIndex"`
    Type      string
    Payload   JSONMap
    CreatedAt time.Time
}

// backend/internal/model/checkpoint.go
type Checkpoint struct {
    ID        string    `gorm:"primaryKey"`
    TaskID    string    `gorm:"index"`    // 任务维度 (非 SessionID)
    NodeKey   string    `gorm:"type:varchar(128);not null"`
    State     []byte    `gorm:"type:blob;not null"`
    CreatedAt time.Time
    UpdatedAt time.Time
}

// backend/internal/model/memory.go
type Memory struct {
    ID              string    `gorm:"primaryKey"`
    SessionID       string    `gorm:"index"`
    PageURL         string
    PageURLPattern  string    `gorm:"index"`
    PageTitle       string
    PageFeatures    string
    DOMSnapshotPath string   // 存文件路径
    ScreenshotPath  string   // 存文件路径
    ActionType      string
    ActionTarget    string
    ActionSelector  string
    ActionValue     string
    Result          string
    FailReason      string
    Source          string
    CreatedAt       time.Time
}
```

### 1.5 Seq 机制

V0.1 单用户单 session，Seq 全局递增。Update.SessionID 维度字段预留，后续多用户扩展时改为 per-session seq。

```go
// backend/internal/ws/seq.go
type SeqManager struct {
    db     *gorm.DB
    maxSeq int64
}

func (s *SeqManager) Init() error {
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

### 1.6 HTTP 路由 + CORS

```go
// backend/internal/handler/handler.go
type Handler struct {
    taskSvc  *service.TaskService
    wsMgr    *ws.Manager
    tabMgr   *service.TabManager
    waiter   *ws.ResponseWaiter
    log      *slog.Logger
}

func NewHandler(taskSvc *service.TaskService, wsMgr *ws.Manager, tabMgr *service.TabManager, waiter *ws.ResponseWaiter, log *slog.Logger) *Handler {
    return &Handler{taskSvc: taskSvc, wsMgr: wsMgr, tabMgr: tabMgr, waiter: waiter, log: log}
}

// ServeWS: 委托给 wsMgr.ServeHTTP (gorilla/websocket 升级处理)
func (h *Handler) ServeWS(w http.ResponseWriter, r *http.Request) {
    h.wsMgr.ServeHTTP(w, r)
}
```

```go
// backend/internal/handler/router.go
func NewRouter(h *Handler) *gin.Engine {
    r := gin.New()
    r.Use(gin.Recovery())
    r.Use(middleware.Logger())

    // CORS: 仅允许 Extension 来源 (AllowOriginFunc 前缀匹配，因 extension origin 含随机 ID)
    r.Use(cors.New(cors.Config{
        AllowOriginFunc: func(origin string) bool {
            return strings.HasPrefix(origin, "chrome-extension://") ||
                strings.HasPrefix(origin, "http://localhost")
        },
        AllowMethods:     []string{"GET", "POST", "PUT", "DELETE"},
        AllowHeaders:     []string{"Authorization", "Content-Type"},
        AllowCredentials: true,
    }))

    r.GET("/health", h.Health)

    v1 := r.Group("/api/v1")
    {
        v1.POST("/tasks", h.StartTask)
        v1.POST("/tasks/:id/stop", h.StopTask)
        v1.POST("/page/state", h.UploadPageState)
        v1.POST("/action/result", h.UploadActionResult)
        v1.POST("/teach", h.UploadTeachData)
        v1.POST("/tabs/opened", h.TabOpened)
        v1.POST("/tabs/closed", h.TabClosed)
        v1.POST("/tabs/activated", h.TabActivated)
        // 轮询端点 (V0.1 单用户，不需 /users/me/ 前缀)
        v1.GET("/updates", h.PollUpdates)
    }

    r.GET("/ws", h.ServeWS)
    return r
}
```

### 1.7 Token 计数

```go
// backend/pkg/token/counter.go
package token

type Counter struct {
    enc tokenizer.Codec
}

func NewCounter() (*Counter, error) {
    enc, err := tokenizer.New(tokenizer.O200kBase)
    if err != nil {
        return nil, err
    }
    return &Counter{enc: enc}, nil
}

func (c *Counter) Count(content string) int {
    tokens, _ := c.enc.Encode(content)
    return len(tokens)
}

func (c *Counter) CountMessages(messages []struct{ Role, Content string }) int {
    total := 0
    for _, m := range messages {
        total += c.Count(m.Role + ": " + m.Content)
    }
    return total
}
```

### 1.8 依赖组装

```go
// backend/cmd/server/main.go
func main() {
    cfg := config.Load() // viper: SetConfigFile("config.yaml") + SetEnvPrefix("MEMBROWSER") + AutomaticEnv()
    log := logger.New(cfg.LogLevel)
    slog.SetDefault(log)  // 设为默认，全局 slog.Info() 等生效

    db := db.NewDB(cfg)
    seqMgr := ws.NewSeqManager(db)
    seqMgr.Init()

    memStore := service.NewMemoryStore(db, cfg.DataDir)
    tabMgr := service.NewTabManager(db)
    waiter := ws.NewResponseWaiter()
    wsMgr := ws.NewManager(db, seqMgr, waiter, cfg.AuthKey)
    modelProvider := eino.NewModelProvider(cfg)
    toolRegistry := eino.NewToolsRegistry(wsMgr.PushUpdate, memStore, tabMgr, waiter)
    runSessionMgr := eino.NewRunSessionManager()
    msgQueue := eino.NewMessageQueue()
    taskService := service.NewTaskService(modelProvider, toolRegistry, wsMgr, seqMgr, runSessionMgr, msgQueue, log)
    h := handler.NewHandler(taskService, wsMgr, tabMgr, waiter, log)

    router := handler.NewRouter(h)
    router.Run(fmt.Sprintf(":%d", cfg.Port))
}
```

### 验证

- 配置从环境变量正确加载
- SQLite WAL 模式启用
- `POST /api/v1/tasks` 返回 200 + task_id
- `GET /api/v1/updates?last_seq=0&limit=500` 返回 `{ "updates": [], "has_more": false, "max_seq": 0 }` (不套 pkg.OK，直接返回原始 JSON)
- CORS 头正确
- slog JSON 格式日志输出，log_level 配置生效

---

## Phase 2: WebSocket 协议

### 2.1 WS 协议定义

帧信封遵循 API.md 规范: `{ "type": string, "payload": object }`，仅两个字段。

```go
// backend/internal/ws/protocol.go

// 服务端 → 客户端帧类型 (遵循 API.md WSServerFrameType)
const (
    FrameConnected = "connected"
    FrameUpdates   = "updates"      // 批量 Update (持久化 + ephemeral)
    FramePing      = "ping"         // 服务端心跳 (遵循 API.md Flow 3)
)

// 客户端 → 服务端帧类型
const (
    ClientFramePong = "pong"        // 客户端回复心跳
)

// Update 类型常量 (payload 内部的 type 字段)
const (
    UpdateTaskStarted    = "task.started"
    UpdateTaskStep       = "task.step"
    UpdateTaskCompleted  = "task.completed"
    UpdateTaskFailed     = "task.failed"
    UpdateNeedHelp       = "need_help"
    UpdateThinking       = "thinking"       // seq=0, ephemeral
    UpdateMessageDelta   = "message.delta"  // seq=0, ephemeral (流式输出)
    UpdatePageQuery      = "page.query"     // seq=0, ephemeral (独立指令)
    UpdateActionExecute  = "action.execute" // seq=0, ephemeral (独立指令)
    UpdateEmpty          = "empty"          // gap filling
)

// ServerFrame: 服务端 → 客户端帧 (payload 为 union 类型)
type ServerFrame struct {
    Type    string `json:"type"`
    Payload any    `json:"payload"`
}

// ClientFrame: 客户端 → 服务端帧
type ClientFrame struct {
    Type    string         `json:"type"`
    Payload map[string]any `json:"payload,omitempty"`
}

// Update (payload 内的单条更新)
type Update struct {
    Seq     int64           `json:"seq"`     // >0 持久化, =0 ephemeral
    Type    string          `json:"type"`
    ID      string          `json:"id"`      // MemBrowser 扩展: request_id 配对 (API.md 无此字段)
    Payload json.RawMessage `json:"payload"`
}

// connected frame payload
type ConnectedPayload struct {
    SessionID  string `json:"session_id"`  // 单用户桌面应用用 session_id 替代 user_id
    ServerTime string `json:"server_time"` // RFC3339 格式 (与 eino-demo 一致)
    MaxSeq     int64  `json:"max_seq"`
}

// updates frame payload: 直接为 []Update (与 eino-demo 一致)
// JSON: { "type": "updates", "payload": [ { "seq": 1, ... }, ... ] }

// page.query / action.execute 的 payload 内含 request_id
type PageQueryPayload struct {
    RequestID        string `json:"request_id"`
    IncludeScreenshot bool  `json:"include_screenshot"`
}

type ActionExecutePayload struct {
    RequestID string `json:"request_id"`
    Action    string `json:"action"`
    Selector  string `json:"selector"`
    Value     string `json:"value,omitempty"`
}
```

**关键设计**:
- 帧信封严格两字段 `{ type, payload }`，遵循 API.md
- `request_id` 在 payload 内部 (非帧信封层)
- 独立指令 (`page.query` / `action.execute`) 也通过 `updates` 帧推送，seq=0 表示 ephemeral
- `task.started` / `task.step` / `task.completed` / `task.failed` / `need_help` 走 `updates` 帧，seq>0 持久化
- `thinking` / `message.delta` 走 `updates` 帧，seq=0 ephemeral (不持久化)
- `empty` 用于 gap filling

### 2.2 WS 连接管理 (gorilla/websocket)

```go
// backend/internal/ws/manager.go
type Manager struct {
    db        *gorm.DB
    upgrader  websocket.Upgrader
    seqMgr    *SeqManager
    waiter    *ResponseWaiter
    conns     map[string]*ConnState
    mu        sync.RWMutex
    authKey   string
    sessionID string // V0.1 单用户: 固定 session ID，用于持久化 Update
}

func NewManager(db *gorm.DB, seqMgr *SeqManager, waiter *ResponseWaiter, authKey string) *Manager {
    return &Manager{
        db:        db,
        upgrader:  websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }},
        seqMgr:    seqMgr,
        waiter:    waiter,
        conns:     make(map[string]*ConnState),
        authKey:   authKey,
        sessionID: uuid.New().String(),
    }
}

// SessionID: 返回当前 session ID (供 callbacks 使用)
func (m *Manager) SessionID() string { return m.sessionID }

type ConnState struct {
    conn      *websocket.Conn
    sendCh    chan []byte    // 缓冲 512
    sessionID string
    lastSeq   int64
    mu        sync.Mutex
}

func (m *Manager) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    token := r.URL.Query().Get("token")
    lastSeq, _ := strconv.ParseInt(r.URL.Query().Get("last_seq"), 10, 64)

    if token != m.authKey {
        http.Error(w, "unauthorized", 401)
        return
    }

    conn, err := m.upgrader.Upgrade(w, r, nil)
    if err != nil { return }

    sessionID := uuid.New().String()
    connState := &ConnState{
        conn:      conn,
        sendCh:    make(chan []byte, 512),  // 足够大，回放 500 条不溢出
        sessionID: sessionID,
        lastSeq:   lastSeq,
    }

    // V0.1 单用户: 固定 key 管理唯一连接，新连接踢掉旧连接
    const connKey = "main"
    m.kickOldConn(connKey)

    // 注册新连接
    m.mu.Lock()
    m.conns[connKey] = connState
    m.mu.Unlock()

    // 发送 connected frame
    m.sendConnected(connState)

    // 回放 missed updates (含 gap filling)
    if lastSeq > 0 && lastSeq < m.seqMgr.Max() {
        m.replayUpdates(connState, lastSeq)
    }

    // 启动 readLoop + writeLoop
    go m.writeLoop(connState)
    go m.readLoop(connState)
}

// sendConnected: 发送 connected frame
func (m *Manager) sendConnected(cs *ConnState) {
    payload := ConnectedPayload{
        SessionID:  cs.sessionID,
        ServerTime: time.Now().UTC().Format(time.RFC3339),
        MaxSeq:     m.seqMgr.Max(),
    }
    frame := ServerFrame{Type: FrameConnected, Payload: payload}
    data, _ := json.Marshal(frame)
    cs.sendCh <- data
}

// PushUpdate: 持久化 (seq>0) 或 ephemeral (seq=0) + 广播
func (m *Manager) PushUpdate(u Update) error {
    if u.Seq == 0 {
        // ephemeral: 不持久化，直接广播 (payload 直接为 []Update，与 eino-demo 一致)
        frame := ServerFrame{Type: FrameUpdates, Payload: []Update{u}}
        m.broadcast(frame)
        return nil
    }
    // 持久化
    seq := m.seqMgr.Next()
    u.Seq = seq
    m.db.Create(&model.Update{SessionID: m.sessionID, Seq: seq, Type: u.Type, ID: u.ID, Payload: u.Payload})
    frame := ServerFrame{Type: FrameUpdates, Payload: []Update{u}}
    m.broadcast(frame)
    return nil
}

// broadcast: 推送给所有连接
func (m *Manager) broadcast(frame ServerFrame) {
    data, _ := json.Marshal(frame)
    m.mu.RLock()
    defer m.mu.RUnlock()
    for _, cs := range m.conns {
        select {
        case cs.sendCh <- data:
        default: // sendCh 满，丢弃 (背压)
        }
    }
}
```

### 2.3 Gap Filling (服务端)

Gap filling 逻辑提取为独立函数，`replayUpdates` 和 `PollUpdates` 复用:

```go
// backend/internal/ws/gapfill.go
func FillGaps(lastSeq int64, updates []model.Update) []Update {
    result := make([]Update, 0, len(updates))
    expectedSeq := lastSeq + 1
    for _, u := range updates {
        for u.Seq > expectedSeq {
            result = append(result, Update{
                Seq:     expectedSeq,
                Type:    UpdateEmpty,
                Payload: json.RawMessage(`{}`),
            })
            expectedSeq++
        }
        result = append(result, Update{
            Seq:     u.Seq,
            Type:    u.Type,
            ID:      u.ID,
            Payload: u.Payload,
        })
        expectedSeq = u.Seq + 1
    }
    return result
}

// backend/internal/ws/replay.go
func (m *Manager) replayUpdates(cs *ConnState, lastSeq int64) {
    var updates []model.Update
    m.db.Where("seq > ? AND seq <= ?", lastSeq, m.seqMgr.Max()).
        Order("seq ASC").Limit(500).Find(&updates)

    result := FillGaps(lastSeq, updates)

    frame := ServerFrame{Type: FrameUpdates, Payload: result}
    data, _ := json.Marshal(frame)
    // 统一通过 sendCh 发送，避免与 writeLoop 竞争写 conn
    select {
    case cs.sendCh <- data:
    default:
        // sendCh 满，丢弃 (回放数据量大时客户端会通过 HTTP 轮询补发)
    }
}
```

### 2.4 HTTP 轮询降级 (含 has_more + gap filling)

```go
// backend/internal/handler/updates.go
// PollUpdates: V0.1 简化 — last_seq=0 时返回所有更新 (受 limit 限制)，不特殊处理
// API.md 规定 last_seq=0 只返回最后一条，有意偏离: 桌面应用首次连接需要更多历史
func (h *Handler) PollUpdates(c *gin.Context) {
    lastSeq, _ := strconv.ParseInt(c.Query("last_seq"), 10, 64)
    limit := c.DefaultQuery("limit", "500")
    limitInt, _ := strconv.Atoi(limit)
    if limitInt > 1000 { limitInt = 1000 }

    var updates []model.Update
    h.db.Where("seq > ?", lastSeq).Order("seq ASC").
        Limit(limitInt + 1).Find(&updates) // 多查一条判断 has_more

    hasMore := len(updates) > limitInt
    if hasMore {
        updates = updates[:limitInt]
    }

    result := ws.FillGaps(lastSeq, updates)

    // 直接返回原始 JSON (不套 pkg.OK)，客户端直接访问 data.updates
    c.JSON(200, gin.H{
        "updates":  result,
        "has_more": hasMore,
        "max_seq":  h.seqMgr.Max(), // 始终返回服务端当前最大 seq
    })
}
```

### 2.5 心跳 (服务端发 ping，客户端回复 pong)

遵循 API.md Flow 3: 服务端每 30s 发 `ping` 帧，客户端回复 `pong`，60s 未收到 pong 断开。

- 服务端 writeLoop 每 30s 发 `{ type: "ping", payload: {} }`
- 客户端收到 ping 后回复 `{ type: "pong" }`
- 服务端 readLoop 收到 pong 后重置 60s 超时计时器
- 60s 未收到任何客户端消息 → 断开，等待重连

```go
// backend/internal/ws/readloop.go
func (m *Manager) readLoop(cs *ConnState) {
    defer func() {
        m.mu.Lock()
        delete(m.conns, "main")
        m.mu.Unlock()
        cs.conn.Close()
    }()

    deadline := 60 * time.Second
    cs.conn.SetReadDeadline(time.Now().Add(deadline))

    for {
        _, msg, err := cs.conn.ReadMessage()
        if err != nil { break }

        var frame ClientFrame
        json.Unmarshal(msg, &frame)

        if frame.Type == ClientFramePong {
            // 客户端回复 pong，重置超时
            cs.conn.SetReadDeadline(time.Now().Add(deadline))
            continue
        }

        // 其他客户端消息暂不处理 (V0.1 客户端只回 pong)
    }
}

// backend/internal/ws/writeloop.go
func (m *Manager) writeLoop(cs *ConnState) {
    pingTicker := time.NewTicker(30 * time.Second)
    defer pingTicker.Stop()
    defer cs.conn.Close()

    for {
        select {
        case data, ok := <-cs.sendCh:
            if !ok { return }
            cs.mu.Lock()
            cs.conn.WriteMessage(websocket.TextMessage, data)
            cs.mu.Unlock()
        case <-pingTicker.C:
            // 服务端主动发 ping (遵循 API.md Flow 3)
            ping, _ := json.Marshal(ServerFrame{Type: FramePing, Payload: map[string]any{}})
            cs.mu.Lock()
            cs.conn.WriteMessage(websocket.TextMessage, ping)
            cs.mu.Unlock()
        }
    }
}
```

### 2.6 重连回放

- Side Panel 连接时带 `?last_seq=N`
- 后端查 `updates WHERE seq > N ORDER BY seq ASC LIMIT 500`
- Gap filling 后通过 `updates` frame 推送

### 2.7 多标签页管理

```go
// backend/internal/service/tab.go
type TabManager struct {
    db          *gorm.DB
    session     *model.Session
    mu          sync.RWMutex
    agentLocked bool
}

func (m *TabManager) OnTabOpened(tabID int, url, title string) { ... }
func (m *TabManager) OnTabClosed(chromeTabID int) { ... }
func (m *TabManager) OnTabActivated(chromeTabID int) { ... }
func (m *TabManager) LockAgent()   { m.agentLocked = true }
func (m *TabManager) UnlockAgent() { m.agentLocked = false }
```

### 2.8 ResponseWaiter (修复内存泄漏)

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
        delete(w.pending, id)
        w.mu.Unlock()
    }()

    select {
    case resp := <-ch:
        return resp, nil
    case <-time.After(timeout):
        return nil, pkg.New(pkg.ErrCodeTimeout, "等待 Extension 响应超时")
    }
}

func (w *ResponseWaiter) Resolve(id string, data json.RawMessage) { ... }
func (w *ResponseWaiter) Clear() { ... }
```

### 验证

- WS 连接后收到 `connected` frame (含 session_id, server_time, max_seq)
- 帧信封严格两字段 `{ type, payload }`
- 心跳: 客户端 30s 发 ping，服务端回 pong + 重置超时
- 断连重连后 seq 回放 (limit 500，含 gap filling)
- HTTP 轮询返回 `{ updates, has_more, max_seq }`
- max_seq 始终返回 seqMgr.Max() (非空结果也正确)
- gap filling: seq 间隙被 empty update 填充
- sendCh 缓冲 512，回放不溢出
- 多标签页: 新标签页自动接管，用户手动切换不干扰 Agent

---

## Phase 3: Eino Agent + Callback 体系 + Tools

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

### 3.2 Callback 三层体系

```go
// backend/internal/eino/runner/handler.go

// RootRunnerHandlerCallback: 回调接口
type RootRunnerHandlerCallback interface {
    OnInputToolCalling(ctx context.Context, info *callbacks.RunInfo, addr compose.Address, input callbacks.CallbackInput)
    OnOutputToolCalling(ctx context.Context, info *callbacks.RunInfo, addr compose.Address, output callbacks.CallbackOutput)
    OnThinking(ctx context.Context, role schema.RoleType, addr compose.Address, reasoningContent string)
    OnOutputting(ctx context.Context, role schema.RoleType, addr compose.Address, content string)
    OnCompleted(ctx context.Context, role schema.RoleType, addr compose.Address, reasoningContent string, outputContent string, usage *schema.TokenUsage)
}

// RootRunnerCallback: 组合接口
type RootRunnerCallback interface {
    RootRunnerHandlerCallback
    adk.CheckPointStore
    OnError(err error)
    OnEnd()
    OnInterrupted(info *adk.InterruptInfo)
}

// RootRunnerHandler: 实现 callbacks.Handler
// tracer 为 nil 时默认使用 otel.Tracer("membrowser:RootRunner-handler")
type RootRunnerHandler struct {
    callback         RootRunnerHandlerCallback
    tracer           trace.Tracer
    onCompletedTimes *atomic.Int32
    LatestOutput     *einomodel.CallbackOutput  // 记录最后一次模型输出
}
```

### 3.3 RootRunnerCallbacks 具体实现

```go
// backend/internal/eino/runner/callbacks.go
type RunCallbackConfig struct {
    TaskID       string
    SessionID    string
    DB           *gorm.DB
    Log          *slog.Logger
    PushUpdate   func(Update) error
    OnComplete   func(success bool, summary string)
    TokenCounter *token.Counter
}

type RootRunnerCallbacks struct {
    cfg                RunCallbackConfig
    mu                 sync.Mutex
    totalPromptTokens     int
    totalCompletionTokens int
    stopped              bool
}

// Stop: 幂等，标记为已停止 (panic recovery / token overflow / context overflow 都调用)
func (c *RootRunnerCallbacks) Stop() {
    c.mu.Lock()
    defer c.mu.Unlock()
    if c.stopped { return }
    c.stopped = true
    // 更新最终 token 统计
    c.cfg.Log.Info("agent stopped",
        "task_id", c.cfg.TaskID,
        "prompt_tokens", c.totalPromptTokens,
        "completion_tokens", c.totalCompletionTokens,
    )
}

// OnThinking: thinking 走 PushUpdate seq=0 (ephemeral)
func (c *RootRunnerCallbacks) OnThinking(ctx context.Context, role schema.RoleType, addr compose.Address, reasoningContent string) {
    c.cfg.PushUpdate(Update{
        Seq:     0, // ephemeral，不持久化
        Type:    UpdateThinking,
        Payload: marshal(map[string]any{"delta": reasoningContent}),
    })
}

// OnOutputting: 流式 delta 走 PushUpdate seq=0 (ephemeral)
// 与 eino-demo 一致: 每个流式 token 不分配 seq，不写 DB
func (c *RootRunnerCallbacks) OnOutputting(ctx context.Context, role schema.RoleType, addr compose.Address, content string) {
    c.cfg.PushUpdate(Update{
        Seq:     0, // ephemeral，不持久化
        Type:    UpdateMessageDelta,
        Payload: marshal(map[string]any{"delta": content}),
    })
}

// OnCompleted: 最终消息，持久化 (seq>0)
func (c *RootRunnerCallbacks) OnCompleted(ctx context.Context, role schema.RoleType, addr compose.Address, reasoningContent string, outputContent string, usage *schema.TokenUsage) {
    c.mu.Lock()
    defer c.mu.Unlock()
    if usage != nil {
        c.totalPromptTokens += usage.PromptTokens
        c.totalCompletionTokens += usage.CompletionTokens
    }
    // 推送持久化的最终消息 (Seq=0，由 PushUpdate 内部分配实际 seq)
    c.cfg.PushUpdate(Update{
        Type:    UpdateTaskStep,
        Payload: marshal(map[string]any{"content": outputContent}),
    })
}

// CheckPointStore: Get/Set
func (c *RootRunnerCallbacks) Get(ctx context.Context, checkpointID string) ([]byte, bool, error) {
    var cp model.Checkpoint
    err := c.cfg.DB.WithContext(ctx).
        Where("task_id = ? AND node_key = ?", c.cfg.TaskID, checkpointID).
        First(&cp).Error
    if err == gorm.ErrRecordNotFound {
        return nil, false, nil
    }
    if err != nil {
        return nil, false, fmt.Errorf("checkpoint get: %w", err)
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
        return fmt.Errorf("checkpoint set: %w", err)
    }
    return c.cfg.DB.WithContext(ctx).Save(&cp).Error
}

// 生命周期
func (c *RootRunnerCallbacks) OnError(err error) { ... }
func (c *RootRunnerCallbacks) OnEnd() { ... }
func (c *RootRunnerCallbacks) OnInterrupted(info *adk.InterruptInfo) {
    // need_help 走 PushUpdate seq>0 (持久化，重连可回放)
    // Seq=0，由 PushUpdate 内部分配实际 seq
    c.cfg.PushUpdate(Update{
        Type:    UpdateNeedHelp,
        Payload: marshal(map[string]any{"info": info}),
    })
}
```

### 3.4 DeepAgent 构建 (完整 Config)

```go
// backend/internal/eino/runner/runner.go
func NewRunner(cfg RunnerConfig) (*Runner, error) {
    chatModel, err := cfg.ModelProvider.GetModel(cfg.Tier)

    // 中间件链: ContextInjection → Reduction
    var handlers []adk.ChatModelAgentMiddleware

    // 1. ContextInjection (消息注入 + token 检查)
    if cfg.MessageQueue != nil {
        var tokenCheck *TokenCheckConfig
        if cfg.TokenCounter != nil {
            tokenCheck = &TokenCheckConfig{Counter: cfg.TokenCounter, MaxTokens: cfg.MaxTokens}
        }
        handlers = append(handlers, NewContextInjectionMiddleware(
            cfg.MessageQueue, cfg.TaskID, tokenCheck,
        ))
    }

    // 2. Reduction (大 DOM 数据裁剪)
    if cfg.ReductionEnabled {
        reductionMW, err := reduction.New(ctx, &reduction.Config{
            SkipTruncation:            true,
            MaxTokensForClear:         100000,
            ClearRetentionSuffixLimit: 2,
            RootDir:                   filepath.Join(os.TempDir(), "membrowser-reduction"),
            ReadFileToolName:          "", // 禁用 read_file 回退，reduction 仍执行 clear 逻辑
        })
        if err != nil {
            return nil, fmt.Errorf("reduction init: %w", err)
        }
        handlers = append(handlers, reductionMW)
    }

    handler := NewRootRunnerHandler(cfg.Callback, nil)

    deepAgent, err := deep.New(ctx, &deep.Config{
        Name:        "membrowser",
        Description: "Web 自动化 Agent",
        ChatModel:   chatModel,
        Instruction: cfg.SystemPrompt,
        SubAgents:   nil, // V0.1 无子 Agent
        ToolsConfig: adk.ToolsConfig{
            ToolsNodeConfig: compose.ToolsNodeConfig{
                Tools:               cfg.Tools,
                UnknownToolsHandler: UnknownToolsHandler,
            },
            EmitInternalEvents: true,
        },
        MaxIteration:      20,
        Handlers:          handlers,
        WithoutWriteTodos: true,
    })

    runner := adk.NewRunner(ctx, adk.RunnerConfig{
        Agent:           deepAgent,
        EnableStreaming: true,
        CheckPointStore: cfg.Callback,
    })

    return &Runner{runner: runner, handler: handler}, nil
}

// Runner 包装方法: 注入 handler (callbacks) 和 checkpointID
// H5: Run 不返回 error (与 eino-demo 一致)，Resume/ResumeWithParams 返回 error
func (r *Runner) Run(ctx context.Context, messages []*schema.Message, taskID string, handler *RootRunnerHandler) *adk.AsyncIterator[*adk.AgentEvent] {
    return r.runner.Run(ctx, messages, taskID, adk.WithCallbacks(handler), adk.WithCheckPointID(taskID))
}

func (r *Runner) Resume(ctx context.Context, checkpointID string, handler *RootRunnerHandler) (*adk.AsyncIterator[*adk.AgentEvent], error) {
    return r.runner.Resume(ctx, checkpointID, adk.WithCallbacks(handler), adk.WithCheckPointID(checkpointID))
}

func (r *Runner) ResumeWithParams(ctx context.Context, checkpointID string, params *adk.ResumeParams, handler *RootRunnerHandler) (*adk.AsyncIterator[*adk.AgentEvent], error) {
    return r.runner.ResumeWithParams(ctx, checkpointID, params, adk.WithCallbacks(handler), adk.WithCheckPointID(checkpointID))
}

func UnknownToolsHandler(ctx context.Context, name string, args json.RawMessage) (string, error) {
    return "", fmt.Errorf("unknown tool: %s", name)
}
```

### 3.5 RunSessionManager (含级联停止 + placeholder)

```go
// backend/internal/eino/runner/run_session.go
type AgentRunSession struct {
    CancelFunc   context.CancelFunc
    TaskID       string
    CheckpointID string
    ResumeParams *adk.ResumeParams
    IsRunning    bool
    StopFunc     func()
    done         chan struct{}
    ParentID     string  // 父会话 ID (用于级联停止)
}

type RunSessionManager struct {
    mu       sync.RWMutex
    sessions map[string]*AgentRunSession
}

func NewRunSessionManager() *RunSessionManager {
    return &RunSessionManager{sessions: make(map[string]*AgentRunSession)}
}

func (m *RunSessionManager) TryStart(taskID string, cancel context.CancelFunc, stopFunc func(), parentID ...string) bool {
    m.mu.Lock()
    defer m.mu.Unlock()
    pid := ""
    if len(parentID) > 0 {
        pid = parentID[0]
    }
    if existing, ok := m.sessions[taskID]; ok {
        if existing.IsRunning {
            return false
        }
        m.createSession(taskID, cancel, stopFunc, pid)
        m.sessions[taskID].CheckpointID = existing.CheckpointID
        m.sessions[taskID].ResumeParams = existing.ResumeParams
        return true
    }
    m.createSession(taskID, cancel, stopFunc, pid)
    return true
}

// Stop: 级联停止子会话 (三段式: RLock 收集 → Unlock → 递归 → Lock 提取 → Unlock → 锁外执行)
func (m *RunSessionManager) Stop(taskID string) {
    // 1. RLock 收集子会话 IDs
    m.mu.RLock()
    var childIDs []string
    for id, s := range m.sessions {
        if s.ParentID == taskID && s.IsRunning {
            childIDs = append(childIDs, id)
        }
    }
    m.mu.RUnlock()

    // 2. 递归停止子会话 (锁外)
    for _, childID := range childIDs {
        m.Stop(childID)
    }

    // 3. Lock 内读取 IsRunning、设置 false、提取函数引用 → Unlock → 锁外执行
    m.mu.Lock()
    s, ok := m.sessions[taskID]
    if !ok || !s.IsRunning {
        m.mu.Unlock()
        return
    }
    s.IsRunning = false
    cancelFn := s.CancelFunc
    stopFn := s.StopFunc
    doneCh := s.done
    m.mu.Unlock()

    if cancelFn != nil {
        cancelFn()
    }
    if stopFn != nil {
        stopFn()
    }
    select {
    case <-doneCh:
    case <-time.After(2 * time.Second):
    }
}

func (m *RunSessionManager) Done(taskID string) { ... }
func (m *RunSessionManager) Cleanup(taskID string) { ... }

// SetResumeParams: 不存在时创建 placeholder
func (m *RunSessionManager) SetResumeParams(taskID string, params *adk.ResumeParams) {
    m.mu.Lock()
    defer m.mu.Unlock()
    s, ok := m.sessions[taskID]
    if !ok {
        // placeholder session: Agent 已结束，但 resume params 需要保留
        s = &AgentRunSession{IsRunning: false, done: make(chan struct{})}
        close(s.done)
        m.sessions[taskID] = s
    }
    s.ResumeParams = params
}

func (m *RunSessionManager) GetResumeParams(taskID string) (*adk.ResumeParams, bool) { ... }
func (m *RunSessionManager) ClearResumeParams(taskID string) { ... }
func (m *RunSessionManager) SetCheckpointID(taskID string, id string) { ... }
func (m *RunSessionManager) GetCheckpointID(taskID string) string { ... }
```

### 3.6 MessageQueue + ContextInjectionMiddleware

```go
// backend/internal/eino/runner/message_queue.go
type MessageQueue struct {
    mu     sync.Mutex
    queues map[string][]string
}

func NewMessageQueue() *MessageQueue { ... }
func (q *MessageQueue) Enqueue(taskID string, content string) { ... }
func (q *MessageQueue) Drain(taskID string) []string { ... }
func (q *MessageQueue) HasPending(taskID string) bool { ... }
func (q *MessageQueue) Cleanup(taskID string) { ... }
```

```go
// backend/internal/eino/runner/context_injection.go
type TokenOverflowError struct {
    TokenCount      int
    MaxTokens       int
    DrainedMessages []string // 溢出时需重新入队的消息
}

func (e *TokenOverflowError) Error() string {
    return fmt.Sprintf("token overflow: %d > %d", e.TokenCount, e.MaxTokens)
}

// TokenCheckConfig: token 溢出检测配置
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

// NewContextInjectionMiddleware: 初始化 BaseChatModelAgentMiddleware
func NewContextInjectionMiddleware(queue *MessageQueue, taskID string, tokenCheck *TokenCheckConfig) *contextInjectionMiddleware {
    return &contextInjectionMiddleware{
        BaseChatModelAgentMiddleware: &adk.BaseChatModelAgentMiddleware{},
        queue:      queue,
        taskID:     taskID,
        tokenCheck: tokenCheck,
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
            return ctx, state, &TokenOverflowError{
                TokenCount:      totalTokens,
                MaxTokens:       m.tokenCheck.MaxTokens,
                DrainedMessages: drained, // 保存已排空消息，供溢出恢复重新入队
            }
        }
    }
    return ctx, state, nil
}
```

### 3.7 Tools 注册 (utils.InferTool)

```go
// backend/internal/eino/tools/registry.go
func (r *Registry) buildTools() {
    getPageState, err := utils.InferTool("get_page_state", "获取当前页面状态",
        func(ctx context.Context, input GetPageStateInput) (GetPageStateOutput, error) {
            return r.runGetPageState(ctx, input)
        })
    if err != nil { panic("failed to create get_page_state: " + err.Error()) }

    executeAction, err := utils.InferTool("execute_action", "执行页面操作", ...)
    if err != nil { panic("failed to create execute_action: " + err.Error()) }
    searchMemory, err := utils.InferTool("search_memory", "查找记忆", ...)
    if err != nil { panic("failed to create search_memory: " + err.Error()) }
    saveMemory, err := utils.InferTool("save_memory", "保存记忆", ...)
    if err != nil { panic("failed to create save_memory: " + err.Error()) }
    askHuman, err := NewAskHumanTool()  // 见 3.8
    if err != nil { panic("failed to create ask_human: " + err.Error()) }
    listTabs, err := utils.InferTool("list_tabs", "列出标签页", ...)
    if err != nil { panic("failed to create list_tabs: " + err.Error()) }
    switchTab, err := utils.InferTool("switch_tab", "切换标签页", ...)
    if err != nil { panic("failed to create switch_tab: " + err.Error()) }
    openTab, err := utils.InferTool("open_tab", "打开标签页", ...)
    if err != nil { panic("failed to create open_tab: " + err.Error()) }

    r.tools = []tool.BaseTool{
        getPageState, executeAction, searchMemory, saveMemory,
        askHuman, listTabs, switchTab, openTab,
    }
}
```

### 3.8 ask_human Tool (修正 Interrupt 逻辑 + gob 注册)

```go
// backend/internal/eino/tools/ask_human.go
func init() {
    // Eino checkpoint 序列化使用 gob，注册自定义类型
    // 如果 Interrupt info 中包含自定义 struct，需在此注册
    gob.Register(map[string]any{})
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
                // re-interrupt 用 nil，不覆盖用户回答
                return AskHumanOutput{}, tool.Interrupt(ctx, nil)
            }
            // 首次调用: 附带结构化信息
            return AskHumanOutput{}, tool.Interrupt(ctx, map[string]any{
                "type":               "ask_human",
                "message":            input.Message,
                "highlight_selector": input.HighlightSelector,
            })
        })
}
```

### 3.9 Agent 恢复流程 (含 DB 清理 + context.WithoutCancel)

```go
// backend/internal/service/task.go
func (s *TaskService) resumeAgent(ctx context.Context, taskID string, interruptID string, answerValue string) {
    // H3: 停止旧 Agent + 清理 (Stop 幂等，已停止的 Agent 再调用无副作用)
    s.runSessionMgr.Stop(taskID)
    s.runSessionMgr.Cleanup(taskID)

    // 清除 Session 上的 checkpoint_id 标记 (不动 Checkpoint state blob，ResumeWithParams 需要读取)
    s.db.Model(&model.Session{}).
        Where("id = ?", taskID).
        Update("checkpoint_id", "")

    // 设置恢复参数
    s.runSessionMgr.SetResumeParams(taskID, &adk.ResumeParams{
        Targets: map[string]any{
            interruptID: answerValue,
        },
    })
    s.runSessionMgr.SetCheckpointID(taskID, taskID)

    // 重启 Agent (使用 context.WithoutCancel 传递父 context)
    go s.runAgent(context.WithoutCancel(ctx), taskID, "")
}
```

### 3.10 Agent 事件循环 (含 Interrupt + Token Overflow + panic recovery + pending 检查)

参照 eino-demo `chat.runagent.go` 单层 goroutine 模式: defer 链 (panic recovery → cleanup → pending check → done) 保证所有退出路径都正确清理。**关键差异**: eino-demo 的 `return` 直接触发 defer 链，如果用双层 goroutine，内层 `return` 只关闭 done channel，不触发外层 defer。

```go
// backend/internal/service/task.go
func (s *TaskService) runAgent(ctx context.Context, taskID string, taskDesc string) {
    ctx, cancel := context.WithTimeout(ctx, DefaultAgentTimeout)

    if !s.runSessionMgr.TryStart(taskID, cancel, nil) {
        cancel()
        s.msgQueue.Enqueue(taskID, taskDesc)
        return
    }

    // 每次 runAgent 创建新的 callbacks 和 handler (F4/H1: 不共享，避免 run 间状态污染)
    callbacks := NewRootRunnerCallbacks(RunCallbackConfig{
        TaskID:    taskID,
        SessionID: s.wsMgr.SessionID(),
        DB:        s.db,
        Log:       s.log,
        PushUpdate: func(u Update) error { return s.wsMgr.PushUpdate(u) },
    })
    handler := NewRootRunnerHandler(callbacks, nil)

    done := make(chan struct{})
    go func() {
        // defer 链: 保证所有退出路径 (正常结束 / panic / token overflow / context overflow) 都执行清理
        defer func() {
            if r := recover(); r != nil {
                slog.Error("agent panic", "task_id", taskID, "recover", r)
                callbacks.Stop() // F4: 使用局部变量，非 s.callbacks
                s.wsMgr.PushUpdate(Update{Type: UpdateTaskFailed, Payload: marshal(map[string]any{
                    "error": fmt.Sprintf("agent panic: %v", r),
                })})
            }
            callbacks.Stop()   // F3/H4: 所有退出路径都调用 Stop (幂等)
            s.runSessionMgr.Cleanup(taskID)
            // pending 消息检查: Agent 结束后仍有消息入队，自动重启
            if s.msgQueue.HasPending(taskID) {
                go s.runAgent(context.WithoutCancel(ctx), taskID, "")
            } else {
                s.runSessionMgr.Done(taskID)
            }
        }()
        defer close(done)

        // 检查 checkpoint + resume
        // H5: Run 不返回 error，Resume/ResumeWithParams 返回 error
        checkpointID := s.runSessionMgr.GetCheckpointID(taskID)
        var iter *adk.AsyncIterator[*adk.AgentEvent]
        if checkpointID != "" {
            var resumeErr error
            resumeParams, hasResume := s.runSessionMgr.GetResumeParams(taskID)
            if hasResume {
                s.runSessionMgr.ClearResumeParams(taskID)
                iter, resumeErr = s.runner.ResumeWithParams(ctx, checkpointID, resumeParams, handler)
            } else {
                iter, resumeErr = s.runner.Resume(ctx, checkpointID, handler)
            }
            if resumeErr != nil {
                // Resume 失败，回退到 Run (不返回 error)
                messages := s.loadMessages(taskID)
                iter = s.runner.Run(ctx, messages, taskID, handler)
            }
        } else {
            messages := s.loadMessages(taskID)
            iter = s.runner.Run(ctx, messages, taskID, handler)
        }

        // 事件循环 (单层 goroutine，return 直接触发 defer 链)
        for {
            event, ok := iter.Next()
            if !ok {
                callbacks.OnEnd() // 更新 token 统计 + 触发 OnComplete 回调
                return
            }
            if event == nil {
                continue
            }

            // 错误处理: Token Overflow / API Context Overflow / 其他 (先于 Interrupt 检查)
            if event.Err != nil {
                // 路径 1: Middleware 检测的 TokenOverflow
                var overflowErr *runner.TokenOverflowError
                if errors.As(event.Err, &overflowErr) {
                    for _, content := range overflowErr.DrainedMessages {
                        s.msgQueue.Enqueue(taskID, content)
                    }
                    // F3: Stop old Agent before starting new one
                    callbacks.Stop()
                    s.runSessionMgr.Cleanup(taskID)
                    go s.runAgent(context.WithoutCancel(ctx), taskID, "")
                    return // defer 链: Stop 幂等 + Cleanup 已执行 + skip pending check (已重启)
                }
                // 路径 2: API 返回的 context overflow
                if isContextOverflowError(event.Err) {
                    slog.Warn("api context overflow", "task_id", taskID, "error", event.Err)
                    s.wsMgr.PushUpdate(Update{Type: UpdateTaskFailed, Payload: marshal(map[string]any{
                        "error": "对话过长，请重新开始任务",
                    })})
                    return // defer 链处理 cleanup + done
                }
                // 其他错误
                s.wsMgr.PushUpdate(Update{Type: UpdateTaskFailed, Payload: marshal(map[string]any{
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
                callbacks.OnInterrupted(event.Action.Interrupted)
                return // defer 链处理 cleanup + done (不触发 pending check，等待 resume)
            }

            switch event.Type {
            case adk.EventThinking:
                s.wsMgr.PushUpdate(Update{Seq: 0, Type: UpdateThinking, Payload: marshal(map[string]any{
                    "delta": event.Delta,
                })})
            case adk.EventOutputting:
                s.wsMgr.PushUpdate(Update{Seq: 0, Type: UpdateMessageDelta, Payload: marshal(map[string]any{
                    "delta": event.Delta,
                })})
            case adk.EventCompleted:
                s.wsMgr.PushUpdate(Update{Type: UpdateTaskCompleted, Payload: marshal(map[string]any{
                    "summary": event.Output,
                })})
            }
        }
    }()

    // select: 等待事件循环完成或超时
    timer := time.NewTimer(DefaultAgentTimeout)
    defer timer.Stop()
    select {
    case <-done:
        // 事件循环正常结束 (defer 链已在 goroutine 内执行)
    case <-timer.C:
        // 超时: cancel 触发 goroutine 内 ctx.Done()，defer 链处理 cleanup
        slog.Warn("agent timeout", "task_id", taskID)
        cancel()
        s.wsMgr.PushUpdate(Update{Type: UpdateTaskFailed, Payload: marshal(map[string]any{
            "error": "Agent 执行超时",
        })})
    }
}
```

// isContextOverflowError: 检测 API 返回的 context overflow 错误
func isContextOverflowError(err error) bool {
    msg := strings.ToLower(err.Error())
    return strings.Contains(msg, "context_length_exceeded") ||
        strings.Contains(msg, "prompt_too_long") ||
        strings.Contains(msg, "maximum context length")
}

### 3.11 Agent 超时

```go
const DefaultAgentTimeout = 20 * time.Minute
```

### 3.12 System Prompt

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
- Callback 三层体系: Handler → Callbacks → WS 推送
- deep.Config 所有字段正确 (EmitInternalEvents, Handlers, WithoutWriteTodos, SubAgents:nil)
- OnOutputting 推送 seq=0 ephemeral，OnCompleted 推送 seq>0 持久化
- Interrupt 事件正确处理: checkpoint 保存 → OnInterrupted → break
- ask_human re-interrupt 用 nil
- ResumeWithParams 恢复: 清理 DB checkpoint → 设置 params → 重启
- RunSessionManager: TryStart 拒绝并发 (409)，Stop 级联停止子会话
- SetResumeParams: session 不存在时创建 placeholder
- Token Overflow: 捕获 → 重新入队 DrainedMessages → 重启
- panic recovery: Agent panic → task.failed 推送
- pending 消息检查: Agent 结束后自动重启消费
- thinking/message.delta 走 PushUpdate seq=0 (不持久化)
- 同时提交两个任务，第二个返回 409
- Agent 超时后推送 task.failed

---

## Phase 4: 记忆系统

### 4.1 记忆存储

```go
// backend/internal/service/memory.go
type MemoryStore struct {
    db      *gorm.DB
    dataDir string
}

func (s *MemoryStore) Save(m *model.Memory) error
func (s *MemoryStore) Search(pageURLPattern, actionType, actionTarget string) (*model.Memory, error)
func (s *MemoryStore) SaveScreenshot(data []byte) (string, error)
func (s *MemoryStore) SaveDOMSnapshot(data string) (string, error)
```

### 4.2 URL 标准化

```go
// backend/internal/service/url_normalize.go
func NormalizeURL(rawURL string) string {
    // /product/12345 → /product/{id}
}
```

### 4.3 记忆匹配 (INSTR)

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

### 4.4 人类示教流程

```go
// backend/internal/handler/teach.go
func (h *Handler) UploadTeach(c *gin.Context) {
    // 1. 接收示教数据
    // 2. 保存截图到文件系统
    // 3. 构造 Memory (source=human, result=success)
    // 4. 存入 SQLite
    // 5. 推送 update (seq>0)
    // 6. 调用 waiter.Resolve() 通知 Agent 继续
}
```

### 验证

- 人类示教后记忆正确存入 SQLite + 文件系统
- search_memory 命中后 Agent 直接复用 (0 Token)
- URL 标准化正确

---

## Phase 5: Chrome Extension (含 applyUpdates)

### 5.1 Manifest V3

```json
{
  "manifest_version": 3,
  "name": "MemBrowser",
  "permissions": ["sidePanel", "activeTab", "scripting", "storage"],
  "host_permissions": ["http://localhost:*/*"],
  "side_panel": { "default_path": "sidepanel.html", "openPanelOnActionClick": true },
  "background": { "service_worker": "background.js" },
  "content_scripts": [{ "matches": ["<all_urls>"], "js": ["content.js"], "run_at": "document_idle" }]
}
```

### 5.2 共享类型 (extension/src/shared/types.ts)

先定义 TS 接口，与 Go 侧保持一致:

```typescript
// extension/src/shared/types.ts
export interface Frame {
  type: string;
  payload: any;
}

export interface Update {
  seq: number;      // >0 持久化, =0 ephemeral
  type: string;
  id: string;       // UUID, 用于 request_id 配对
  payload: any;
}

export interface ConnectedPayload {
  session_id: string;
  server_time: string;  // RFC3339
  max_seq: number;
}

export interface PageQueryPayload {
  request_id: string;
  include_screenshot: boolean;
}

export interface ActionExecutePayload {
  request_id: string;
  action: string;
  selector: string;
  value?: string;
}
```

### 5.3 applyUpdates 流程

参考 API.md Flow 5，Extension 端实现 seq 追踪 + gap 检测 + 持久化 + 中断恢复:

```typescript
// extension/src/sidepanel/update-store.ts
class UpdateStore {
  private lastSeq: number = 0;
  private onGapDetected: (from: number, to: number) => void;

  async init() {
    const data = await chrome.storage.local.get(['lastSeq']);
    this.lastSeq = data.lastSeq || 0;
  }

  getLastSeq(): number { return this.lastSeq; }

  // applyUpdates: 核心逻辑 (遵循 API.md Flow 5)
  // H6: 先检查 persisted 连续性，再按原始顺序处理所有 update
  applyUpdates(updates: Update[]): void {
    // Step 1: 分区
    const ephemeral = updates.filter(u => u.seq === 0);
    const persisted = updates.filter(u => u.seq > 0).sort((a, b) => a.seq - b.seq);

    // Step 2: 检查 persisted 连续性 (API.md Flow 5: 先检查，有 gap 则中止整个批次)
    for (const update of persisted) {
      if (update.seq > this.lastSeq + 1) {
        console.warn(`seq gap: expected ${this.lastSeq + 1}, got ${update.seq}`);
        // ABORT: 丢弃整个批次 (含 ephemeral)，触发 HTTP pull
        this.onGapDetected(this.lastSeq + 1, update.seq - 1);
        return;
      }
    }

    // Step 3: 按原始顺序处理所有 update (ephemeral + persisted 混合)
    for (const update of updates) {
      if (update.seq === 0) {
        // ephemeral: 直接处理，不追踪 seq
        this.processUpdate(update);
      } else if (update.seq <= this.lastSeq) {
        continue; // 已处理，跳过
      } else if (update.type === 'empty') {
        this.lastSeq = update.seq;
      } else {
        this.processUpdate(update);
        this.lastSeq = update.seq;
      }
    }

    this.persist();
  }

  private processUpdate(update: Update) {
    switch (update.type) {
      case 'task.started':     this.onTaskStarted(update.payload); break;
      case 'task.step':        this.onTaskStep(update.payload); break;
      case 'task.completed':   this.onTaskCompleted(update.payload); break;
      case 'task.failed':      this.onTaskFailed(update.payload); break;
      case 'need_help':        this.onNeedHelp(update.payload); break;
      case 'thinking':         this.onThinking(update.payload); break;
      case 'message.delta':    this.onMessageDelta(update.payload); break;
      case 'page.query':       this.onPageQuery(update.payload); break;
      case 'action.execute':   this.onActionExecute(update.payload); break;
      case 'empty':            break; // gap filling，已在 applyUpdates 中处理
      default:                 console.warn('unknown update type:', update.type);
    }
  }

  private async persist() {
    await chrome.storage.local.set({ lastSeq: this.lastSeq });
  }
}
```

### 5.4 WS 客户端 (遵循 API.md)

```typescript
// extension/src/sidepanel/ws-client.ts
class WSClient {
  private ws: WebSocket | null = null;
  private updateStore: UpdateStore;
  private reconnectDelay: number = 1000;
  private maxReconnectDelay: number = 30000;
  private timeoutTimer: number | null = null;

  constructor(updateStore: UpdateStore) {
    this.updateStore = updateStore;
  }

  connect(url: string, authKey: string) {
    const lastSeq = this.updateStore.getLastSeq();
    this.ws = new WebSocket(`${url}?token=${authKey}&last_seq=${lastSeq}`);

    this.ws.onopen = () => {
      this.reconnectDelay = 1000;
      this.resetTimeout(); // 启动超时检测 (服务端发 ping，客户端回 pong)
    };

    this.ws.onmessage = (e) => this.handleFrame(JSON.parse(e.data));

    this.ws.onclose = () => {
      this.stopTimers();
      setTimeout(() => this.connect(url, authKey), this.reconnectDelay);
      this.reconnectDelay = Math.min(this.reconnectDelay * 2, this.maxReconnectDelay);
    };

    this.ws.onerror = () => this.ws?.close();
  }

  private handleFrame(frame: Frame) {
    switch (frame.type) {
      case 'connected':
        // 服务端返回 max_seq，如果本地 lastSeq < max_seq，服务端会自动回放
        break;

      case 'ping':
        // 服务端心跳 (API.md Flow 3): 回复 pong
        this.send({ type: 'pong' });
        this.resetTimeout();
        break;

      case 'updates':
        // 批量 update (payload 直接为 []Update，与 eino-demo 一致)
        this.updateStore.applyUpdates(frame.payload);
        this.resetTimeout(); // 收到 updates 也重置超时
        break;

      default:
        console.warn('unknown frame type:', frame.type);
        break;
    }
  }

  // F2: 心跳方向修正 — 服务端发 ping，客户端回 pong，不再主动发 ping
  // 60s 未收到服务端消息 (ping / updates) → 断开重连
  private resetTimeout() {
    if (this.timeoutTimer) clearTimeout(this.timeoutTimer);
    this.timeoutTimer = window.setTimeout(() => {
      console.warn('server timeout, closing');
      this.ws?.close();
    }, 60000);
  }

  private stopTimers() {
    if (this.timeoutTimer) clearTimeout(this.timeoutTimer);
  }

  private send(data: any) {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(data));
    }
  }
}
```

### 5.5 HTTP 轮询降级

```typescript
// extension/src/sidepanel/poll-fallback.ts
class PollFallback {
  private timer: number | null = null;
  private interval: number = 1000;  // 1s 起始 (遵循 API.md)
  private maxInterval: number = 30000;
  private maxConsecutive: number = 10;  // 防止无限递归
  private consecutive: number = 0;

  start(baseUrl: string, updateStore: UpdateStore) {
    this.poll(baseUrl, updateStore);
  }

  private async poll(baseUrl: string, updateStore: UpdateStore) {
    try {
      const lastSeq = updateStore.getLastSeq();
      const resp = await fetch(`${baseUrl}/api/v1/updates?last_seq=${lastSeq}&limit=500`);
      const data = await resp.json();

      if (data.updates.length > 0) {
        updateStore.applyUpdates(data.updates);
        this.interval = 1000; // 有数据，重置
        this.consecutive = 0;
      } else {
        this.interval = Math.min(this.interval * 2, this.maxInterval);
      }

      // has_more: 立即继续，但限制连续次数
      if (data.has_more && this.consecutive < this.maxConsecutive) {
        this.consecutive++;
        this.poll(baseUrl, updateStore);
        return;
      }
    } catch (e) {
      this.interval = Math.min(this.interval * 2, this.maxInterval);
    }

    this.consecutive = 0; // 定时器触发时重置连续计数
    this.timer = window.setTimeout(() => this.poll(baseUrl, updateStore), this.interval);
  }

  stop() {
    if (this.timer) clearTimeout(this.timer);
  }
}
```

### 5.6 消息路由层

```
后端 WS → Side Panel (WS 客户端 + applyUpdates)
                ↓ chrome.runtime.sendMessage
          Background Service Worker
                ↓ chrome.tabs.sendMessage(tabId)
          Content Script (目标标签页)
```

```typescript
// extension/src/background/tab-router.ts
chrome.tabs.onCreated.addListener((tab) => {
  chrome.runtime.sendMessage({ type: 'tab_event', event: 'opened',
    payload: { tab_id: tab.id, url: tab.url, title: tab.title } });
});
chrome.tabs.onRemoved.addListener((tabId) => {
  chrome.runtime.sendMessage({ type: 'tab_event', event: 'closed',
    payload: { tab_id: tabId } });
});
chrome.tabs.onActivated.addListener((activeInfo) => {
  chrome.runtime.sendMessage({ type: 'tab_event', event: 'activated',
    payload: { tab_id: activeInfo.tabId } });
});
chrome.runtime.onMessage.addListener((msg, sender, sendResponse) => {
  if (msg.type === 'forward_to_tab') {
    chrome.tabs.sendMessage(msg.tabId, msg.frame).then(sendResponse);
    return true;
  }
});
```

### 5.7 Content Script — DOM 采集

```typescript
interface DOMNode {
  tag: string; chromeTabId: string; text: string;
  attributes: Record<string, string>; interactable: boolean; children: DOMNode[];
}
function collectDOM(): DOMNode { /* 限制 500 个可交互元素 */ }
```

### 5.8 Content Script — 指令执行

```typescript
function executeAction(action: {
  request_id: string;  // 从 payload 中获取，用于 HTTP 回调配对
  type: 'click' | 'input' | 'scroll' | 'navigate';
  selector: string; value?: string;
}): void {
  const el = document.querySelector(action.selector);
  if (!el) { reportResult(action.request_id, false, 'element not found'); return; }
  // 执行操作，通过 HTTP POST /api/v1/action/result 上报 (带 request_id)
  reportResult(action.request_id, true);
}
```

### 5.9 Content Script — 人类示教 UI

```typescript
function showHighlight(selector: string, message: string, timeout: number = 300000) {
  // 高亮 → 用户点击 → HTTP 上报 → 超时 5 分钟
}
```

### 5.10 Side Panel UI

参考 `docs/prototype/extension-mockup.html`:
- 模型选择 / 任务输入 / 执行进度 / thinking 流式显示
- 人类示教提示 / 连接状态
- 设置 (折叠式): WebSocket URL + Auth Key
- 任务运行中: 灰掉提交按钮，显示停止按钮

### 验证

- Side Panel WS 连接成功，收到 `connected` frame
- applyUpdates seq 追踪正确，gap 检测触发 HTTP pull + 中断当前批次
- chrome.storage.local 持久化 lastSeq
- WS 客户端每 30s 发 ping，服务端回 pong
- 60s 无响应自动断开重连
- 重连后 seq 回放恢复状态
- HTTP 轮询降级: 1s 起始，指数退避，has_more 限制连续 10 次
- 独立指令 (page.query/action.execute) 的 request_id 在 payload 内正确传递
- 多标签页自动接管
- 人类示教: 高亮 → 用户点击 → 超时 5 分钟

---

## Phase 6: 集成测试 + 联调

### 6.1 测试框架

```go
// github.com/stretchr/testify + net/http/httptest
```

### 6.2 L1 单元测试

| 测试对象 | 测试用例 |
|---------|---------|
| pkg/errors | ErrorCode 创建、AppError |
| pkg/response | 统一响应信封 (OK/Fail) |
| pkg/token | Token 计数、空输入 |
| SeqManager | 并发安全、Init 恢复 |
| ResponseWaiter | 超时清理、正常返回、并发 |
| RunSessionManager | TryStart 拒绝并发、Stop 级联、placeholder |
| MessageQueue | Enqueue/Drain/Cleanup |
| URL Normalize | 纯数字、UUID、多级路径 |
| MemoryStore.Search | 精确匹配、无结果、INSTR |
| TabManager | 创建、关闭、Agent 锁定 |
| RootRunnerCallbacks | Get/Set checkpoint、token 累计 |
| ask_human Tool | 通过构造带特定 key 的 context.Context 模拟中断状态，验证 GetInterruptState/GetResumeContext/Interrupt(nil) 的正确分支 |
| UpdateStore (TS) | applyUpdates seq 追踪、gap 检测中断、empty 处理 |
| FillGaps | 间隙填充、无间隙、空输入 |

### 6.3 L2 集成测试

| 场景 | 验证点 |
|------|--------|
| 完整任务流程 | 任务 → Agent → Callback → WS updates 帧 → Extension → HTTP 回调 → 完成 |
| 记忆复现 | 第一次 AI 推理，第二次记忆命中 (0 Token) |
| 人类示教 | ask_human → Interrupt → checkpoint → 用户操作 → resumeAgent (清理 DB + WithoutCancel) → ResumeWithParams → 恢复 |
| 多标签页 | 新标签页打开 → 自动切换 → 继续执行 |
| WS 断连重连 | 断连 → 重连 → seq 回放 (含 gap filling) → 状态恢复 |
| HTTP 轮询降级 | WS 断开 → 轮询 (1s 起始) → has_more → 连续 10 次限制 |
| 并发任务拒绝 | RunSessionManager.TryStart 拒绝 → 409 |
| Agent 超时 | 20 分钟超时 → task.failed |
| Token overflow | ContextInjectionMiddleware 检测 → DrainedMessages 重新入队 → 重启 |
| pending 消息 | Agent 忙时入队 → 结束后自动重启消费 |
| Interrupt 事件 | ask_human → 事件循环捕获 event.Action.Interrupted → 保存 checkpoint → break |

**SQLite 清理**: 每个 L2 场景使用独立临时 db 文件（`t.TempDir()`），测试结束后自动清理，遵循 CLAUDE.md §6 要求。

### 6.4 L3 E2E 测试 (chrome-mcp)

通过 chrome-mcp 自动化:
1. 启动后端 (`make run`)
2. Chrome 加载 Extension (chrome-mcp 打开 Side Panel)
3. 输入任务 → 检查 UI 渲染 (thinking 流式显示)
4. 验证执行进度列表更新
5. 完成后检查执行统计 (token 计数)

手动验证:
- 人类示教流程 (需真实用户操作)
- 多标签页切换 (需观察自动接管)

### 6.5 异常测试 (L3)

| 场景 | 验证点 |
|------|--------|
| 用户关闭活跃标签页 | 自动切换到最近标签页 |
| 所有标签页关闭 | Agent 暂停，通知用户 |
| AI API 限流 | 重试或降级 |
| DOM 采集返回空 | Agent 报错并尝试其他方案 |
| Side Panel 关闭 | WS 断开 → HTTP 轮询降级 → 重连恢复 |
| 服务端重启 | Extension 重连 → last_seq 回放 |
| Agent panic | panic recovery → task.failed 推送 |

### 验证

- 所有 L1 单元测试通过
- 所有 L2 集成测试通过
- L3 chrome-mcp 自动化流程通过
- L3 异常测试覆盖

---

## 关键技术决策

### 1. 不用 uber/fx，手动组装 (有意偏离 CLAUDE.md)

V0.1 单用户桌面应用，复杂度不需要 fx。此决策有意偏离 CLAUDE.md 第 10 节的 fx 要求。`main()` 中手动 `New()` 即可。

### 2. WS 客户端在 Side Panel

MV3 Service Worker 会被 Chrome 休眠，WS 客户端放在 Side Panel 页面更可靠。Side Panel 关闭时降级为 HTTP 轮询。

### 3. ask_human 用 tool.Interrupt

- 首次调用: `tool.Interrupt(ctx, map[string]any{...})` — 附带结构化信息
- re-interrupt: `tool.Interrupt(ctx, nil)` — 保持 pending 状态
- 恢复: `resumeAgent` 清理 DB checkpoint → SetResumeParams → context.WithoutCancel → ResumeWithParams
- gob 注册: `init()` 中注册自定义类型

### 4. Callback 三层体系

RootRunnerHandlerCallback → RootRunnerCallbacks (DB + WS) → RootRunnerHandler (callbacks.Handler)

### 5. 帧信封严格遵循 API.md

两字段 `{ type, payload }`，request_id 在 payload 内部。服务端帧类型: `connected` / `updates` / `ping` / `pong`。

### 6. thinking/message.delta 走 seq=0 ephemeral

与 eino-demo 一致: 流式 token 不分配 seq，不写 DB。OnCompleted 才持久化。

### 7. 心跳: 客户端发 ping

遵循 API.md: Client → Server 只有 ping。服务端重置超时计时器。

### 8. need_help 走 Update 通道 (seq>0)

持久化，重连后可回放。

### 9. Gap Filling

服务端: FillGaps 独立函数，replayUpdates 和 PollUpdates 复用。
客户端: 检测到 gap → abort 当前批次 → 触发 HTTP pull。

### 10. gorilla/websocket

遵循 CLAUDE.md 指定。注意 API 与 coder/websocket 不同 (Upgrade vs Accept)。

### 11. DOMSnapshot/Screenshot 存文件

SQLite 只存路径。限制 DOM 采集最多 500 个可交互元素。

### 12. INSTR 代替 LIKE

避免通配符注入。

### 13. RunSessionManager (比 atomic.Bool 更完整)

TryStart / Stop (级联) / Done / Cleanup / placeholder session / 保留 checkpoint/resume。

### 14. Token 计数 + 溢出保护

tiktoken-go + ContextInjectionMiddleware + DrainedMessages 重新入队。

### 15. Checkpoint 模型用 TaskID

与 MemBrowser 的任务模型对齐，非 eino-demo 的 ConversationID。

### 16. CORS 精细化

`chrome-extension://*` + `http://localhost:*`。V0.1 允许所有 extension，后续可收紧。

### 17. ErrorCode 参考 OpenMAIC 命名风格

不追求完全一致，MemBrowser 特有码作为扩展。
