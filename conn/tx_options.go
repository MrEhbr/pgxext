package conn

import (
	"context"
	"time"

	"github.com/jackc/pgx/v4"
)

const (
	transactionTimeoutQuery = "SET local idle_in_transaction_session_timeout = $1"
	statementTimeoutQuery   = "SET local statement_timeout = $1"
)

// TxOptions contains options for transactions.
type TxOptions struct {
	TransactionTimeout int64
	StatementTimeout   int64
}

// TxOption is a function that configures TxOptions.
type TxOption func(*TxOptions)

// TransactionTimeout sets idle_in_transaction_session_timeout.
func TransactionTimeout(d time.Duration) TxOption {
	return func(o *TxOptions) {
		o.TransactionTimeout = d.Milliseconds()
	}
}

// StatementTimeout sets transaction statement_timeout.
func StatementTimeout(d time.Duration) TxOption {
	return func(o *TxOptions) {
		o.StatementTimeout = d.Milliseconds()
	}
}

// Context options for transaction

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
