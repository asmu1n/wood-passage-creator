package database

import (
	"context"
	"fmt"

	"wood-passage-creator/ent"
	"wood-passage-creator/internal/port"
)

type txCtxKey struct{}

type txManager struct {
	client *ent.Client
}

// NewTxManager 使用根 Client 创建事务管理器。
func NewTxManager(client *ent.Client) port.TxManager {
	return &txManager{client: client}
}

// ClientFrom 解析当前应使用的 ent Client：
// 若 ctx 由 WithinTx 注入了事务 Client 则用之，否则回退到 fallback（进程级根 Client）。
// 所有 module/*/repo 读写均应经此函数取 Client，以便日后跨 module 共享事务。
func ClientFrom(ctx context.Context, fallback *ent.Client) *ent.Client {
	if c, ok := ctx.Value(txCtxKey{}).(*ent.Client); ok && c != nil {
		return c
	}
	return fallback
}

// WithinTx 实现 port.TxManager。
func (m *txManager) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if m == nil || m.client == nil {
		return fmt.Errorf("database: nil tx manager")
	}
	if _, ok := ctx.Value(txCtxKey{}).(*ent.Client); ok {
		return fn(ctx)
	}

	tx, err := m.client.Tx(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	txCtx := context.WithValue(ctx, txCtxKey{}, tx.Client())
	if err := fn(txCtx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("rollback after error: %v (original: %w)", rbErr, err)
		}
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}
