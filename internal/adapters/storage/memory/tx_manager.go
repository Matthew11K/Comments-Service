package memory

import (
	"context"

	"github.com/Matthew11K/Comments-Service/internal/application/txctx"
)

type TxManager struct{}

func NewTxManager() *TxManager {
	return &TxManager{}
}

func (*TxManager) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	txCtx, registry := txctx.WithRegistry(ctx)

	if err := fn(txCtx); err != nil {
		return err
	}

	registry.Run(ctx)
	return nil
}
