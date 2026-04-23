# MemBrowser V0.1 实现计划

> **日期**: 2026-04-23
> **设计文档**: `docs/plans/2026-04-23-membrowser-v01-design.md`
> **参考项目**: `/Users/leichujun/go/src/github.com/PineappleBond/eino-demo-dev/`

---

## 总览

将 V0.1 拆分为 7 个阶段，按依赖顺序执行。每个阶段有明确的交付物和验证标准。

| 阶段 | 内容 | 依赖 |
|------|------|------|
| Phase 0 | 项目脚手架 | 无 |
| Phase 1 | 后端基础 (HTTP + WS + Config + DB) | Phase 0 |
| Phase 2 | WebSocket 协议 (seq 回放 + 心跳) | Phase 1 |
| Phase 3 | Eino Agent + Tools | Phase 2 |
| Phase 4 | 记忆系统 | Phase 3 |
| Phase 5 | Chrome Extension | Phase 2 |
| Phase 6 | 集成测试 + 联调 | Phase 4 + 5 |

---

## Phase 0: 项目脚手架

**目标**: 搭建 Go 后端和 Chrome Extension 的基础目录结构。

### 任务

1. 初始化 Go module: `github.com/PineappleBond/MemBrowser/backend`
2. 创建目录结构:

```
backend/
├── cmd/server/main.go           # 入口 (uber/fx)
├── internal/
│   ├── config/config.go         # 配置 (viper/env)
│   ├── db/db.go                 # SQLite3 + GORM
│   ├── di/module.go             # DI 注册
│   ├── handler/                 # HTTP handlers
│   ├── model/                   # GORM models
│   ├── service/                 # 业务逻辑
│   ├── ws/                      # WebSocket 服务端
│   ├── eino/                    # Agent + Tools
│   │   ├── runner/
│   │   ├── tools/
│   │   └── prompts/
│   └── types/                   # 请求/响应类型
├── configs/config.yaml          # 默认配置
├── go.mod
└── Makefile

extension/
├── manifest.json                # Manifest V3
├── src/
│   ├── background/              # Service Worker
│   ├── sidepanel/               # Side Panel UI
│   ├── content/                 # Content Script (DOM 采集 + 指令执行)
│   └── shared/                  # 共享类型和工具
├── package.json
└── tsconfig.json
```

3. 安装核心依赖:
   - `gorm.io/gorm` + `gorm.io/driver/sqlite` (modernc.org/sqlite, 无 CGO)
   - `github.com/gin-gonic/gin`
   - `github.com/gorilla/websocket` 或 `nhooyr.io/websocket`
   - `github.com/spf13/viper`
   - `go.uber.org/fx`
   - `github.com/google/uuid`
   - Eino 相关包 (参考 eino-demo-dev 的 downloads/ 目录)

4. 编写 Makefile (build, run, test)

### 验证

- `go build ./cmd/server/` 编译通过
- `make run` 启动后监听端口，`GET /health` 返回 200

---

## Phase 1: 后端基础

**目标**: HTTP 服务、配置系统、数据库初始化跑通。

### 任务

#### 1.1 配置系统

参考 eino-demo 的 `config.go`，用 viper + env 实现:

```go
// backend/internal/config/config.go
type Config struct {
    Port     int    `mapstructure:"PORT"`
    DBPath   string `mapstructure:"DB_PATH"`
    WSPath   string `mapstructure:"WS_PATH"`     // ws 路径
    AuthKey  string `mapstructure:"AUTH_KEY"`     // 认证 key

    // 分级模型配置
    Haiku  ModelConfig  // 快速模型
    Sonnet ModelConfig  // 平衡模型
    Opus   ModelConfig  // 推理模型
}

type ModelConfig struct {
    BaseURL string `mapstructure:"BASE_URL"`
    APIKey  string `mapstructure:"API_KEY"`
    Model   string `mapstructure:"MODEL"`
}
```

环境变量前缀: `MEMBROWSER_`，如 `MEMBROWSER_HAIKU_BASE_URL`。

#### 1.2 数据库 (SQLite3 + GORM)

参考 eino-demo 的 `db.go`，但用 SQLite3:

```go
// backend/internal/db/db.go
func NewDB(cfg *config.Config) (*gorm.DB, error) {
    db, err := gorm.Open(sqlite.Open(cfg.DBPath), &gorm.Config{})
    // AutoMigrate
    db.AutoMigrate(&model.Memory{}, &model.Session{}, &model.Update{})
    return db, nil
}
```

GORM Models:

```go
// backend/internal/model/memory.go
type Memory struct {
    ID             string    `gorm:"primaryKey"`
    SessionID      string    `gorm:"index"`
    PageURL        string
    PageURLPattern string    `gorm:"index"`
    PageTitle      string
    PageFeatures   string
    DOMSnapshot    string
    Screenshot     []byte
    ActionType     string
    ActionTarget   string
    ActionSelector string
    ActionValue    string
    Result         string
    FailReason     string
    Source         string    // human / ai / memory
    CreatedAt      time.Time
}

// backend/internal/model/update.go
type Update struct {
    ID        string         `gorm:"primaryKey"`
    Seq       int64          `gorm:"uniqueIndex"`
    Type      string
    Payload   model.JSONMap  // 自定义 JSONB 类型
    CreatedAt time.Time
}
```

**注意**: MemBrowser 是单用户，不需要 users 表。seq 用 SQLite 自增实现 (不依赖 Redis)。

#### 1.3 Seq 机制 (无 Redis)

V0.1 是单实例，seq 用内存原子计数 + SQLite 持久化:

```go
// backend/internal/ws/seq.go
type SeqManager struct {
    db    *gorm.DB
    mu    sync.Mutex
    maxSeq int64
}

func (s *SeqManager) Next() (int64, error) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.maxSeq++
    // 持久化到 DB (写 Update 记录时一起写)
    return s.maxSeq, nil
}

func (s *SeqManager) Max() int64 {
    return atomic.LoadInt64(&s.maxSeq)
}
```

#### 1.4 HTTP 路由

参考 eino-demo 的 handler 注册模式:

```go
// backend/internal/handler/router.go
func RegisterRoutes(r *gin.Engine, h *Handler) {
    v1 := r.Group("/api/v1")
    {
        v1.POST("/page/state", h.UploadPageState)
        v1.POST("/action/result", h.UploadActionResult)
        v1.POST("/teach", h.UploadTeachData)
        v1.GET("/updates", h.PollUpdates)     // 轮询降级
    }
    r.GET("/health", h.Health)
}
```

#### 1.5 DI 容器

参考 eino-demo 的 `di/module.go`，用 uber/fx:

```go
// backend/internal/di/module.go
var Module = fx.Options(
    fx.Provide(config.Load),
    fx.Provide(db.NewDB),
    fx.Provide(ws.NewManager),
    fx.Provide(handler.NewHandler),
    fx.Invoke(handler.RegisterRoutes),
)
```

### 验证

- 配置从环境变量正确加载
- SQLite 数据库文件创建，表结构正确
- `POST /api/v1/page/state` 接收 JSON 返回 200
- `GET /api/v1/updates?last_seq=0` 返回空数组

---

## Phase 2: WebSocket 协议

**目标**: WS 连接、心跳、seq 回放跑通。

### 任务

#### 2.1 WS 协议定义

参考 eino-demo 的 `ws/protocol.go`:

```go
// backend/internal/ws/protocol.go
const (
    FrameConnected   = "connected"
    FramePing        = "ping"
    FramePong        = "pong"
    FrameUpdate      = "update"
    FrameUpdateBatch = "update_batch"
)

type Frame struct {
    Type    string          `json:"type"`
    Payload json.RawMessage `json:"payload"`
}

type ConnectedPayload struct {
    ServerTime time.Time `json:"server_time"`
    MaxSeq     int64     `json:"max_seq"`
}

type Update struct {
    Seq     int64           `json:"seq"`
    Type    string          `json:"type"`
    ID      string          `json:"id"`
    Payload json.RawMessage `json:"payload"`
}
```

#### 2.2 WS 连接管理

参考 eino-demo 的 `ws/manager.go`，但简化为单用户:

