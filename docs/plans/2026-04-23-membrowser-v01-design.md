# MemBrowser V0.1 Design Document

> **Date**: 2026-04-23
> **Status**: Approved
> **Version**: V0.1

---

## 1. Project Overview

**MemBrowser** is an open-source "Human-in-the-loop" Web Agent framework. It combines AI vision understanding with multimodal experience memory to automate repetitive Web tasks.

**Core Philosophy**: Don't pursue AI perfection. Use human demonstration as a reliable fallback, and build up a memory of successful executions that can be replayed at near-zero cost.

### Key Decisions

| Decision | Choice |
|----------|--------|
| Core value | Human-in-the-loop fallback + memory replay for cost reduction |
| Target users (V0.1) | Non-technical business users |
| Interaction model | Natural language driven |
| Architecture | Chrome Extension + Go backend |
| Communication | WS (backend push) + HTTP (extension upload) |
| Agent framework | Eino (ByteDance open-source) |
| AI model strategy | Tiered (Haiku/Sonnet/Opus-like), OpenAI-compatible protocol, custom base URL |
| Storage | Pure local SQLite3 (no CGO) + GORM |
| Page understanding | DOM-first, human teaching with images as supplement |
| Deployment (V0.1) | Single executable file |

---

## 2. System Architecture

```
┌─────────────────────┐    WS (push)     ┌──────────────────────┐
│   Chrome Extension   │ ◄─────────────── │    Go Backend        │
│                      │                  │    (Eino Agent)      │
│  · DOM Collector     │    HTTP (upload)  │                      │
│  · Action Executor   │ ───────────────► │  · Task Orchestrator │
│  · Human-in-loop UI  │                  │  · Memory Store      │
│  · Side Panel        │                  │  · AI Model Client   │
│  · WS Client         │                  │  · WS Server         │
│  · HTTP Client       │                  │  · HTTP Server       │
└─────────────────────┘                  └──────────────────────┘
```

### Extension Responsibilities (Thin Frontend)

- Collect page DOM structure (simplified semantic tree, not raw HTML)
- Receive backend instructions and execute (click, input, scroll, navigate)
- Provide human-in-the-loop interaction UI (highlight elements, record operations, upload screenshots)
- Display task execution status and progress
- WebSocket client for receiving backend pushes
- HTTP client for uploading page state and results

### Go Backend Responsibilities (Fat Backend)

- Receive natural language task descriptions
- Eino Agent orchestration with tools
- Memory management (SQLite3 + GORM)
- AI model calls (tiered, OpenAI-compatible)
- Task state management and session context
- WebSocket server for pushing instructions
- HTTP server for receiving extension uploads

---

## 3. Task Execution Loop

The backend is the only brain. The Eino Agent drives the execution loop reactively — no pre-planning, each step observes the current state and decides the next action.

```
User: "Publish this product on Taobao"
        │
        ▼
┌─ Eino Agent receives task ──────────────────────┐
│                                                   │
│  Agent has the following Tools:                    │
│                                                   │
│  · get_page_state()                               │
│    → Requests Extension via HTTP to return         │
│      current DOM + URL + title                     │
│                                                   │
│  · search_memory(pageContext)                     │
│    → Searches memory store for similar pages       │
│      and successful operations                     │
│                                                   │
│  · execute_action(actionType, target, value)      │
│    → Pushes instruction to Extension via WS        │
│                                                   │
│  · save_memory(pageContext, action, result)        │
│    → Saves successful operation to memory store    │
│                                                   │
│  · ask_human(question)                            │
│    → Pushes "need_help" to Extension via WS        │
│    → Waits for human teach data via HTTP           │
│    → Auto-saves to memory after human teaches      │
│                                                   │
│  · call_ai_model(prompt, pageState)               │
│    → Calls tiered model for inference              │
│                                                   │
│  Agent loop:                                      │
│  1. get_page_state() — observe current state       │
│  2. Decide next action (memory / AI / human)       │
│  3. execute_action() — push to extension           │
│  4. Extension executes, reports result via HTTP     │
│  5. Back to step 1                                 │
│  6. Loop until task complete or failed             │
└───────────────────────────────────────────────────┘
```

### Key Design Points

- **Reactive loop**: No pre-planning. Each iteration observes → decides → executes → observes
- **Tool-based**: Memory, AI, human-in-the-loop are all tools the Agent calls autonomously
- **Memory at each step**: Agent decides when to check memory, not hardcoded
- **Failed operations stored**: For session-level dedup, cross-session reuse deferred to later versions

