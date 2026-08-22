package test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"wood-passage-creator/ent"
	"wood-passage-creator/internal/config"
	"wood-passage-creator/internal/infra/database"
)

var testDB *database.DB

func TestMain(m *testing.M) {
	var err error
	cfg := config.LoadConfig()
	testDB, err = database.New(&cfg.Database)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect db: %v\n", err)
		os.Exit(1)
	}

	code := m.Run()
	testDB.Close()
	os.Exit(code)
}

func Client() *ent.Client {
	return testDB.Client
}

func TestDBConnection(t *testing.T) {
	ctx := context.Background()

	if err := testDB.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Log("migration ok")
}