```go
// backend/internal/ws/manager.go
type Manager struct {
    conn     *websocket.Conn
    mu       sync.RWMutex
    seqMgr   *SeqManager
    sendCh   chan *Frame
    onMessage func(Update)  // 处理 Extension 上报的消息
}

func (m *Manager) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // 1. 验证 auth key (从 query param 或 header)
    // 2. Accept WebSocket upgrade
    // 3. 发送 connected frame { max_seq }
    // 4. 如果 last_seq < max_seq，回放丢失的更新
    // 5. 启动 readLoop + writeLoop
}

func (m *Manager) PushUpdate(u Update) {
    // 写入 DB (seq > 0) + 推送到 Extension
}
```

#### 2.3 心跳

- 后端每 30s 发 ping
- Extension 收到后回 pong
- 60s 未收到 pong → 断开，等待重连

#### 2.4 重连回放

- Extension 连接时带 `?last_seq=N`
- 后端查 `updates WHERE seq > N ORDER BY seq ASC LIMIT 500`
- 通过 `update_batch` frame 一次性推送

#### 2.5 HTTP 轮询降级

```go
// GET /api/v1/updates?last_seq=N
func (h *Handler) PollUpdates(c *gin.Context) {
    lastSeq := c.QueryInt64("last_seq")
    updates := h.updateService.GetSince(lastSeq)
    c.JSON(200, gin.H{"updates": updates, "max_seq": h.seqMgr.Max()})
}
```

### 验证

- WS 连接后收到 `connected` frame
- 后端推送 update 后，Extension 通过 WS 收到
- 断开重连后，`last_seq` 之后的 update 被回放
- HTTP 轮询返回相同的 update 数据

---

## Phase 3: Eino Agent + Tools

**目标**: Eino Agent 能接收自然语言任务，通过 Tools 与 Extension 交互。

### 任务

#### 3.1 模型提供者

参考 eino-demo 的 `eino/model.go`:

```go
// backend/internal/eino/model.go
type ModelProvider struct {
    cfg config.ModelConfig
}

func (m *ModelProvider) GetModel(tier string) (*openai.ChatModel, error) {
    // 根据 tier (haiku/sonnet/opus) 返回对应的 ChatModel
}
```

#### 3.2 Agent Tools 定义

参考 eino-demo 的 `eino/tools/registry.go`，用 `utils.InferTool`:

```go
// backend/internal/eino/tools/get_page_state.go
type GetPageStateInput struct {
    IncludeScreenshot bool `json:"include_screenshot"`
}

type GetPageStateOutput struct {
    URL      string     `json:"url"`
    Title    string     `json:"title"`
    DOMTree  []DOMNode  `json:"dom_tree"`
}

// 通过 WS 向 Extension 发送 page.query，等待 HTTP 回调
func (t *GetPageStateTool) Run(ctx context.Context, input GetPageStateInput) (GetPageStateOutput, error)
```

```go
// backend/internal/eino/tools/execute_action.go
type ExecuteActionInput struct {
    Action   string `json:"action"`    // click / input / scroll / navigate
    Selector string `json:"selector"`  // DOM 选择器
    Value    string `json:"value"`     // 输入值 (可选)
}

// 通过 WS 向 Extension 发送 action.execute，等待 HTTP 回调
func (t *ExecuteActionTool) Run(ctx context.Context, input ExecuteActionInput) (ExecuteActionResult, error)
```

```go
// backend/internal/eino/tools/search_memory.go
type SearchMemoryInput struct {
    PageURL  string `json:"page_url"`
    Action   string `json:"action_type"`
    Target   string `json:"action_target"`
}

// 查 SQLite 记忆库
func (t *SearchMemoryTool) Run(ctx context.Context, input SearchMemoryInput) (SearchMemoryOutput, error)
```

```go
// backend/internal/eino/tools/save_memory.go
type SaveMemoryInput struct {
    PageURL  string `json:"page_url"`
    Action   string `json:"action_type"`
    Selector string `json:"action_selector"`
    Value    string `json:"value"`
    Result   string `json:"result"`
}

func (t *SaveMemoryTool) Run(ctx context.Context, input SaveMemoryInput) (SaveMemoryOutput, error)
```

