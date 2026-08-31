# SSE 进度推送说明（Wood Passage Creator）

> 与实现保持同步的契约笔记。权威事件名与 payload 见 `internal/module/article/sse.go`。  
> 最后对齐代码日期：以仓库当前 `sse.go` / `pkg/sse` 为准。

---

## 1. 范式

采用 **B 范式**：

- 传输层：`event: <name>` + `data: <json>`
- 业务 JSON **不再**内嵌自定义 `type` 做路由
- 前端：`EventSource` + `addEventListener(eventName, ...)`

连接：

```http
GET /api/article/progress/{taskId}
Cookie: session=...
Accept: text/event-stream
```

需登录；仅作者或管理员可订。

---

## 2. 架构

```text
Browser (EventSource)
    │  GET /api/article/progress/:taskId
    ▼
Handler.GetProgress          ← 头、读 channel、写帧、心跳、终态退出
    │  Service.SubscribeProgress
    ▼
article.Service              ← 鉴权、publish
    │
pkg/sse.Hub                  ← topic=taskId，fan-out（多页同订互不踢）
    ▲
流水线 / Orchestrator 回调   ← onProgress → publish
```

| 层级 | 位置 | 职责 |
|------|------|------|
| 帧编码 | `internal/pkg/sse/frame.go` | `WriteEvent` / `WriteComment` / `Flush` |
| 总线 | `internal/pkg/sse/manager.go` | Subscribe / Publish（接口与实现同包） |
| 业务事件 | `internal/module/article/sse.go` | 事件名、payload、`IsTerminalSSEEvent` |
| HTTP | `article/http/handler.go` | 长连接循环 |

说明：历史上曾有 `port/sse.go`，现已并入 `pkg/sse`，不要再引用 port 版 SSE。

---

## 3. 帧格式

业务帧：

```text
event: outline_delta
data: {"delta":"## 一、"}

```

心跳（注释帧，JS 无回调，仅保活）：

```text
: ping

```

- `data` 多行会拆成多条 `data:` 行（实现见 `WriteEvent`）
- 写后必须 `Flush`，否则浏览器收不到增量

---

## 4. 事件表（完整）

### 通用

| event | 终态? | payload 要点 |
|-------|-------|----------------|
| `connected` | 否 | `{ phase, status }` 订阅瞬间快照 |
| `task_error` | **是** | `{ message, phase? }` |

### Phase1 标题

| event | 终态? | payload |
|-------|-------|---------|
| `titles_done` | **是** | `{ phase, titleOptions }` |

### Phase2 大纲

| event | 终态? | payload |
|-------|-------|---------|
| `outline_delta` | 否 | `{ delta }` 文本增量 |
| `outline_done` | **是** | `{ phase, outline }` |

### Phase3 正文 + 配图

| event | 终态? | payload |
|-------|-------|---------|
| `content_delta` | 否 | `{ delta }` |
| `content_generated` | 否 | `{ phase, contentLength }` 正文生成完 |
| `images_planned` | 否 | `{ phase, count }` 分析出配图需求数 |
| `image_complete` | 否 | `{ image, done, total }` **单张** |
| `images_done` | 否 | `{ phase, count, images[] }` **全量** |
| `merge_done` | 否 | `{ phase, fullContentLength }` |
| `content_done` | **是** | `{ phase, status }` 默认不推全文 |

`IsTerminalSSEEvent`：`titles_done` | `outline_done` | `content_done` | `task_error`  
→ Handler 收到后结束本次 SSE 连接（用户确认标题/大纲后需重新订阅进入下一阶段）。

---

## 5. 推荐时序

```text
# 创建任务后订一次
connected → titles_done | task_error

# 确认标题后重连
connected → outline_delta* → outline_done | task_error

# 确认大纲后重连
connected → content_delta* → content_generated
         → images_planned
         → image_complete* → images_done
         → merge_done → content_done | task_error
```

REST 负责阶段动作（confirm-title / confirm-outline / modify-outline），**不**与 SSE 混在同一长连接里发命令。

---

## 6. Fan-out 与重连

- 同一 `taskId` 多标签页：`Subscribe` 各有独立 channel，互不踢除
- 缓冲区满：丢该订阅者本条，不影响其它人（见 hub 实现）
- 终态后连接关闭；需要下一段进度请再 `EventSource` 打开

---

## 7. 前端示例

```js
const es = new EventSource(`/api/article/progress/${taskId}`, { withCredentials: true })

es.addEventListener('connected', (e) => { /* JSON.parse(e.data) */ })
es.addEventListener('outline_delta', (e) => { /* append delta */ })
es.addEventListener('image_complete', (e) => { /* single image */ })
es.addEventListener('content_done', (e) => { es.close() })
es.addEventListener('task_error', (e) => { es.close() })
```

---

## 8. 相关 REST（非 SSE）

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/article/create` | 创建任务 |
| POST | `/api/article/confirm-title` | 进 Phase2 |
| POST | `/api/article/confirm-outline` | 进 Phase3 |
| POST | `/api/article/modify-outline` | VIP AI 改大纲 |
| GET | `/api/article/execution-logs/{taskId}` | Agent 耗时日志 |

---

## 9. 维护约定

- 新增事件：先改 `sse.go` 常量与 payload，再改本笔记与前端监听
- 改帧格式：只动 `pkg/sse/frame.go`，并补单测
- OpenAPI：SSE 长连接 swag 描述有限，细节以本文 + `sse.go` 为准
