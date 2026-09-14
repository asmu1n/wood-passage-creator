package database

import (
	"context"
	"fmt"

	"wood-passage-creator/ent"
	"wood-passage-creator/internal/config"
	"wood-passage-creator/internal/port"

	_ "github.com/lib/pq"
)

type DB interface {
	Migrate(ctx context.Context) error
	Close() error
	Cli(context.Context) *ent.Client
	port.TxManager
}

type db struct {
	Client *ent.Client
}

func New(config *config.DatabaseConfig) (*db, error) {

	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		config.Host, config.Port, config.User, config.Password, config.DBName)

	client, err := ent.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	return &db{Client: client}, nil
}

func (d *db) Migrate(ctx context.Context) error {
	return d.Client.Schema.Create(ctx)
}

func (d *db) Close() error {
	if d.Client != nil {
		return d.Client.Close()
	}
	return nil
}