```go
// backend/internal/eino/tools/ask_human.go
type AskHumanInput struct {
    Message         string `json:"message"`
    HighlightSelector string `json:"highlight_selector"`
}

// 使用 eino 的 tool.Interrupt() 暂停 Agent，等待人类示教
func (t *AskHumanTool) Run(ctx context.Context, input AskHumanInput) (AskHumanOutput, error)
```

#### 3.3 Tool 注册

```go
// backend/internal/eino/tools/registry.go
type Registry struct {
    tools []tool.BaseTool
}

func NewRegistry(wsMgr *ws.Manager, memStore *memory.Store) *Registry {
    r := &Registry{}
    r.tools = append(r.tools,
        utils.InferTool("get_page_state", "...", NewGetPageStateTool(wsMgr).Run),
        utils.InferTool("execute_action", "...", NewExecuteActionTool(wsMgr).Run),
        utils.InferTool("search_memory", "...", NewSearchMemoryTool(memStore).Run),
        utils.InferTool("save_memory", "...", NewSaveMemoryTool(memStore).Run),
        utils.InferTool("ask_human", "...", NewAskHumanTool(wsMgr).Run),
    )
    return r
}
```

#### 3.4 Agent Runner

参考 eino-demo 的 `eino/runner/rootrunner.go`:

```go
// backend/internal/eino/runner/runner.go
func NewRunner(cfg RunnerConfig) (*Runner, error) {
    model := cfg.ModelProvider.GetModel(cfg.Tier)
    tools := cfg.ToolRegistry.GetAll()

    agent := deep.New().
        WithModel(model).
        WithTools(tools...).
        WithInstruction(cfg.SystemPrompt).
        WithMaxIteration(20)

    return &Runner{agent: agent}, nil
}
```

#### 3.5 System Prompt

```go
// backend/internal/eino/prompts/templates/agent.tmpl
`你是 MemBrowser，一个 Web 自动化助手。

你的任务是帮用户自动执行 Web 操作。

## 工作流程
1. 用 get_page_state() 观察当前页面
2. 用 search_memory() 查找是否有已知的操作路径
3. 如果有记忆命中，直接用 execute_action() 执行
4. 如果没有记忆，用 AI 分析 DOM 决定下一步操作
5. 如果 AI 无法确定，用 ask_human() 请求用户帮助
6. 每次操作成功后，用 save_memory() 保存经验

## 规则
- 每次只执行一个操作，然后观察结果
- 不要预规划多个步骤
- 操作失败时记录失败原因，尝试其他方案
- 连续失败 3 次后必须 ask_human()
`
```

#### 3.6 Agent 执行服务

```go
// backend/internal/service/task.go
type TaskService struct {
    runner   *eino.Runner
    wsMgr    *ws.Manager
    seqMgr   *ws.SeqManager
}

func (s *TaskService) StartTask(taskDesc string) error {
    // 1. 推送 task.started update
    // 2. 运行 Agent: s.runner.Run(ctx, messages)
    // 3. 消费 Agent 事件流，推送 task.step / task.completed / task.failed
    // 4. Agent 通过 Tools 与 Extension 交互
}
```

### 验证

- Agent 能接收任务描述并开始执行
- get_page_state() 能通过 WS 向 Extension 发送查询
- execute_action() 能通过 WS 向 Extension 发送执行指令
- search_memory() 能从 SQLite 查询记忆
- ask_human() 能暂停 Agent 并推送 need_help

---

## Phase 4: 记忆系统

**目标**: 记忆的存储、匹配、人类示教流程完整可用。

### 任务

#### 4.1 记忆存储服务

```go
// backend/internal/service/memory.go
type MemoryStore struct {
    db *gorm.DB
}

func (s *MemoryStore) Save(m *model.Memory) error
func (s *MemoryStore) Search(pageURL, actionType, actionTarget string) (*model.Memory, error)
func (s *MemoryStore) NormalizeURL(url string) string
```

#### 4.2 URL 标准化

```go
// backend/internal/service/url_normalize.go
func NormalizeURL(rawURL string) string {
    // /product/12345 → /product/{id}
    // /user/abc-def/profile → /user/{id}/profile
    // 用正则替换纯数字/UUID 路径段
}
```

