# MemBrowser V0.1 设计文档

> **日期**: 2026-04-23
> **状态**: 已确认
> **版本**: V0.1

---

## 1. 项目概述

**MemBrowser** 是一个开源的"人在回路（Human-in-the-loop）"Web Agent 框架。它结合 AI 视觉理解与多模态经验记忆，用于自动化重复性的 Web 操作。

**核心理念**: 不追求 AI 完美，而是用人类示教作为可靠的降级方案，通过记忆复现让成功路径的成本趋近于零。

### 关键决策

| 决策项 | 选择 |
|--------|------|
| 核心价值 | 人类示教降级 + 记忆复现降本 |
| 目标用户 (V0.1) | 非技术业务人员 |
| 交互方式 | 自然语言驱动 |
| 架构 | Chrome Extension + Go 后端 |
| 通信方式 | WS (后端推送) + HTTP (Extension 上传) |
| Agent 框架 | Eino (字节跳动开源) |
| AI 模型策略 | 分级 (类 Haiku/Sonnet/Opus)，OpenAI 兼容协议，自定义 base URL |
| 存储 | 纯本地 SQLite3 (无 CGO) + GORM |
| 页面理解 | DOM 优先，人类示教时上传图片作为补充 |
| 部署方式 (V0.1) | 单可执行文件 |

---

## 2. 系统架构

```
┌─────────────────────┐    WS (推送)     ┌──────────────────────┐
│   Chrome Extension   │ ◄────────────── │    Go 后端            │
│                      │                 │    (Eino Agent)      │
│  · DOM 采集器        │    HTTP (上传)   │                      │
│  · 指令执行器        │ ──────────────► │  · 任务编排引擎       │
│  · 人类示教 UI       │                 │  · 记忆存储           │
│  · 侧边栏            │                 │  · AI 模型调用        │
│  · WS 客户端         │                 │  · WS 服务端          │
│  · HTTP 客户端       │                 │  · HTTP 服务端        │
└─────────────────────┘                 └──────────────────────┘
```

### Extension 职责 (瘦前端)

- 采集页面 DOM 结构 (精简后的语义树，非原始 HTML)
- 接收后端指令并执行 (click, input, scroll, navigate)
- 提供人类示教的交互 UI (高亮元素、录制操作、上传截图)
- 展示任务执行状态和进度
- WebSocket 客户端，接收后端推送
- HTTP 客户端，上传页面状态和执行结果

### Go 后端职责 (胖后端)

- 接收自然语言任务描述
- Eino Agent 编排，通过 Tools 驱动执行循环
- 记忆管理 (SQLite3 + GORM)
- AI 模型调用 (分级，OpenAI 兼容)
- 任务状态管理和会话上下文
- WebSocket 服务端，推送指令给 Extension
- HTTP 服务端，接收 Extension 上传的数据

---

## 3. 任务执行循环

后端是唯一大脑。Eino Agent 驱动响应式执行循环——不做预规划，每一步都观察当前状态再决定下一步。

```
用户: "把这个商品上架到淘宝"
        │
        ▼
┌─ Eino Agent 接收任务 ───────────────────────────┐
│                                                   │
│  Agent 拥有以下 Tools:                             │
│                                                   │
│  · get_page_state()                               │
│    → 通过 HTTP 请求 Extension，获取当前             │
│      DOM + URL + 页面标题                          │
│                                                   │
│  · search_memory(pageContext)                     │
│    → 在记忆库中查找相似页面的成功操作               │
│                                                   │
│  · execute_action(actionType, target, value)      │
│    → 通过 WS 推送指令给 Extension 执行             │
│                                                   │
│  · save_memory(pageContext, action, result)        │
│    → 将成功操作存入记忆库                           │
│                                                   │
│  · ask_human(question)                            │
│    → 通过 WS 推送"需要帮助"信号给 Extension         │
│    → 通过 HTTP 接收人类示教数据                     │
│    → 示教完成后自动存入记忆库                       │
│                                                   │
│  · call_ai_model(prompt, pageState)               │
│    → 调用分级模型进行推理                           │
│                                                   │
│  Agent 循环:                                      │
│  1. get_page_state() — 观察当前状态                │
│  2. 决定下一步操作 (记忆 / AI / 人类)              │
│  3. execute_action() — 推送给 Extension 执行       │
│  4. Extension 执行后，通过 HTTP 上报结果            │
│  5. 回到第 1 步                                    │
│  6. 循环直到任务完成或失败                          │
└───────────────────────────────────────────────────┘
```

### 关键设计点

- **响应式循环**: 不做预规划，每一步都是"观察 → 决策 → 执行 → 观察"
- **Tool 化**: 记忆、AI、人类示教都是 Agent 自主调用的 Tool
- **每步都查记忆**: Agent 自主决定何时查记忆，不是硬编码的流程
- **失败操作也存储**: 用于会话内避免重复失败，跨会话复用留到后续版本

