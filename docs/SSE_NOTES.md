# 文章工作流 SSE 协议

本文描述已启用的协议。前后端同步切换，不混用旧 `connected`、`outline_delta`、
`image_complete` 或 `merge_done`。原始模型文本不再用于大纲 UI。

## 1. 连接与权限

```http
GET /api/article/progress/{taskId}
Cookie: session=...
Accept: text/event-stream
```

仅作者或管理员可订阅。鉴权在写入 SSE 响应头之前完成，心跳仍为 `: ping` 注释。
每段工作流完成或失败后结束本次订阅，用户确认标题/大纲后建立下一段订阅。
断开进度连接不取消后台生成。

## 2. 帧与类型

业务名称来自 SSE `event:`，JSON 不再重复声明路由类型：

```text
event: content_delta
data: {"runId":"run-1","seq":7,"phase":"CONTENT_GENERATING","data":{"delta":"正文增量"}}

```

| 字段 | 说明 |
|------|------|
| `runId` | 一次阶段执行的身份，不是 article/task ID |
| `seq` | 运行内递增版本；初始快照可为 0，必须处于 JavaScript 安全整数范围 |
| `phase` | 文章业务阶段；人工确认仍由数据库状态表达 |
| `data` | 与事件名称绑定的类型化业务 payload |

Go 定义：`internal/app/article/workflow/events.go`；TypeScript 定义：
`frontend/src/types/articleWorkflow.ts`。前端先校验 JSON 和完整 payload，再加入 `type`
判别字段，不能通过类型断言直接信任网络数据。

## 3. 事件

| event | data | 本段终态 |
|-------|------|----------|
| `task_snapshot` | 完整当前投影，见下一节 | 由 `data.terminal` 决定 |
| `titles_done` | `{ options: TitleOption[] }` | 是 |
| `outline_snapshot` | `{ sections: OutlinePreviewSection[] }` | 否 |
| `outline_done` | `{ sections: OutlineSection[] }` | 是 |
| `content_delta` | `{ delta: string }` | 否 |
| `content_generated` | `{ content: string }`，完整正文用于校准 | 否 |
| `images_planned` | `{ tasks: ImageTask[] }` | 否 |
| `image_finished` | `{ outcome: ImageOutcome, summary: ImageSummary }` | 否 |
| `images_done` | `{ outcomes: ImageOutcome[], summary: ImageSummary }` | 否 |
| `content_done` | `{ content, fullContent, images, warnings }` | 是 |
| `task_error` | `{ code, message }`，可公开的错误信息 | 是 |

成功终态只在结果及对应阶段持久化之后发布。阶段持久化和终态发布共用进度锁，避免
下一阶段抢先覆盖上一阶段的完成事件。原始 Provider 错误仍写入服务端诊断日志，不进入
进度错误消息。`GENERATION_TIMEOUT` 与 `GENERATION_FAILED` 区分超时及一般失败。

## 4. 快照、重连与边界

订阅首帧一定是仅发给该订阅者的 `task_snapshot`，不是广播：

```json
{
  "runId": "run-1",
  "seq": 7,
  "phase": "CONTENT_GENERATING",
  "data": {
    "status": "PROCESSING",
    "terminal": false,
    "step": "images",
    "editorial": {
      "mainTitle": "主标题",
      "subTitle": "副标题",
      "userDescription": ""
    },
    "content": "完整的当前正文",
    "imageSummary": {"total": 3, "settled": 2, "succeeded": 1, "skipped": 1}
  }
}
```

快照整体替换前端投影，缺省字段表示清空，不是“保留旧值”。可选字段还包括 `topic`、
`titleOptions`、`outline`、`fullContent`、`images`、`imageOutcomes`、`warnings` 和 `error`。
`step` 为 `titles`、`outline`、`content`、`image_plan`、`images`、`merge` 或 `completed`。
数组在完整结果中发送 `[]` 而不是 `null`。

活跃任务从内存投影构造快照，订阅和事件更新串行，保证快照后的第一条事件版本连续。
已经结束的任务释放内存投影；新订阅重新读取数据库恢复结果。历史图片通过
`savedImages` 保留既有持久化格式（包括旧 PICSUM 记录），不伪造新的图片任务或改写历史正文。

等待标题选择、大纲确认及已完成/失败的快照均为终态；HTTP Handler 发出后立即结束订阅。
因此任务在浏览器连上之前已结束时，不需要等待一个已经错过的完成事件。

慢订阅者缓冲区满时会被关闭，不能静默丢弃文本增量，也不能发送到已关闭 channel。
前端遇到连接中断、版本缺口或不匹配的运行身份时重新订阅，读取新快照修复状态；重复和
过期版本不重复应用。重连采用有上限的退避，连续失败达到上限后明确提示连接错误。
新建任务将 task ID 放入当前页面查询参数，刷新该页面可重新订阅该任务。

这不是持久化事件日志，不依赖 `Last-Event-ID` 重放。运行中投影及 fan-out 仍是进程内能力，
多实例共享进度、进程重启后恢复模型执行和持久化任务续跑不在本步范围内。

## 5. 大纲与配图语义

大纲预览是累计对象快照，每次整体替换。预览字段可以尚未完整，不等同于最终大纲；
`outline_done` 才提供完整校验后的章节。前端不拼接 JSON 字符串或猜测不完整 JSON。
Genkit 大纲 Flow 只使用类型化 `ExecuteStream`，原来的双路原始增量入口已经移除。

配图规划只返回插入点 ID、用途、描述与首选方式，不返回整篇正文或占位符。
模型看不到插入字节偏移，任务 ID 和排序由代码分配。封面固定 `position=1`，
正文图片不会因封面跳过而重新编号。

`ImageOutcome` 只有两种终态：

- `succeeded`：包含 `task` 和 `asset`，不含 `warning`。
- `skipped`：包含 `task` 和 `IMAGE_SKIPPED` 警告，不含 `asset`。

`summary` 为 `{ total, settled, succeeded, skipped }`，
满足 `settled = succeeded + skipped <= total`。图片进度按 `settled / total` 计算，
零任务直接处理结束，不除以零。跳过不发 `task_error`；文章仍可完成。
最终图文及警告通过 `content_done` 直接提供，无需为补全文再竞态读取 REST。
警告同时存储于 `articles.generation_warnings`，文章详情刷新后仍可见。

## 6. 典型时序

```text
创建：snapshot → titles_done | task_error
确认标题：snapshot → outline_snapshot* → outline_done | task_error
确认大纲：snapshot → content_delta* → content_generated → images_planned
         → image_finished* → images_done → content_done | task_error
晚订阅已结束阶段：terminal snapshot → 关闭
```

确认操作通过数据库预期阶段条件更新原子领取下一阶段；重复请求不能启动多份生成任务。
人工确认不使用长期挂起的模型或 Flow 实例。

## 7. 执行日志与相关 API

`GET /api/article/execution-logs/{taskId}` 保留现有摘要与日志格式。每次执行仍是一条业务记录，
Genkit 内部 prompt/model/tool span 不重复写成业务步骤。重复大纲修改的同名耗时累加，
前端同时支持当前与历史步骤名，并按“执行步骤”展示记录数。

REST 继续负责 `/create`、`/confirm-title`、`/confirm-outline` 和 `/modify-outline`，
不在 SSE 连接中发送命令。新增或修改事件时必须同步更新 Go/TypeScript 契约、
解析校验、投影、终态判断及相关测试。
