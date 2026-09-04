package port

import "context"

// TxManager 跨仓储本地事务端口：由 infra/database 实现，业务与 app 只依赖本接口。
//
// 约定：
//   - 由「用例入口」开启事务（app 用例，如 article.Create、payment.CompleteMockVIP）；
//   - 被编排的 Repository / 写方法不再自行 Begin，只通过 ctx 中的 client 参与同一事务；
//   - 嵌套 WithinTx 复用外层事务。
type TxManager interface {
	// WithinTx 在事务中执行 fn：成功则提交，失败则回滚。
	// 若 ctx 已处于事务中，则直接执行 fn，不新开事务。
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}
