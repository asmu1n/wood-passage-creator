# 本地事务与跨 module 一致性

本项目使用 **同一 Postgres + 同一 ent.Client**。  
module 在代码上垂直分片，**并不**等于分库；跨 module 的强一致写通过 **共享事务** 完成。

## 组件

| 组件 | 位置 | 职责 |
|------|------|------|
| `port.TxManager` / `port.WithinTx` | `internal/port/tx.go` | 全局事务入口（main `database.InitTxManager` 注册，用法类似 logger） |
| `database.ClientFrom` | `internal/infra/database/tx.go` | repo 从 ctx 取当前 Client（事务内或根 Client） |
| 用例 | `internal/app/*` | **唯一**决定何时 `WithinTx` |

```text
port.WithinTx(ctx)
  ├─ begin *ent.Tx
  ├─ ctx' = WithClient(tx.Client())
  ├─ repoA.Write(ctx')  → ClientFrom → 同一 Tx
  ├─ repoB.Write(ctx')  → ClientFrom → 同一 Tx
  └─ commit / rollback
```

## 规则

1. 所有 `module/*/repo` 经 `ClientFrom(ctx, r.client)` 取 Client。  
2. **禁止** repo 内 Begin/Commit。  
3. 仅 app 用例开事务；嵌套 `WithinTx` 复用外层。  
4. 事务内不做 LLM / 外部 HTTP / 长等待；异步与缓存失效放在 **Commit 之后**。

## 已接入用例

| 用例 | 原子写 |
|------|--------|
| `app/article.Create` | 扣配额 + 插入文章 |
| `app/payment.CompleteMockVIP` | MarkSucceeded + GrantVIP |

单域只读/单表写不必强行套事务。

## 反模式

- 在 Service 里先写完 A 再调 B 且双方用根 Client → 非同一事务  
- repo 直接依赖另一 module 的 repo 实现  
- 事务里调 LLM 并等待  
