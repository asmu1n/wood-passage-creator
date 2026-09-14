package database

import (
	"context"
	"errors"
	"fmt"

	"wood-passage-creator/ent"
	"wood-passage-creator/internal/port"
)

type txCtxKey struct{}
type txState struct {
	client      *ent.Client
	afterCommit []port.AfterCommitFunc
}

func txStateFrom(ctx context.Context) *txState {
	state, _ := ctx.Value(txCtxKey{}).(*txState)
	return state
}

// Cli 解析当前应使用的 ent Client：
// 若 ctx 由 WithinTx 注入了事务 Client 则用之，否则回退到进程级根 Client。
// 所有 module/*/repo 读写均应经此函数取 Client，以便日后跨 module 共享事务。
func (db *db) Cli(ctx context.Context) *ent.Client {
	if s := txStateFrom(ctx); s != nil {
		return s.client
	}
	return db.Client
}

func (db *db) AfterCommit(
	ctx context.Context,
	fn port.AfterCommitFunc,
) {
	if fn == nil {
		return
	}

	if state := txStateFrom(ctx); state != nil {
		state.afterCommit = append(state.afterCommit, fn)

	} else {
		// 不在事务内，前面的数据库操作已经完成
		fn(ctx)
	}

}

func (db *db) WithinTx(
	ctx context.Context,
	fn func(context.Context) error,
) error {
	if db == nil || db.Client == nil {
		return fmt.Errorf("database: nil transaction manager")
	}
	if state := txStateFrom(ctx); state != nil {
		return fn(ctx)
	}

	tx, err := db.Client.Tx(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	// 同时覆盖普通错误和 panic。
	defer func() {
		_ = tx.Rollback()
	}()

	state := &txState{
		client: tx.Client(),
	}
	txCtx := context.WithValue(ctx, txCtxKey{}, state)

	if err := fn(txCtx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return errors.Join(err, rbErr)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	// 必须传原始 ctx，不能传含有已提交 ent client 的 txCtx。
	for i := range state.afterCommit {
		state.afterCommit[i](ctx)
	}

	return nil
}
