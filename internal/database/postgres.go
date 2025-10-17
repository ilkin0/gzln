package database

import (
	"context"
	"fmt"
	"os"

	"github.com/ilkin0/gzln/internal/repository/sqlc"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Database struct {
	Pool    *pgxpool.Pool
	Queries *sqlc.Queries
}

func NewDatabase(ctx context.Context) (*Database, error) {
	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DB_URL environment variable is not set")
	}

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	queries := sqlc.New(pool)

	return &Database{
		Pool:    pool,
		Queries: queries,
	}, nil
}
