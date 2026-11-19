# 应用层（app）

**全部用例**与 **`*Request` 入参**放在本目录。  
`module` 只保留领域实体、领域工具、`Repository` 与 `repo` 实现。
HTTP 在 `httpapi/api`，**只依赖 app**。

| 包 | 职责 |
|----|------|
| `auth` | 注册 / 登录 |
| `user` | 资料、管理、配额、VIP |
| `article` | 文章流水线用例；标题、大纲、正文与配图均使用 Genkit |
| `payment` | Mock 支付 / 列表 |
| `statistics` | 管理端概览 |

`article/workflow`（package `articleworkflow`）包含 Genkit 标题、大纲、大纲修改、文章生成
Flow、内部配图执行器，以及类型化进度契约。`PlanningFlows` 与 `ArticleFlows` 分别装配确认点前后的执行单元；
只使用 Genkit。旧 Agent、模板、LLM 适配与旧 SSE 契约已删除，没有第二套生成实现或自动回退。

## Workflow 文件组织

实现按职责收敛到 7 个文件，保持同一个 `articleworkflow` 包：

| 文件 | 职责 |
|------|------|
| [`workflow.go`](article/workflow/workflow.go) | Flow 名称、运行上下文、业务输入输出与接口、共享响应校验和步骤日志 |
| [`planning.go`](article/workflow/planning.go) | 标题、大纲、大纲修改 Flow，以及对应的创作规则校验 |
| [`article.go`](article/workflow/article.go) | 正文、配图规划、执行器调用与合成的主 Flow 编排，进度事件适配 |
| [`image.go`](article/workflow/image.go) | 配图契约与内部执行器：批次并发、结果汇总、单图追踪与有界工具调用 |
| [`document.go`](article/workflow/document.go) | Markdown 文档与插入点结构、位置索引、任务排序和图文合成 |
| [`events.go`](article/workflow/events.go) | SSE 事件、快照、步骤与终态契约 |
| [`prompts.go`](article/workflow/prompts.go) | Prompt 资产入口、模型输入 DTO、schema 注册、风格投影与图表/生图绑定 |

测试归并到对应的 `planning`、`article`、`image`、`document`、`events`、`prompts` 测试文件。
阅读业务流程从 `planning.go` / `article.go` 进入；调整单图执行看 `image.go`，
调整模型输入或生成能力绑定看 `prompts.go`。Prompt 文本仍独立存放在 `prompts/`，
不因 Go 文件合并而混为一个模板。

## 文章工作流 Prompt 资产

新模板位于 `article/workflow/prompts/`，采用 Dotprompt。`PromptFS` 用
`go:embed all:prompts` 包含已确认模板和下划线开头的公共片段。标题、大纲和大纲修改
及正文、配图规划、单图工具决策、Mermaid/SVG 均使用这些模板。

| Prompt 名 | 输入 schema | 输出 | 作用 |
|-----------|-------------|------|------|
| [`titles`](article/workflow/prompts/titles.prompt) | `TitlesPromptInput` | `TitlesResult` | 3-5 组主副标题 |
| [`outline`](article/workflow/prompts/outline.prompt) | `OutlinePromptInput` | `OutlineResult` | 根据确认标题和用户要求生成大纲 |
| [`revise-outline`](article/workflow/prompts/revise-outline.prompt) | `ReviseOutlinePromptInput` | `OutlineResult` | 继承风格和补充要求，返回修改后的完整大纲 |
| [`content`](article/workflow/prompts/content.prompt) | `ContentPromptInput` | Markdown 文本 | 根据确认大纲生成不含配图的正文 |
| [`image-plan`](article/workflow/prompts/image-plan.prompt) | `PlanImagesInput` | `ImagePlan` | 从合法插入点中选择位置，只返回配图决策 |
| [`image-execute`](article/workflow/prompts/image-execute.prompt) | `ImageExecutePromptInput` | 工具请求或无法完成的简短说明 | 单图工具选择与失败修正 |
| [`mermaid`](article/workflow/prompts/mermaid.prompt) | `DiagramPromptInput` | Mermaid 源码 | 仅生成图表源码 |
| [`svg`](article/workflow/prompts/svg.prompt) | `DiagramPromptInput` | SVG 源码 | 仅生成自包含的 SVG |

`input.schema` 和结构化 `output.schema` 引用 Go 类型名称，不重复编写一套 YAML 字段定义。
`NewPlanningFlows` 在启动时注册这些命名 schema。独立的 `*PromptInput` 不包含任务身份、原始权限列表和
预算；图片执行只接收当前图片描述和从已授权 Provider Catalog 派生的工具对应关系。
`PlanImagesInput` 复用已有契约；原文及字节偏移的 `json:"-"` 字段不进入模型输入。
未指定风格时省略可选字段；`PromptEditorialContext` 将 Go 风格枚举/指针投影为字符串，
使原生 Dotprompt `ifEquals` 正确匹配，而不是依赖临时 JSON 转换或自定义模板 helper。

