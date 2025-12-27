package database

import (
	"context"
	"fmt"

	"github.com/ilkin0/gzln/internal/repository/sqlc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TxRunner = func(ctx context.Context, fn func(q *sqlc.Queries) error) error

func NewTxRunner(pool *pgxpool.Pool) TxRunner {
	return func(ctx context.Context, fn func(q *sqlc.Queries) error) error {
		return RunWithTx(ctx, pool, fn)
	}
}

func RunWithTx(ctx context.Context, pool *pgxpool.Pool, fn func(q *sqlc.Queries) error) error {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}

	q := sqlc.New(tx)

	if err := fn(q); err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			return fmt.Errorf("rollback error: %v; original: %w", rbErr, err)
		}
		return err
	}

	return tx.Commit(ctx)
}
