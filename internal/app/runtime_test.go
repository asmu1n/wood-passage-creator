package app

import (
	"context"
	"testing"

	"wood-passage-creator/internal/port"
)

type fakeTxManager struct{}

func (fakeTxManager) AfterCommit(ctx context.Context, fn port.AfterCommitFunc) {
	fn(ctx)
}

func (fakeTxManager) WithinTx(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

func TestRuntimeInitialization(t *testing.T) {
	currentRuntime.Store(nil)
	t.Cleanup(func() { currentRuntime.Store(nil) })

	tx := fakeTxManager{}
	if err := InitRuntime(Runtime{tx}); err != nil {
		t.Fatalf("initialize runtime: %v", err)
	}
	if CurrentRuntime().TxManager == nil {
		t.Fatal("expected transaction manager")
	}
	if err := InitRuntime(Runtime{tx}); err == nil {
		t.Fatal("expected duplicate initialization error")
	}
}

func TestRuntimeRejectsNilTransactionManager(t *testing.T) {
	currentRuntime.Store(nil)
	t.Cleanup(func() { currentRuntime.Store(nil) })

	if err := InitRuntime(Runtime{}); err == nil {
		t.Fatal("expected nil transaction manager error")
	}
}
