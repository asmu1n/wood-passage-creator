# article/agent（阶段 1）

确定性阶段编排 + 有界配图 Tool Calling。文本节点与配图 Agent 都直接使用 Eino 原生模型、消息和流类型。

```text
prompt/          模板与 Render
agent/
  title.go       标题方案
  outline.go     大纲（Stream）
  content.go     正文（Stream）
  image_analyze.go  配图需求 JSON
  content_merge.go  占位符替换（无 LLM）
  orchestrator.go   Phase1/2/3
```

配图执行由 `infra/image.ToolCallingGenerator` 完成：模型根据配图需求调用
Pexels、Iconify、Emoji、Mermaid、SVG 或 AI 生图工具，Provider 的执行结果会作为
tool message 返回模型。调用轮次和总次数均有限制；未完成项最后使用原有确定性
fallback，避免配图 Agent 失败拖垮整篇文章。`ProviderExecutor` 只负责实际执行工具，
不再实现 `port.ImageGenerator`。

## 使用

```go
orch := agent.NewOrchestrator(chatModel, imageGenerator, imageMethods, logRecorder)
state := &article.ArticleState{TaskID: id, Topic: topic, Style: "tech"}
_ = orch.RunPhase1(ctx, state)
// 用户选标题后：
state.Title = &article.TitleResult{MainTitle: "...", SubTitle: "..."}
_ = orch.RunPhase2(ctx, state, sseDelta)
_ = orch.RunPhase3(ctx, state, sseDelta)
```

## 边界

- module/agent 直接依赖 Eino `model.BaseChatModel` 与 `schema.Message`，不再经过自定义 LLM 端口转换
- Eino StreamReader 由 agent 单次消费，增量回调显式传入，不放入 context
- 真实出图通过 `ImageGenerator` 端口注入
- agent_log / SSE 推送完成事件放在 Service 层，不塞进 Agent 内部（Agent 只改 state）