---

## 4. Memory System

### Storage

- **Database**: SQLite3 (no CGO) + GORM
- **Location**: Local filesystem
- **No RAG in V0.1**: Pure SQL matching, vector search deferred to later versions

### Data Model

```sql
CREATE TABLE memories (
    id              TEXT PRIMARY KEY,
    session_id      TEXT NOT NULL,

    -- Page context
    page_url        TEXT NOT NULL,
    page_url_pattern TEXT,          -- Normalized URL pattern (e.g., /product/{id})
    page_title      TEXT,
    page_features   TEXT,           -- Page feature fingerprint for matching
    dom_snapshot    TEXT,           -- Simplified DOM fragment
    screenshot      BLOB,          -- Screenshot (from human teaching)

    -- Action
    action_type     TEXT NOT NULL,  -- click / input / scroll / navigate / select
    action_target   TEXT,           -- Semantic description of element
    action_selector TEXT,           -- DOM selector (fallback positioning)
    action_value    TEXT,           -- Input value (if applicable)

    -- Result
    result          TEXT NOT NULL,  -- success / fail
    fail_reason     TEXT,           -- Failure reason

    -- Metadata
    source          TEXT NOT NULL,  -- human / ai / memory
    created_at      DATETIME NOT NULL
);

CREATE INDEX idx_memories_url ON memories(page_url_pattern);
CREATE INDEX idx_memories_session ON memories(session_id);
```

### Matching Strategy (Pure SQL)

```
1. Normalize current URL to pattern:
   /product/12345 → /product/{id}

2. Query:
   SELECT * FROM memories
   WHERE page_url_pattern = '/product/{id}'
   AND action_type = 'click'
   AND action_target LIKE '%publish%'
   AND result = 'success'
   ORDER BY created_at DESC
   LIMIT 1

3. If hit → replay directly
4. If miss → Agent uses AI inference
```

### When to Consider RAG

- Memory store reaches thousands of entries
- Cross-site generalization needed (e.g., "fill address" on site A applies to site B)
- At that point, add vector search as supplementary layer, SQL remains primary

---

## 5. Chrome Extension Design

### UI Components

```
┌─ Chrome Browser ──────────────────────────────────┐
│                                                     │
│  ┌─ Side Panel ──────────────────────────────────┐ │
│  │                                                │ │
│  │  [Model: Haiku | Sonnet | Opus]               │ │
│  │                                                │ │
│  │  Task Input:                                   │ │
│  │  ┌──────────────────────────────────┐          │ │
│  │  │ Describe your task...            │ [→]      │ │
│  │  └──────────────────────────────────┘          │ │
│  │                                                │ │
│  │  ─────────────────────────────────             │ │
│  │                                                │ │
│  │  Execution Progress:                           │ │
│  │  ✓ Click "Publish" button    [memory hit]      │ │
│  │  ✓ Wait for page load       [auto detect]      │ │
│  │  ● Fill product title       [ai: sonnet]       │ │
│  │  ○ Fill description         [pending]          │ │
│  │  ○ Set price                [pending]          │ │
│  │                                                │ │
│  │  ┌────────────────────────────────────┐        │ │
│  │  │ ⚠ Need Help                       │        │ │
│  │  │ AI can't find "price input".       │        │ │
│  │  │ Please click it once.              │        │ │
│  │  │              [Start Teaching]       │        │ │
│  │  └────────────────────────────────────┘        │ │
│  │                                                │ │
│  ──────────────────────────────────────────────    │ │
│  │ v0.1.0              Settings | Help            │ │
│  └────────────────────────────────────────────────┘ │
│                                                     │
│  ┌─ Page Content ────────────────────────────────┐ │
│  │                                                │ │
│  │  (When human teaching needed:)                  │ │
│  │  ┌ ─ ─ ─ ─ ─ ─ ─ ─ ┐                         │ │
│  │  │  Highlighted     │ ← amber border +         │ │
│  │  │  target element  │   pulsing animation      │ │
│  │  └ ─ ─ ─ ─ ─ ─ ─ ─┘                         │ │
│  │  "Please click the price input"                │ │
│  │                                                │ │
│  └────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────┘
```

### Core Modules

| Module | Responsibility |
|--------|---------------|
| DOM Collector | Collect simplified DOM semantic tree (tag, text, attributes, interactable state) |
| Action Executor | Receive backend instructions, execute click/input/scroll/navigate |
| Human-in-the-loop UI | Highlight + overlay + prompt, wait for user action, capture result |
| WebSocket Client | Maintain long connection with backend, receive pushes |
| HTTP Client | Upload page state, action results, human teach data |
| Side Panel | Task input, status display, connection status, settings (collapsed) |