---

## 4. 记忆系统

### 存储

- **数据库**: SQLite3 (无 CGO) + GORM
- **位置**: 本地文件系统
- **V0.1 不做 RAG**: 纯 SQL 匹配，向量检索留到后续版本

### 数据模型

```sql
CREATE TABLE memories (
    id              TEXT PRIMARY KEY,
    session_id      TEXT NOT NULL,

    -- 页面上下文
    page_url        TEXT NOT NULL,
    page_url_pattern TEXT,          -- 标准化后的 URL 模式 (如 /product/{id})
    page_title      TEXT,
    page_features   TEXT,           -- 页面特征指纹，用于匹配
    dom_snapshot    TEXT,           -- 精简后的 DOM 片段
    screenshot      BLOB,          -- 截图 (来自人类示教)

    -- 操作
    action_type     TEXT NOT NULL,  -- click / input / scroll / navigate / select
    action_target   TEXT,           -- 元素的语义描述
    action_selector TEXT,           -- DOM 选择器 (备用定位)
    action_value    TEXT,           -- 输入值 (如有)

    -- 结果
    result          TEXT NOT NULL,  -- success / fail
    fail_reason     TEXT,           -- 失败原因

    -- 元数据
    source          TEXT NOT NULL,  -- human / ai / memory
    created_at      DATETIME NOT NULL
);

CREATE INDEX idx_memories_url ON memories(page_url_pattern);
CREATE INDEX idx_memories_session ON memories(session_id);
```

### 匹配策略 (纯 SQL)

```
1. 将当前 URL 标准化为模式:
   /product/12345 → /product/{id}

2. 查询:
   SELECT * FROM memories
   WHERE page_url_pattern = '/product/{id}'
   AND action_type = 'click'
   AND action_target LIKE '%发布%'
   AND result = 'success'
   ORDER BY created_at DESC
   LIMIT 1

3. 命中 → 直接复用
4. 未命中 → Agent 走 AI 推理
```

### 什么时候考虑 RAG

- 记忆库达到数千条以上
- 需要跨网站泛化相似操作 (如在 A 网站填地址的经验迁移到 B 网站)
- 到那时加向量检索作为辅助层，SQL 仍然是主匹配方式

---

## 5. Chrome Extension 设计

### UI 组件

```
┌─ Chrome 浏览器 ──────────────────────────────────┐
│                                                     │
│  ┌─ 侧边栏 (Side Panel) ───────────────────────┐  │
│  │                                               │  │
│  │  [模型: Haiku | Sonnet | Opus]               │  │
│  │                                               │  │
│  │  任务输入:                                    │  │
│  │  ┌──────────────────────────────────┐         │  │
│  │  │ 描述你想自动化的任务...            │ [→]     │  │
│  │  └──────────────────────────────────┘         │  │
│  │                                               │  │
│  │  ─────────────────────────────────            │  │
│  │                                               │  │
│  │  执行进度:                                    │  │
│  │  ✓ 点击"发布商品"按钮      [记忆命中]          │  │
│  │  ✓ 等待页面加载完成        [自动检测]          │  │
│  │  ● 填写商品标题            [AI: sonnet]        │  │
│  │  ○ 填写商品描述            [待执行]            │  │
│  │  ○ 设置价格                [待执行]            │  │
│  │                                               │  │
│  │  ┌────────────────────────────────────┐       │  │
│  │  │ ⚠ 需要你的帮助                    │       │  │
│  │  │ AI 找不到"价格输入框"，             │       │  │
│  │  │ 请手动点击一次。                    │       │  │
│  │  │              [开始示教]              │       │  │
│  │  └────────────────────────────────────┘       │  │
│  │                                               │  │
│  ─────────────────────────────────────────────    │  │
│  │ v0.1.0              设置 | 帮助               │  │
│  └───────────────────────────────────────────────┘  │
│                                                     │
│  ┌─ 页面内容 ───────────────────────────────────┐  │
│  │                                               │  │
│  │  (人类示教时:)                                 │  │
│  │  ┌ ─ ─ ─ ─ ─ ─ ─ ─ ┐                        │  │
│  │  │  高亮目标元素    │ ← 琥珀色边框 +           │  │
│  │  │                 │   脉冲动画               │  │
│  │  └ ─ ─ ─ ─ ─ ─ ─ ─┘                        │  │
│  │  "请点击价格输入框"                            │  │
│  │                                               │  │
│  └───────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────┘
```

### 核心模块

| 模块 | 职责 |
|------|------|
| DOM 采集器 | 采集精简后的 DOM 语义树 (标签、文本、属性、可交互状态) |
| 指令执行器 | 接收后端指令，执行 click/input/scroll/navigate |
| 人类示教 UI | 高亮 + 遮罩 + 提示，等待用户操作，捕获结果 |
| WebSocket 客户端 | 与后端保持长连接，接收推送 |
| HTTP 客户端 | 上传页面状态、执行结果、人类示教数据 |
| 侧边栏 | 任务输入、状态展示、连接状态、设置 (折叠式) |

