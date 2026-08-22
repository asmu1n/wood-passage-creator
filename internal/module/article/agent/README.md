# article/agent（阶段 1）

确定性任务智能体 + 自研编排，**只依赖** `port.ChatModel`。

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

## 使用

```go
orch := agent.NewOrchestrator(agent.Deps{LLM: chatModel})
state := &article.ArticleState{TaskID: id, Topic: topic, Style: "tech"}
_ = orch.RunPhase1(ctx, state)
// 用户选标题后：
state.Title = &article.TitleResult{MainTitle: "...", SubTitle: "..."}
_ = orch.RunPhase2(ctx, state, sseDelta)
_ = orch.RunPhase3(ctx, state, sseDelta)
```

## 边界

- 不 import eino / compose / adk
- 真实出图通过 `ImageGenerator` 端口注入
- agent_log / SSE 推送完成事件放在 Service 层，不塞进 Agent 内部（Agent 只改 state）