#### 4.3 人类示教流程

```go
// backend/internal/handler/teach.go
func (h *Handler) UploadTeach(c *gin.Context) {
    // 1. 接收 Extension 上报的示教数据
    //    - element: DOM 选择器 + 语义描述
    //    - action: click/input/...
    //    - screenshot: 截图 (可选)
    //    - session_id: 当前会话
    // 2. 构造 Memory 记录 (source=human, result=success)
    // 3. 存入 SQLite
    // 4. 推送 task.step update (human_teach saved)
    // 5. 通知 Agent 继续执行 (通过 channel/callback)
}
```

#### 4.4 记忆命中后的自动复用

Agent 的 search_memory Tool 命中后，直接用记忆中的 selector + action 执行，不调用 AI。

### 验证

- 人类示教后，记忆正确存入 SQLite
- 相同页面 + 相同操作，search_memory 能命中
- 记忆命中后，Agent 直接复用，不调用 AI (0 Token)
- URL 标准化正确: `/product/123` → `/product/{id}`

---

## Phase 5: Chrome Extension

**目标**: Extension 能连接后端、采集 DOM、执行操作、人类示教。

### 任务

#### 5.1 Manifest V3 配置

```json
{
  "manifest_version": 3,
  "name": "MemBrowser",
  "permissions": ["sidePanel", "activeTab", "scripting"],
  "side_panel": { "default_path": "sidepanel.html" },
  "background": { "service_worker": "background.js" },
  "content_scripts": [{ "matches": ["<all_urls>"], "js": ["content.js"] }]
}
```

#### 5.2 Content Script — DOM 采集

```typescript
// extension/src/content/dom-collector.ts
interface DOMNode {
  tag: string;
  id: string;           // 唯一标识
  text: string;         // 文本内容
  attributes: Record<string, string>;  // 语义属性
  interactable: boolean;
  children: DOMNode[];
}

function collectDOM(): DOMNode {
  // 递归遍历 DOM 树
  // 只采集可交互元素 (input, button, a, select, textarea)
  // 保留 placeholder, aria-label, text content
  // 去掉样式、脚本
  // 生成精简后的语义树
}
```

#### 5.3 Content Script — 指令执行

```typescript
// extension/src/content/action-executor.ts
function executeAction(action: {
  type: 'click' | 'input' | 'scroll' | 'navigate';
  selector: string;
  value?: string;
}): { success: boolean; error?: string } {
  const el = document.querySelector(action.selector);
  switch (action.type) {
    case 'click': el.click(); break;
    case 'input': (el as HTMLInputElement).value = action.value; break;
    case 'scroll': el.scrollIntoView(); break;
    case 'navigate': window.location.href = action.value; break;
  }
}
```

#### 5.4 Content Script — 人类示教 UI

```typescript
// extension/src/content/teaching-ui.ts
function showHighlight(selector: string, message: string) {
  // 1. 找到目标元素
  // 2. 添加高亮边框 (amber + pulse animation)
  // 3. 显示提示浮层
  // 4. 等待用户点击
  // 5. 捕获点击事件，记录 selector + action
  // 6. 通过 HTTP 上报到后端
}
```

#### 5.5 Background Service Worker — WS 客户端

```typescript
// extension/src/background/ws-client.ts
class WSClient {
  private ws: WebSocket;
  private lastSeq: number = 0;

  connect(url: string, authKey: string) {
    this.ws = new WebSocket(`${url}?token=${authKey}&last_seq=${this.lastSeq}`);
    this.ws.onmessage = (e) => this.handleFrame(JSON.parse(e.data));
  }

  private handleFrame(frame: Frame) {
    switch (frame.type) {
      case 'connected': this.lastSeq = frame.payload.max_seq; break;
      case 'update': this.handleUpdate(frame.payload); break;
      case 'update_batch': frame.payload.forEach(u => this.handleUpdate(u)); break;
      case 'ping': this.send({ type: 'pong' }); break;
    }
  }

  private handleUpdate(update: Update) {
    switch (update.type) {
      case 'page.query': this.queryPage(update); break;
      case 'action.execute': this.executeAction(update); break;
      case 'need_help': this.showHelp(update); break;
      case 'task.step': this.updateUI(update); break;
    }
    this.lastSeq = update.seq;
  }
}
```