### DOM Collection Strategy

- Only collect interactable elements (input, button, a, select, textarea, etc.)
- Preserve semantic attributes (placeholder, aria-label, text content)
- Strip styles, scripts, and irrelevant information
- Generate tree structure with unique identifiers for each node

---

## 6. Communication Protocol

### Design Principle

Following the IM system pattern:
- **WebSocket**: Backend → Extension pushes only (instructions, status updates)
- **HTTP**: Extension → Backend uploads (page state, action results, human teach data)

### WebSocket Messages (Backend → Extension)

All messages follow the Update pattern with `seq`:

```json
{
  "seq": 42,
  "type": "message_type",
  "id": "uuid",
  "payload": { ... }
}
```

| type | seq | Description | payload |
|------|-----|-------------|---------|
| `connected` | - | WS handshake complete | `{ server_time, max_seq }` |
| `ping` | - | Heartbeat | `{}` |
| `page.query` | 0 | Request page state | `{ include_screenshot }` |
| `action.execute` | >0 | Execute action | `{ action, selector, value? }` |
| `task.started` | >0 | Task started | `{ task_id, description }` |
| `task.step` | >0 | Step update | `{ step_id, status, message }` |
| `task.completed` | >0 | Task completed | `{ task_id, summary }` |
| `task.failed` | >0 | Task failed | `{ task_id, error }` |
| `need_help` | >0 | Request human help | `{ message, highlight_selector }` |
| `thinking` | 0 | AI reasoning stream | `{ delta }` |

### HTTP Endpoints (Extension → Backend)

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/page/state` | Upload current page state (DOM + URL + title) |
| POST | `/api/v1/action/result` | Upload action execution result |
| POST | `/api/v1/teach` | Submit human teach data (element + action + screenshot) |
| GET | `/api/v1/updates?last_seq=N` | Polling fallback for missed updates |

### Connection Lifecycle

```
1. Extension starts → connects to ws://host:port/ws?token=<key>&last_seq=<N>
2. Backend validates auth, returns { type: "connected", payload: { max_seq } }
3. If server max_seq > local last_seq → HTTP poll to fill gap
4. Heartbeat: Extension sends ping every 30s, backend expects within 60s
5. On disconnect → auto-reconnect with exponential backoff (1s → 2s → 4s → 30s cap)
6. On reconnect → kick old connection, re-sync with last_seq
```

---

## 7. AI Model Strategy

### Tiered Model System

| Tier | Use Case | Characteristics |
|------|----------|-----------------|
| Haiku | Simple page matching, intent parsing | Fast, cheap |
| Sonnet | DOM analysis, action generation | Balanced |
| Opus | Complex multi-step reasoning | Slow, expensive |

### API Compatibility

- OpenAI-compatible protocol (`/v1/chat/completions`)
- Custom `base_url` support (for Azure, local LLMs, etc.)
- Model selection per step: Agent decides which tier to use based on complexity

---

## 8. Tech Stack

| Component | Technology |
|-----------|-----------|
| Backend language | Go |
| Agent framework | Eino (ByteDance) |
| Database | SQLite3 (no CGO) via modernc.org/sqlite |
| ORM | GORM |
| WebSocket | gorilla/websocket or nhooyr/websocket |
| HTTP router | gin or chi |
| Chrome Extension | Manifest V3, TypeScript |
| Extension UI | Side Panel API |
| AI API | OpenAI-compatible protocol |

---

## 9. V0.1 Scope

### In Scope

- Chrome Extension with Side Panel UI
- Go backend with Eino Agent
- Natural language task input
- DOM-based page understanding
- Memory system (SQLite3, SQL matching)
- Human-in-the-loop teaching flow
- Tiered model support (OpenAI-compatible)
- WebSocket communication with seq-based replay
- Single executable deployment

### Out of Scope (Deferred)

- Cloud deployment / multi-user
- RAG / vector search for memory
- Visual understanding (VLM)
- Cross-session failed operation reuse
- Plugin API
- Docker / installer packaging

---

## 10. Reference

- BP: `docs/BP.md` (Gemini-generated business plan)
- Prototype: `docs/prototype/extension-mockup.html`
- IM Protocol Reference: `/Users/leichujun/go/src/github.com/PineappleBond/eino-demo-dev/docs/API.md`
