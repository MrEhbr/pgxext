package conn

import (
	"context"

	"github.com/jackc/pgx/v5"
)

type txKeyType uint8

const (
	txKey txKeyType = 0
)

// NewTxContext returns a new context carrying the transaction connection.
func NewTxContext(ctx context.Context, tx pgx.Tx) context.Context {
	if tx == nil {
		return ctx
	}
	return context.WithValue(ctx, txKey, tx)
}

// TxFromContext extracts the transaction connection if present.
func TxFromContext(ctx context.Context) (pgx.Tx, bool) {
	v, ok := ctx.Value(txKey).(pgx.Tx)
	return v, ok
}
