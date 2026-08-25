# SSE 随记：帧封装 · 事件订阅 · Fan-out 推送

> 基于 `wood-passage-creator` 当前实现整理，便于回顾设计取舍与代码落点。  
> 产品语境：文章任务按阶段推送进度（大纲 / 正文分开）；阶段之间的确认与修改走 REST，不走同一条长连接。

---

## 1. 我们在解决什么问题

需要把「任务生成过程中的增量与阶段结果」从服务端推到浏览器，特点是：

- **单向**：服务端 → 客户端
- **长连接**：一次 HTTP 响应持续写
- **多类型消息**：连接成功、流式 delta、阶段完成、业务错误
- **同任务可能多开页**：不能只保留最后一个订阅者

选用 **SSE（Server-Sent Events）**，并采用 **B 范式**：

- 协议层用 `event:` 表达类型
- 业务 JSON 只放在 `data:`
- 不在 JSON 里再塞一层自定义 `type` 做路由（与旧「data-only + type」范式区分）

---

## 2. 总体架构

```text
Browser (EventSource)
    │  GET /api/article/progress/:taskId
    ▼
Handler.GetProgress          ← HTTP 头、读 channel、写帧、心跳、终态退出
    │  SubscribeProgress
    ▼
article.Service              ← 鉴权、访问控制、业务 publish*
    │  port.SSEHub
    ▼
pkg/sse.Hub                  ← topic=taskId 的 fan-out 总线
    │
Agent / 流水线回调            ← publishOutlineDelta / Done / Error ...
```

| 层级 | 代码位置 | 职责 |
|------|----------|------|
| 契约 | `internal/port/sse.go` | `SSEEvent`、`SSEHub` |
| 总线 | `internal/pkg/sse/manager.go` | 订阅 / 退订 / 按 topic 广播 |
| 帧编码 | `internal/pkg/sse/frame.go` | 标准 SSE 文本 + Flush |
| 业务事件 | `internal/module/article/sse.go` | 事件名、payload、`publish*` |
| 用例 | `service.SubscribeProgress` | 权限校验 + 订阅 + 首包快照 |
| 传输 | `http/handler.GetProgress` | 长连接循环 |

**原则：** 业务只发布「结构化事件」；只有 Handler 写 HTTP；Hub 只做进程内路由，不拼 `event:`/`data:` 字符串。

---

## 3. 帧封装（Wire Format）

### 3.1 标准长什么样

一条业务消息：

```text
event: outline_delta
data: {"delta":"## 一、"}

```

心跳（注释帧，浏览器 **收不到** JS 回调，只保活）：

```text
: ping

```

字段含义：

| 行 | 作用 |
|----|------|
| `event:` | 事件名 → 前端 `addEventListener(name, ...)` |
| `data:` | 负载；多行则多条 `data:`，浏览器拼成一条（中间 `\n`） |
| 空行 | 一帧结束 |
| `: ...` | 注释；用于心跳 |
| `id:` | 本实现 **暂未使用**（预留给续传） |
| `retry:` | 本实现 **暂未使用** |

### 3.2 封装 API（`internal/pkg/sse/frame.go`）

```text
WriteEvent(w, name, data)   // 业务帧
WriteComment(w, comment)    // 心跳等注释
Flush(rc)                   // http.ResponseController.Flush
```

`WriteEvent` 要点：

1. `name != ""` 时写 `event: %s\n`
2. 将 `data` 中 `\r\n` / `\r` 规范成 `\n`，再按行拆分
3. 每一行写 `data: %s\n`；`data` 为空则写一行空 `data:`
4. 最后写 `\n` 结束帧
5. 任一步 `Write` 失败返回 `error`（上层当作断连）

业务上仍应尽量发 **单行 JSON**；多行逻辑是协议层防御，避免 payload 含换行时破坏帧边界。

### 3.3 Handler 里如何用

进流之后约定：

- 写失败 / Flush 失败 → **`return nil`**（不要再返回业务 JSON，避免污染已开始的 stream）
- 顺序：**Write → Flush →（若终态）return nil**

响应头：

```text
Content-Type: text/event-stream
Cache-Control: no-cache
Connection: keep-alive
X-Accel-Buffering: no          // 减少 Nginx 缓冲
```

另：SSE 场景 HTTP Server 的 **`WriteTimeout` 应为 `0`**（禁用）。Echo v5 默认不设写超时；建议在启动配置里显式写出，防止以后被改成 30s 误杀长连接。

---

## 4. 事件模型（业务侧）

