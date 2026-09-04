package port

import (
	"context"
	"fmt"
	"sync"
)

// TxManager 跨仓储本地事务端口：由 infra/database 实现。
//
// 约定：
//   - 由「用例入口」开启事务（app 用例）；
//   - Repository 经 ctx + ClientFrom 参与同一事务，禁止自行 Begin；
//   - 嵌套 WithinTx 复用外层事务。
//
// 访问方式与 logger 类似：main 装配一次 SetTxManager，业务用包级 WithinTx，
// 避免每个 Service 构造函数都注入 tx（事务编排几乎不需要单测替身）。
type TxManager interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}

var (
	txMu      sync.RWMutex
	defaultTx TxManager
)

// SetTxManager 由 cmd 在连接数据库后设置全局事务管理器（进程内一次）。
func SetTxManager(m TxManager) {
	txMu.Lock()
	defaultTx = m
	txMu.Unlock()
}

// Tx 返回当前全局 TxManager（只读；业务优先用 WithinTx）。
func Tx() TxManager {
	txMu.RLock()
	defer txMu.RUnlock()
	return defaultTx
}

// WithinTx 使用全局 TxManager 执行事务。未 SetTxManager 时返回错误。
func WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	txMu.RLock()
	m := defaultTx
	txMu.RUnlock()
	if m == nil {
		return fmt.Errorf("port: tx manager not initialized")
	}
	return m.WithinTx(ctx, fn)
}
