package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Matthew11K/Comments-Service/internal/application/txctx"
	"github.com/Matthew11K/Comments-Service/internal/domain"
)

type TxManager struct {
	pool *pgxpool.Pool
}

func NewTxManager(pool *pgxpool.Pool) *TxManager {
	return &TxManager{pool: pool}
}

func (m *TxManager) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	tx, err := m.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return &domain.OperationError{
			Op:  "begin postgres transaction",
			Err: err,
		}
	}

	txCtx := withTx(ctx, tx)
	txCtx, registry := txctx.WithRegistry(txCtx)

	if err := fn(txCtx); err != nil {
		if rollbackErr := tx.Rollback(ctx); rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
			return &domain.OperationError{
				Op:  "rollback postgres transaction",
				Err: rollbackErr,
			}
		}

		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return &domain.OperationError{
			Op:  "commit postgres transaction",
			Err: err,
		}
	}

	registry.Run(ctx)
	return nil
}