### 4.1 总线上的事件（`port.SSEEvent`）

```text
Topic  string  // 路由键 = taskId（不是 SSE 的 id:）
Name   string  // → event:
Data   []byte  // → data:（JSON）
```

**切记：** `Topic` 只用于 Hub 投递；不要写进 SSE `id:` 行。事件序号与 taskId 是两个概念。

### 4.2 事件名与 payload（`article` 模块）

| Name (`event:`) | 含义 | data 示例 | 终态？ |
|-----------------|------|-----------|--------|
| `connected` | 订阅成功，当前任务快照 | `{"phase","status"}` | 否 |
| `outline_delta` | 大纲流式片段 | `{"delta"}` | 否 |
| `outline_done` | 大纲阶段完成 | `{"phase","outline"}` | **是** |
| `content_delta` | 正文流式片段 | （按实现） | 否 |
| `content_done` | 正文阶段完成 | （按实现） | **是** |
| `task_error` | 业务失败 | `{"message","phase?"}` | **是** |

发布出口集中在 `article/sse.go` 的 `publish` / `publish*`：

```text
json.Marshal(payload)
hub.Publish(SSEEvent{Topic: taskID, Name: typ, Data: bytes})
```

业务代码 **不要** 直接 `fmt.Fprintf` 写 Response。

### 4.3 产品生命周期与 SSE 段

大纲与正文 **分开**；中间有用户改大纲、确认大纲（REST）：

```text
SSE #1（大纲）
  connected → outline_delta* → outline_done | task_error
  → 服务端结束本连接

REST：编辑 / confirm-outline

SSE #2（正文，同 path 新连接）
  connected → content_delta* → content_done | task_error
  → 再次结束
```

同一接口：`GET /article/progress/:taskId`，不同阶段各订一次即可。

---

## 5. 事件订阅

### 5.1 对外接口

```text
SSEHub.Subscribe(topic) → (ch <-chan SSEEvent, cancel func())
```

- `topic`：此处即 `taskId`
- `ch`：该连接 **专属** channel（缓冲 64）
- `cancel`：只退订自己；内部 `sync.Once`，可重复调用

Handler 用法：

```text
ch, cancel, err := svc.SubscribeProgress(ctx, taskID, actorID, role)
// err ≠ nil：尚未进 stream，可返回普通 JSON 错误
defer cancel()
```

`SubscribeProgress` 内部：

1. `loadAccessibleByTaskID`（权限）
2. `hub.Subscribe(taskID)`
3. `PublishConnected(taskID, phase, status)` 推快照

### 5.2 subID 怎么处理（关键设计）

| 问题 | 结论 |
|------|------|
| subID 是什么？ | **一条 SSE 连接**在 Hub 内的身份证 |
| 谁生成？ | Hub 内 `atomic.Uint64` 自增，从 1 起 |
| 暴露给 Handler/前端吗？ | **否**；只活在 cancel 闭包里 |
| 能用 taskId 当 subID 吗？ | **不能**（那是 topic） |
| 能用 userId 当 subID 吗？ | **不适合**（同用户多标签会撞） |
| 要 UUID 吗？ | 单进程内存总线 **不必** |

结构：

```text
topics: map[taskId]map[subID]chan SSEEvent
```

cancel 闭包捕获 `(topic, subID, ch)`，删除时校验 `cur == ch`，避免误伤同 topic 其他连接。最后一名离开时删掉整个 `topics[taskId]`，防止 map 泄漏。

---

## 6. Fan-out 数据流

### 6.1 为什么要 fan-out

单 channel + 后订阅踢先订阅：

- 多标签互踢
- 若前端自动重连，可能来回抢连接

**不能**让多个 HTTP handler 读 **同一个** channel：Go channel 是队列，每条消息只会被一个 receiver 拿走，两边都会变成残缺流。

正确模型：**每个订阅者一个 channel，Publish 复制投递。**

### 6.2 Publish 路径

```text
业务 publish*
    → hub.Publish(event)  // event.Topic = taskId
    → RLock 取 topics[taskId]
    → 拷贝 []chan（避免持锁发送）
    → RUnlock
    → 对每个 ch：
         select { case ch <- event: ; default: 丢弃该订阅者本条 }
```

行为约定：

| 情况 | 行为 |
|------|------|
| 无订阅者 | no-op（事件不落盘、不重放） |
| 某订阅者 ch 满 | 只丢 **他的** 本条，不影响他人；Publish 不阻塞 |
| 多个订阅者 | 每人完整收到同一逻辑事件序列 |