公共片段：

- [`_style.prompt`](article/workflow/prompts/_style.prompt)：使用 Go Dotprompt 内置 `ifEquals` 选择四种文章风格；未指定风格时不追加约束。
- [`_editorial-context.prompt`](article/workflow/prompts/_editorial-context.prompt)：统一呈现主副标题、风格、用户补充要求。
- [`_input-boundary.prompt`](article/workflow/prompts/_input-boundary.prompt)：区分素材与系统/权限指令，并约束事实、数据与引语的使用。

输入素材通过 `json` helper 呈现，而不是把用户文字作为模板再次编译。Provider 描述、
用法与工具对应关系继续从 Catalog 注入，模板不另存一份工具清单；指南中的
`keywords`、`prompt` 是执行阶段参数，不得混入配图规划的输出结构。

模板不固定模型、密钥、端点、温度、token 上限或工具列表。`image-execute` 声明
`returnToolRequests: true`，模型轮次、工具尝试次数、超时及成功后立即停止由
单图控制器落实，不能仅依靠 Prompt 文本。

已确认的文案方向：保留新媒体标题的吸引力，但不强制添加数字或夸张承诺；文章默认约
2000 字并允许用户明确覆盖；风格不再只改变语气，也要求保持事实边界。配图规划不返回
正文，最终失败不使用随机图兜底。

## Genkit 运行与迁移边界

`cmd/server` 创建一个共享 Genkit 实例，通过 `infra/llm.NewGenkit` 加载 OpenAI-compatible
插件及嵌入模板，再装配 `NewPlanningFlows`、`NewImageSources`、Provider Catalog 和
`NewArticleFlows`。继续使用现有 `llm.api_key`、`llm.base_url`、
`llm.model`，模型名无需手动添加 Genkit 前缀，也不要求 Gemini、Firebase 或 Google Cloud。
启动只构造客户端和注册资源，不请求模型 API。

已注册的 Flow：

- `generateTitles`：返回 3-5 个方案，并校验单行标题及主标题 30 字符上限。
- `generateOutline`：原生结构化流生成累计预览，最终校验非空大纲、连续编号及非空单行要点。
- `reviseOutline`：携带原有风格、用户补充要求和当前大纲，根据新建议返回完整结果。
- `generateArticle`：正文流 → Markdown 位置索引 → 配图规划 → 有界并发单图执行 → 原文插图。

单张配图不注册独立 Flow，也不作为供主模型调用的 Tool。`ArticleFlows` 持有包内具体类型
`imageExecutor`，由代码按固定顺序调用。文章侧负责配图规划与进度事件转换；执行器负责
批次并发、取消、结果汇总，以及每张图片的模型/工具循环，不依赖文章流式更新类型。
单图通过 `genkit.RunWithContext` 记录 `generateImage` 子步骤，附带 task/run/image ID，
模型与工具调用使用该步骤的上下文。输入输出继续显式校验；成功后立即返回，不追加模型总结调用。

Service 保留鉴权、VIP 检查、阶段状态和持久化；模型错误、取消、非正常结束及无效结果
均向上返回，不静默切回旧模型。异步阶段及同步大纲修改均受调用上下文和 10 分钟期限约束。
业务执行日志继续使用 `title_generator`、`outline_generator`、`outline_modifier`、
`content_generator`、`image_generator`、`content_merger`，
仅写粗粒度摘要；内部 prompt/model 调用由 Genkit trace 记录，并用 task/run/trace ID 关联。
未启用外部 telemetry exporter；生产日志仍使用项目现有 slog 配置。

前后端已同步切换类型化 SSE。大纲 Flow 只输出结构化快照，原始增量和
`GenerateOutlineDeltas` 临时入口已移除。Service 使用运行内 `runId/seq/phase/data`
envelope 发布所有事件；订阅首帧是私有的权威快照，慢消费者断开后通过重连修复投影。
成功终态在持久化之后发布，人工确认以数据库预期阶段条件更新原子领取下一阶段。
活跃投影只留在当前进程，结束后释放；晚订阅从已保存文章恢复，包含历史图片和缺图警告。
详见 [`docs/SSE_NOTES.md`](../../docs/SSE_NOTES.md)。

### 配图位置、预算和失败

`BuildArticleDocument` 使用 Goldmark 块解析器记录顶层 Markdown 块的原文字节边界，
不渲染、不重排原文。不提供代码块、列表、引用或 HTML 内部的插入位置；末尾未闭合
代码/容器不提供 EOF 插入点。模型只读取块文本和合法位置 ID，返回 `ImagePlan`。
代码校验授权、位置和重复项后按位置分配任务，封面固定 `position=1`，正文图片从 2 开始，
封面缺失不重编号。

