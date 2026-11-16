# 业务模块（module）

**领域层 + 持久化**：实体、枚举、领域工具、`Repository` 接口、`repo` 实现（通过 `database.DB.Cli(ctx)` 选择 Client）。
**不含** 应用 Service、**不含** HTTP、**不含** `*Request`。

`article` 的 AI 编排、Dotprompt 和类型化进度契约统一放在
`internal/app/article/workflow`；阶段触发、持久化和进度投影由 `app/article` 负责。
领域层保留文章、配额相关枚举、生成警告和业务执行日志契约，不依赖 AI SDK。
