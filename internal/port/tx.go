package port

import (
	"context"
)

type AfterCommitFunc func(context.Context)

// TxManager 跨仓储本地事务端口：由 infra/database 实现。
//
// 约定：
//   - 由「用例入口」开启事务（app 用例）；
//   - Repository 经 ctx + database.Cli 参与同一事务，禁止自行 Begin；
//   - 嵌套 WithinTx 复用外层事务。
//
// 根 TxManager 由 app.Runtime 在启动阶段统一持有；当前事务与提交后回调
// 始终保存在请求 context 中，不使用进程级事务状态。
type TxManager interface {
	AfterCommit(ctx context.Context, fn AfterCommitFunc)
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}
