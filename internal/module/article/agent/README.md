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

| 节点 | 模型 | 调用方式 | 输出 |
|------|------|----------|------|
| `TitleGenerator` | **jsonLLM**（`response_format=json_object`） | `Generate` | `{"options":[{"mainTitle","subTitle"},...]}` |
| `OutlineGenerator` | **jsonLLM** | `Stream` | `{"sections":[{"section","title","points"},...]}`；增量经回调转发 |
| `ContentGenerator` | textLLM | `Stream` | Markdown 正文；增量经回调转发 |
| `ImageAgent.analyze` | **jsonLLM** | `Generate` | `{"contentWithPlaceholders","imageRequirements":[...]}` |
| `ToolCallingGenerator` | textLLM（装配在 infra，非 Orchestrator jsonLLM） | `WithTools` + 有界 `Generate` 循环 | `ImageResult[]` |
| `ContentMerger` | — | 不调用 LLM | 将图片 URL 替换进占位符 |

结构化节点统一 **object 根**，以兼容 OpenAI 系 `json_object`（根不能是顶层数组）：
- title → `options`
- outline / modify-outline → `sections`
- image plan → `contentWithPlaceholders` + `imageRequirements`

`cmd/server` 分别构造 `NewChatModel`（text）与 `NewJSONObjectChatModel`（json）注入 `NewOrchestrator(text, json, ...)`。
Content 与 Tool Calling 必须保持 text model。

`ImageAgent` 先使用完整正文完成一次配图规划，再把紧凑的
`ImageRequirement[]` 交给 `port.ImageGenerator`。因此 Provider Tool Calling 的后续轮次
不会重复携带完整正文。

`infra/image.ToolCallingGenerator` 根据配图需求调用 Pexels、Iconify、Emoji、Mermaid、
SVG 或 AI 生图工具。Provider 执行结果作为 tool message 返回模型。调用轮次和总次数
均有限制；模型未完成的项目最后进入确定性 fallback。`ProviderExecutor` 只负责工具的
注册、元信息查询、实际执行与结果发布，不负责 LLM 编排，也不实现
`port.ImageGenerator`。

## Provider 元信息

每个图片 Provider 通过 `port.Provider.Metadata()` 提供自己的完整 LLM 元信息：

- `Method`：业务枚举与权限过滤标识。
- `Access`：访问级别（`FREE`、`VIP`、`INTERNAL`）。
- `PlannerDescription` / `PlannerUsageGuide`：注入配图规划 Prompt。
- `ToolName` / `ToolDescription`：转换为 Eino `ToolInfo`。

`ProviderExecutor` 只注册已通过配置可用性检查的 Provider；
应用层通过 `LookupProvider(method)` 校验请求方法是否已注册及其访问级别，并在创建任务时
固化当前用户可用的 Provider 列表；`INTERNAL` Provider 只允许系统内部使用。
`AvailableProviders(allowedMethods)` 再按当前任务允许的方法过滤。`ImageAgent` 的规划
Prompt 与 `ToolCallingGenerator` 的工具列表都读取这份结果，因此不会分别维护两套
Provider 描述，也不会向模型暴露未注册或未授权的 Provider。

Phase3 的节点顺序为：

```text
ContentGenerator
  → ImageAgent（analyze → ToolCallingGenerator）
  → ContentMerger
```

## 使用

```go
orch := agent.NewOrchestrator(textModel, jsonModel, imageGenerator, logRecorder)
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
- `ToolCallingGenerator` 负责工具对话循环，`ProviderExecutor` 负责 Provider Catalog 与确定性执行。
- Agent 节点只修改 `ArticleState`；Orchestrator 负责步骤日志和进度回调，Service 负责持久化与 SSE 发布。
