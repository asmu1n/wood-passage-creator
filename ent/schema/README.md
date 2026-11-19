# ent schema

手写表结构只放这里，改完后：

```bash
go generate ./ent
```

服务启动时 `database.Migrate` 会按当前 schema 建表（以项目现状为准）。

## 当前实体

| Schema | 表名 | 说明 |
|--------|------|------|
| `User` | `users` | 用户（账号/密码哈希/资料/角色/配额/VIP/软删除） |
| `Article` | `articles` | 文章生成任务（主题/标题/大纲/正文/配图/状态阶段/软删除） |
| `AgentLog` | `agent_logs` | 智能体执行日志（相对原 model 增补 `article_id` FK） |
| `PaymentRecord` | `payment_records` | 支付记录（Stripe session / 金额 / 状态 / 产品类型） |

字段语义对齐 `ai-passage-creator` 的 GORM model，列名采用 snake_case。

## 关系（edges）

```text
User 1 ─── N Article
User 1 ─── N PaymentRecord
Article 1 ─── N AgentLog
```

| 边 | 基数 | 外键列 | 说明 |
|----|------|--------|------|
| `User.articles` ↔ `Article.user` | 1:N | `articles.user_id` | 文章所属用户 |
| `User.payment_records` ↔ `PaymentRecord.user` | 1:N | `payment_records.user_id` | 支付所属用户 |
| `Article.agent_logs` ↔ `AgentLog.article` | 1:N | `agent_logs.article_id` | 日志所属文章 |

说明：原 GORM `AgentLog` 仅有 `taskId`（对应 `articles.task_id` 业务唯一键）。Ent edge 只能指向对方主键，故 schema 增加 `article_id → articles.id`；`task_id` 仍保留并建索引，便于按任务检索。

## 未建模内容（非表实体）

以下原 `model` 包文件是请求体 / VO / 工具类型，不建 Ent schema：

- `request.go` / `article_request.go`（API DTO）
- `image.go`（运行时图片结构）
- `statistics.go`（统计 VO）
- `util.go` / `article.go` 内嵌的 TitleOption 等（JSON 结构，落在 Article 的 jsonb 字段）

新增实体：在本目录添加 `xxx.go` → `go generate ./ent`。
