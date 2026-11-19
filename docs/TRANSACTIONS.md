# 本地事务与提交后动作

项目使用同一个 Postgres 与根 `ent.Client`。module 是代码分层，不代表分库；跨 module 写入通过共享的本地事务保证一致性。

## 组成

| 组件 | 位置 | 职责 |
|------|------|------|
| `app.Runtime` | `internal/app/runtime.go` | 保存进程级共享能力，启动时初始化一次 |
| `port.TxManager` | `internal/port/tx.go` | 声明 `WithinTx` 与 `AfterCommit` 能力 |
| `database.db` | `internal/infra/database` | 基于 ent 实现事务、Client 选择和提交后回调 |
| app 用例 | `internal/app/*` | 决定事务边界并编排跨 module 写入 |
| module repo | `internal/module/*/repo` | 通过 `Cli(ctx)` 自动使用根 Client 或事务 Client |

`Runtime` 是全局单例，但只保存线程安全的根 `TxManager`。当前事务、事务 Client 和提交后回调都保存在请求 `context` 中，不是全局状态，因此不同请求不会共享事务。

启动时由 composition root 初始化：

```go
if err := app.InitRuntime(app.Runtime{TxManager: db}); err != nil {
    logger.Fatal("init app runtime failed", logger.FieldErr, err)
}
```

## 跨 module 事务

只有 app 用例决定何时开启事务：

```go
err := app.CurrentRuntime().WithinTx(ctx, func(txCtx context.Context) error {
    if err := repoA.Write(txCtx); err != nil {
        return err
    }
    return repoB.Write(txCtx)
})
```

执行过程：

```text
CurrentRuntime().WithinTx(ctx)
  ├─ Begin ent.Tx
  ├─ txCtx 保存事务 Client 与 AfterCommit 队列
  ├─ repoA.Write(txCtx) → Cli(txCtx) → 事务 Client
  ├─ repoB.Write(txCtx) → Cli(txCtx) → 同一事务 Client
  ├─ 失败或 panic → Rollback，丢弃 AfterCommit 队列
  └─ Commit 成功 → 使用原始非事务 ctx 执行 AfterCommit 队列
```

嵌套 `WithinTx` 不创建新事务，而是复用最外层事务。项目当前不提供 savepoint，因此内层成功不代表事务已经提交，任意外层错误仍会回滚全部写入。

## AfterCommit

数据写入成功后需要删除缓存、记录成功事件或触发轻量通知时，由拥有该业务规则的方法登记 `AfterCommit`：

```go
out, err := repo.Update(ctx, params)
if err != nil {
    return nil, err
}

app.CurrentRuntime().AfterCommit(ctx, func(commitCtx context.Context) {
    invalidateCache(commitCtx)
})
return out, nil
```

其行为取决于当前 context：

| 调用场景 | 行为 |
|----------|------|
| 不在事务中 | 数据库单条语句已由 autocommit 提交，回调立即执行 |
| 位于事务中 | 回调加入当前事务队列，最外层 Commit 成功后执行 |
| 事务回滚 | 回调被丢弃 |
| 嵌套事务 | 回调归属最外层事务，只在最终 Commit 后执行 |

这使 `GrantVIP` 等方法能够自行维护缓存策略，而调用方无需区分它是独立执行还是参与支付事务。

`AfterCommit` 是进程内最佳努力机制：数据库提交后若进程立即崩溃，尚未执行的回调会丢失。缓存失效可以依赖 TTL 兜底；必须可靠投递的业务事件应使用事务 Outbox。

## 当前事务用例

| 用例 | 原子写入 | 提交后动作 |
|------|----------|------------|
| `article.Create` | 扣减用户配额 + 插入文章 | 失效统计缓存、启动 Phase1 |
| `payment.CompleteMockVIP` | 支付记录成功 + 用户升级 VIP | `GrantVIP` 自行失效统计缓存 |

## 约束

1. repo 必须使用 `Cli(ctx)`，不得保存或直接选择事务 Client。
2. repo 不得自行 Begin、Commit 或 Rollback。
3. 事务回调中的所有调用必须继续传递收到的 `txCtx`。
4. 不要在事务中启动 goroutine，也不要把 `txCtx` 保存到事务之外。
5. LLM、外部 HTTP、长等待、缓存操作和异步任务应放在事务外或 `AfterCommit` 中。
6. 单次原子 SQL、只读操作不必额外套事务。

## 常见错误

- 在事务回调中误用原始 `ctx`，导致某个 repo 使用根 Client。
- 在 repo 内自行开启事务，破坏 app 用例定义的原子边界。
- Commit 前删除缓存，其他请求可能把旧数据重新写回缓存。
- 在 `AfterCommit` 中继续使用 `txCtx`，访问已经结束的事务 Client。
- 把需要可靠投递的消息仅放在内存回调中。