#### 5.6 Side Panel UI

参考 `docs/prototype/extension-mockup.html` 的设计，用 TypeScript 实现:

- 模型选择 (Haiku / Sonnet / Opus)
- 任务输入框
- 执行进度列表
- 人类示教提示
- 连接状态
- 设置 (折叠式): WebSocket URL + Auth Key

### 验证

- Extension 安装后，Side Panel 显示正确 UI
- Content Script 能采集当前页面的 DOM 语义树
- WS 连接成功，收到 `connected` frame
- 后端发送 `page.query`，Extension 返回 DOM 数据
- 后端发送 `action.execute`，Extension 在页面上执行操作
- 人类示教时，高亮元素 → 用户点击 → 数据上报后端

---

## Phase 6: 集成测试 + 联调

**目标**: 完整流程跑通——用户输入任务，Agent 自动执行，记忆复现生效。

### 任务

#### 6.1 端到端流程测试

1. 启动后端 (`make run`)
2. 安装 Extension 到 Chrome
3. 打开一个测试页面 (如本地模拟的表单页)
4. 在 Side Panel 输入任务: "填写表单并提交"
5. 观察 Agent 执行:
   - get_page_state() → 获取 DOM
   - AI 分析 DOM → 决定操作
   - execute_action() → Extension 执行
   - 循环直到完成
6. 再次执行相同任务，验证记忆命中 (0 Token)

#### 6.2 人类示教流程测试

1. 输入一个 Agent 无法完成的任务
2. Agent 触发 ask_human()
3. Extension 高亮目标元素
4. 用户手动点击
5. 教导数据存入记忆
6. Agent 继续执行
7. 再次遇到相同场景，记忆命中

#### 6.3 重连回放测试

1. 执行任务过程中，手动断开 WS
2. Extension 自动重连
3. 验证断连期间的 update 被回放
4. 任务继续执行

### 验证

- 完整任务流程跑通
- 记忆复现生效 (第二次执行 0 Token)
- 人类示教后记忆正确保存
- WS 断连重连后任务不丢失

---

## 关键技术决策

### 1. Seq 机制: 无 Redis

V0.1 单实例，用内存原子计数 + SQLite 持久化:
- `SeqManager` 内存中维护 `maxSeq`
- 每次 `Next()` 原子递增
- 启动时从 SQLite `SELECT MAX(seq) FROM updates` 恢复
- 写 Update 记录时 seq 和业务数据在同一事务中

### 2. Tool 与 Extension 的交互模式

Agent 调用 Tool → Tool 通过 WS 发送指令 → Extension 执行 → Extension 通过 HTTP 上报结果 → Tool 返回

这需要一个同步等待机制:

```go
// backend/internal/eino/tools/waiter.go
type ResponseWaiter struct {
    pending map[string]chan json.RawMessage
    mu      sync.RWMutex
}

func (w *ResponseWaiter) Wait(id string, timeout time.Duration) (json.RawMessage, error) {
    ch := make(chan json.RawMessage, 1)
    w.pending[id] = ch
    select {
    case resp := <-ch: return resp, nil
    case <-time.After(timeout): return nil, errors.New("timeout")
    }
}

func (w *ResponseWaiter) Resolve(id string, data json.RawMessage) {
    if ch, ok := w.pending[id]; ok {
        ch <- data
        delete(w.pending, id)
    }
}
```

### 3. 记忆匹配: 纯 SQL

不做 RAG，用 SQL WHERE 子句匹配:
- `page_url_pattern` 精确匹配
- `action_type` 精确匹配
- `action_target` LIKE 模糊匹配
- `result = 'success'` 过滤
- `ORDER BY created_at DESC LIMIT 1` 取最近的

### 4. Agent 会话管理

参考 eino-demo 的 `RunSessionMgr`:
- 同时只能有一个 Agent 在运行
- 用 atomic CAS 防止并发
- Agent 运行超时 10 分钟