单图最多 3 次模型决策和 3 次实际 Provider 尝试，90 秒 Provider 调用期限，3 分钟单图期限；
批次最多并发 3 项，均受阶段总期限约束。单图内部串行执行，每轮多个请求会被拒绝并反馈，
完全相同的失败调用不会再次执行；未知、未授权工具及非法参数不触发 Provider。
工具实例仅在单次运行可见，不向共享注册表重复注册，不向模型回传图片 URL。
父阶段取消和鉴权/配置错误向上失败；普通单图生成失败、预算耗尽或单图超时生成
`IMAGE_SKIPPED`，保留其余图片和正文，最终文章仍可为 `COMPLETED`。

`ComposeArticle` 仅在合法边界插入成功图片；失败项不插入占位符，不解释原文中的花括号。
Mermaid 继续以源码代码块插入，图片和 SVG 使用现有 URL/data URL 表示。
缺图警告保存到新增的可空 JSONB 字段 `articles.generation_warnings`，与 `error_message`
区分；空列表可清除旧警告，历史记录没有该字段时视为无警告。Ent 已从 schema 重新生成，
启动时沿用原有迁移机制，不修改历史正文和图片结构。

Mermaid/SVG 通过已确认的 Prompt 调用同一个 Genkit 运行时；NanoBanana 在配置密钥时
启用 `googlegenai.GoogleAI` 插件，并通过 Genkit 使用原有 `nano_banana.model` 和
`aspect_ratio`。未配置的能力不进入 Catalog，包括 Go interface 中包装的 typed-nil Provider。
真实搜索和对象存储仍由 ProviderExecutor 执行。生产链路不再调用 Gemini 手写 HTTP API，
不再注册或尝试 PICSUM 兜底。

图片占位符字段已移除，不再提供该字段的兼容处理。`ImageRequirement` 不含占位符或
重复的来源字段；实际来源由 `ProviderExecutor.Execute` 的显式 method 参数决定。
若旧数据与当前结构出现严重冲突，另行清洗数据库，本次变更不改写存量数据。

本地可在现有数据库和 Redis 环境下运行 Developer UI：

```bash
genkit start -- go run ./cmd/server
genkit flow:run generateTitles '{"run":{"articleId":0,"taskId":"dev","runId":"dev"},"topic":"木工入门"}'
```

需要 Zed 断点调试时，先在仓库根目录单独启动 Developer UI（需安装 `genkit-cli`）：

```bash
genkit start --port 4000
```

然后在 Zed 中选择 [`.zed/debug.json`](../../.zed/debug.json) 的 `Debug Server (Genkit)`。
Delve 继续加载根目录 `.env` 并启动 `./cmd/server`；调试配置额外设置 `GENKIT_ENV=dev`，
显式启用 Reflection API，且只监听 `127.0.0.1`。Genkit 在 `.genkit/runtimes/` 写入发现信息，
同一目录启动的 CLI 会自动连接，并为运行时配置本地 trace/log telemetry 地址，无需固定端口。
打开 <http://localhost:4000> 即可查看已注册的 Flow、Prompt、Model 和执行 trace；
从 UI 发起的调用也能命中 Zed 断点。此模式不要再使用 `genkit start -- go run ./cmd/server`，
否则会重复启动后端。调试仍需现有数据库、Redis 和模型配置，`.genkit/` 本地数据不提交。

不要直接将所有 Flow 暴露为公共 HTTP 路由；生产请求仍通过现有应用鉴权入口。

## Runtime 与事务

全局 `app.Runtime` 匿名嵌入 `port.TxManager`，由 `cmd/server` 在启动阶段初始化一次。需要事务的用例通过 `app.CurrentRuntime().WithinTx` 编排；当前事务仍保存在请求 `context` 中，不是全局事务。

业务方法可通过 `app.CurrentRuntime().AfterCommit` 登记缓存失效等提交后动作。方法独立执行时回调立即运行；参与外层事务时等待最外层 Commit；Rollback 时不运行。

当前事务用例：`article.Create`、`payment.CompleteMockVIP`。完整约定见 [事务文档](../../docs/TRANSACTIONS.md)。


## 包名约定（重要）

app 目录下的 Go **package 名**必须与 module 领域包名错开，便于 swag 与 import 消歧：

| 目录 | package |
|------|---------|
| `app/auth` | `authapp` |
| `app/user` | `userapp` |
| `app/article` | `articleapp` |
| `app/payment` | `paymentapp` |
| `app/statistics` | `statsapp` |

module 侧保持领域名：`user` / `article` / `payment` / `statistics`。