### DOM 采集策略

- 只采集可交互元素 (input, button, a, select, textarea 等)
- 保留语义属性 (placeholder, aria-label, text content)
- 去掉样式、脚本等无关信息
- 生成树形结构，每个节点带唯一标识 (用于后续定位)

---

## 6. 通信协议

### 设计原则

遵循 IM 系统的模式:
- **WebSocket**: 只用于后端 → Extension 推送 (指令、状态更新)
- **HTTP**: 只用于 Extension → 后端上传 (页面状态、执行结果、人类示教数据)

### WebSocket 消息 (后端 → Extension)

所有消息遵循 Update 模式，带 `seq` 字段:

```json
{
  "seq": 42,
  "type": "消息类型",
  "id": "uuid",
  "payload": { ... }
}
```

| type | seq | 说明 | payload |
|------|-----|------|---------|
| `connected` | - | WS 握手完成 | `{ server_time, max_seq }` |
| `ping` | - | 心跳 | `{}` |
| `page.query` | 0 | 请求页面状态 | `{ include_screenshot }` |
| `action.execute` | >0 | 执行操作指令 | `{ action, selector, value? }` |
| `task.started` | >0 | 任务开始 | `{ task_id, description }` |
| `task.step` | >0 | 步骤更新 | `{ step_id, status, message }` |
| `task.completed` | >0 | 任务完成 | `{ task_id, summary }` |
| `task.failed` | >0 | 任务失败 | `{ task_id, error }` |
| `need_help` | >0 | 请求人类帮助 | `{ message, highlight_selector }` |
| `thinking` | 0 | AI 推理过程 (流式) | `{ delta }` |

### HTTP 接口 (Extension → 后端)

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/page/state` | 上传当前页面状态 (DOM + URL + 标题) |
| POST | `/api/v1/action/result` | 上传操作执行结果 |
| POST | `/api/v1/teach` | 提交人类示教数据 (元素 + 操作 + 截图) |
| GET | `/api/v1/updates?last_seq=N` | 轮询降级，补发丢失的推送 |

### 连接生命周期

```
1. Extension 启动 → 连接 ws://host:port/ws?token=<key>&last_seq=<N>
2. 后端验证 auth，返回 { type: "connected", payload: { max_seq } }
3. 如果服务端 max_seq > 本地 last_seq → HTTP 轮询补发
4. 心跳: Extension 每 30s 发 ping，后端 60s 内未收到则断开
5. 断连后 → 自动重连，指数退避 (1s → 2s → 4s → 30s 上限)
6. 重连后 → 踢掉旧连接，用 last_seq 重新同步
```

---

## 7. AI 模型策略

### 分级模型

| 级别 | 使用场景 | 特点 |
|------|---------|------|
| Haiku | 简单页面匹配、意图解析 | 快、便宜 |
| Sonnet | DOM 分析、操作生成 | 平衡 |
| Opus | 复杂多步推理 | 慢、贵 |

### API 兼容性

- OpenAI 兼容协议 (`/v1/chat/completions`)
- 支持自定义 `base_url` (适配 Azure、本地 LLM 等)
- 每步可选模型级别: Agent 根据复杂度决定用哪个级别

---

## 8. 技术栈

| 组件 | 技术 |
|------|------|
| 后端语言 | Go |
| Agent 框架 | Eino (字节跳动) |
| 数据库 | SQLite3 (无 CGO) via modernc.org/sqlite |
| ORM | GORM |
| WebSocket | gorilla/websocket 或 nhooyr/websocket |
| HTTP 路由 | gin 或 chi |
| Chrome Extension | Manifest V3, TypeScript |
| Extension UI | Side Panel API |
| AI API | OpenAI 兼容协议 |

---

## 9. V0.1 范围

### 包含

- Chrome Extension + Side Panel UI
- Go 后端 + Eino Agent
- 自然语言任务输入
- DOM 优先的页面理解
- 记忆系统 (SQLite3, SQL 匹配)
- 人类示教流程
- 分级模型支持 (OpenAI 兼容)
- WebSocket 通信 + seq 回放
- 单可执行文件部署

### 不包含 (延后)

- 云端部署 / 多用户
- RAG / 向量检索
- 视觉理解 (VLM)
- 跨会话失败操作复用
- Plugin API
- Docker / 安装包

---

## 10. 参考资料

- BP: `docs/BP.md` (Gemini 生成的商业计划书)
- 原型: `docs/prototype/extension-mockup.html`
- IM 协议参考: `/Users/leichujun/go/src/github.com/PineappleBond/eino-demo-dev/docs/API.md`