### 6.3 订阅侧读路径

```text
GetProgress select:
  ctx.Done()     → return nil
  heartbeat 15s  → WriteComment("ping") + Flush
  msg <- ch:
      !ok                    → return nil
      WriteEvent + Flush
      终态名？               → return nil
```

终态名（当前）：

```text
outline_done | content_done | task_error
```

`defer cancel()` 保证退出时退订。

### 6.4 关于 `connected` 与 fan-out

每次新连接 `SubscribeProgress` 成功都会 `PublishConnected`，这是 **按 topic 广播**。  
因此 B 页连上时，已在线的 A 也可能再收到一条 `connected`。一般可当作快照刷新；若以后要「只推给新连接」，应往 **新 ch 单发**，而不是 `Publish` 全员。

---

## 7. 端到端时序（大纲阶段示例）

```text
Client          Handler              Service             Hub              Agent
  │ GET progress  │                    │                  │                 │
  │──────────────►│ SubscribeProgress  │                  │                 │
  │               │───────────────────►│ Subscribe        │                 │
  │               │                    │─────────────────►│ +sub            │
  │               │                    │ PublishConnected │                 │
  │               │                    │─────────────────►│ fan-out         │
  │  connected    │◄──── ch ───────────│◄─────────────────│                 │
  │◄──────────────│ WriteEvent         │                  │                 │
  │               │                    │                  │  onDelta        │
  │               │                    │◄─────────────────│◄────────────────│
  │  outline_delta│                    │ Publish          │                 │
  │◄──────────────│◄───────────────────│─────────────────►│ → all subs      │
  │  outline_done │                    │ Publish done     │                 │
  │◄──────────────│ return nil         │                  │                 │
  │               │ cancel             │                  │ -sub            │
```

服务端 `return` 只结束 **本次 HTTP 响应**。  
浏览器 `EventSource` 默认会在异常断开后 **自动重连**；**业务终态后应在客户端 `es.close()`**，否则可能再次 Subscribe。这是 SSE 协议层限制，不是业务 bug。

---

## 8. 关闭连接：社区共识（结合本项目）

| 角色 | 职责 |
|------|------|
| 服务端 | 发出明确终态事件 → Flush → 结束 handler → unsub |
| 客户端 | 收到终态 → UI 收尾 → `EventSource.close()`（停自动重连） |
| 兜底 | 重连后发现阶段已结束 → REST 拉详情并 close |

仅服务端掐 TCP、不发终态 / 客户端不 close，都会带来重连或资源占用问题。

本项目服务端半边已在 Handler 终态判断中完成；前端对接时补 `close()` 即可。

---

## 9. 明确未做的能力（边界）

- SSE `id:` + `Last-Event-ID` 断线续传  
- 事件持久化 / 断线补发历史 delta  
- 慢消费者背压策略（目前满则丢）  
- Hub 丢弃 metrics  
- 反代超时与 `proxy_buffering`（部署侧需单独配置）

**最终状态以数据库 + REST 查询为准**；SSE 是实时增强，不是唯一真相源。

---

## 10. 代码地图

```text
internal/port/sse.go                      接口
internal/pkg/sse/manager.go               Hub fan-out
internal/pkg/sse/frame.go                 帧封装
internal/pkg/sse/frame_test.go            写帧测试
internal/pkg/sse/manager_test.go          fan-out 测试
internal/module/article/sse.go            事件与 publish*
internal/module/article/service.go        SubscribeProgress、业务触发 publish
internal/module/article/http/handler.go   GetProgress
cmd/server/main.go                        NewHub() 注入
```

---

## 11. 一句话备忘

> **Service 往 taskId 频道发命名事件；Hub 按连接 fan-out；frame 包负责标准 SSE 字节；Handler 负责长连接与阶段终态收摊。大纲与正文是两段独立 SSE，中间 REST 确认大纲。**

---

## 12. 前端消费提示（备忘，非本仓库重点）

```js
const es = new EventSource(`/api/article/progress/${taskId}`)

es.addEventListener('connected', (e) => { /* JSON.parse(e.data) */ })
es.addEventListener('outline_delta', (e) => { /* ... */ })
es.addEventListener('outline_done', (e) => {
  // ...
  es.close()
})
es.addEventListener('task_error', (e) => {
  // ...
  es.close()
})

// 有 event: 时，业务事件不会进 onmessage
es.onerror = () => { /* 传输层；注意与 task_error 区分 */ }
```

---

*文档随实现演进；若 fan-out、终态集合或事件名有变，请同步改本节表格与时序。*
