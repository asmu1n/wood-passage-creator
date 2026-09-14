package database

import (
	"context"
	"testing"

	"wood-passage-creator/ent"
)

func TestCliUsesTransactionClient(t *testing.T) {
	rootClient := &ent.Client{}
	txClient := &ent.Client{}
	database := &db{Client: rootClient}

	if got := database.Cli(context.Background()); got != rootClient {
		t.Fatal("expected root client outside transaction")
	}

	ctx := context.WithValue(context.Background(), txCtxKey{}, &txState{client: txClient})
	if got := database.Cli(ctx); got != txClient {
		t.Fatal("expected transaction client")
	}
}

func TestAfterCommitImmediateOutsideTransaction(t *testing.T) {
	database := &db{}
	called := false

	database.AfterCommit(context.Background(), func(context.Context) {
		called = true
	})

	if !called {
		t.Fatal("expected callback to run immediately")
	}
}

func TestAfterCommitQueuedInTransaction(t *testing.T) {
	database := &db{}
	state := &txState{}
	ctx := context.WithValue(context.Background(), txCtxKey{}, state)
	called := false

	database.AfterCommit(ctx, func(context.Context) {
		called = true
	})

	if called {
		t.Fatal("callback ran before commit")
	}
	if len(state.afterCommit) != 1 {
		t.Fatalf("expected one queued callback, got %d", len(state.afterCommit))
	}

	state.afterCommit[0](context.Background())
	if !called {
		t.Fatal("expected queued callback to run")
	}
}

func TestWithinTxRejectsNilManager(t *testing.T) {
	var database *db
	if err := database.WithinTx(context.Background(), func(context.Context) error { return nil }); err == nil {
		t.Fatal("expected nil transaction manager error")
	}
}
