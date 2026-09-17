# article/agent

文章生成采用确定性阶段编排；LLM 节点直接使用 Eino 原生模型、消息和流类型。
配图规划与图片生成已经收敛到统一的 `ImageAgent`，其内部仍保持
“规划 → 有界 Tool Calling”两个子步骤。

```text
prompt/          模板与 Render
agent/
  title.go       标题方案
  outline.go     大纲（Stream）
  content.go     正文（Stream）
  image.go       配图规划 + 图片生成
  content_merge.go  占位符替换（无 LLM）
  orchestrator.go   Phase1/2/3
```

## LLM 调用方式

| 节点 | 调用方式 | 输出 |
|------|----------|------|
| `TitleGenerator` | `Generate` | 标题方案 JSON |
| `OutlineGenerator` | `Stream` | 大纲 JSON；增量经回调转发 |
| `ContentGenerator` | `Stream` | Markdown 正文；增量经回调转发 |
| `ImageAgent.analyze` | `Generate` | 带占位符正文 + `ImageRequirement[]` |
| `ToolCallingGenerator` | `WithTools` + 有界 `Generate` 循环 | `ImageResult[]` |
| `ContentMerger` | 不调用 LLM | 将图片 URL 替换进占位符 |

`ImageAgent` 先使用完整正文完成一次配图规划，再把紧凑的
`ImageRequirement[]` 交给 `port.ImageGenerator`。因此 Provider Tool Calling 的后续轮次
不会重复携带完整正文。

`infra/image.ToolCallingGenerator` 根据配图需求调用 Pexels、Iconify、Emoji、Mermaid、
SVG 或 AI 生图工具。Provider 执行结果作为 tool message 返回模型。调用轮次和总次数
均有限制；模型未完成的项目最后进入确定性 fallback。`ProviderExecutor` 只负责工具的
实际执行与结果发布，不负责 LLM 编排，也不实现 `port.ImageGenerator`。

Phase3 的节点顺序为：

```text
ContentGenerator
  → ImageAgent（analyze → ToolCallingGenerator）
  → ContentMerger
```

## 使用

```go
orch := agent.NewOrchestrator(chatModel, imageGenerator, logRecorder)
style := article.StyleTech
state := &article.ArticleState{TaskID: id, Topic: topic, Style: &style}
_ = orch.RunPhase1(ctx, state)
// 用户选标题后：
mainTitle, subTitle := "主标题", "副标题"
state.MainTitle = &mainTitle
state.SubTitle = &subTitle
_ = orch.RunPhase2(ctx, state, sseDelta)
_ = orch.RunPhase3(ctx, state, sseDelta)
```

## 边界

- `module/article/agent` 直接依赖 Eino `model.BaseChatModel` 与 `schema.Message`，不经过自定义 LLM 端口转换。
- Eino `StreamReader` 由节点单次消费；增量通过显式回调传递，不写入 `context`。
- `ImageAgent` 负责配图规划和生成顺序，真实出图通过 `port.ImageGenerator` 注入。
- `ToolCallingGenerator` 负责工具对话循环，`ProviderExecutor` 负责确定性 Provider 执行。
- Agent 节点只修改 `ArticleState`；Orchestrator 负责步骤日志和进度回调，Service 负责持久化与 SSE 发布。
