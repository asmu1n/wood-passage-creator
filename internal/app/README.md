# 应用层（app）

**全部用例**与 **`*Request` 入参**放在本目录。  
`module` 只保留领域实体、领域工具、`Repository` 与 `repo` 实现（article 另含 agent/prompt）。  
HTTP 在 `httpapi/api`，**只依赖 app**。

| 包 | 职责 |
|----|------|
| `auth` | 注册 / 登录 |
| `user` | 资料、管理、配额、VIP |
| `article` | 文章流水线用例（agent 仍在 module） |
| `payment` | Mock 支付 / 列表 |
| `statistics` | 管理端概览 |

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
