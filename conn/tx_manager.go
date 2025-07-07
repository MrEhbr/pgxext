package conn

import (
	"context"
)

// TxManager provides transaction management abstraction for service layer.
type TxManager interface {
	NewTx(ctx context.Context, opts ...TxOption) (context.Context, func() error, func() error, error)
}

type txManager struct {
	querier Querier
}

// NewTxManager creates a new transaction manager that wraps the given querier.
func NewTxManager(q Querier) TxManager {
	return &txManager{
		querier: q,
	}
}

// NewTx starts a new transaction and returns a context with the transaction embedded,
// along with commit and rollback functions.
func (tm *txManager) NewTx(ctx context.Context, opts ...TxOption) (context.Context, func() error, func() error, error) {
	txOpts := &TxOptions{}
	for _, o := range opts {
		o(txOpts)
	}

	conn := tm.querier.Conn(ctx)
	tx, err := conn.Begin(ctx)
	if err != nil {
		return ctx, nil, nil, err
	}

	if applyErr := txOpts.Apply(ctx, tx); applyErr != nil {
		_ = tx.Rollback(ctx)
		return ctx, nil, nil, applyErr
	}

	txCtx := NewTxContext(ctx, tx)

	commit := func() error {
		return tx.Commit(ctx)
	}

	rollback := func() error {
		return tx.Rollback(ctx)
	}

	return txCtx, commit, rollback, nil
}
