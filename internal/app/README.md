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

跨 module 本地事务：`port.WithinTx` 全局访问（main `database.InitTxManager`）；`article.Create`、`payment.CompleteMockVIP` 已使用。
