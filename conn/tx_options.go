package conn

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
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

// Apply applies the timeout configuration to the given transaction.
func (opts *TxOptions) Apply(ctx context.Context, tx pgx.Tx) error {
	if opts.TransactionTimeout > 0 {
		if _, err := tx.Exec(ctx, transactionTimeoutQuery, pgx.QueryExecModeSimpleProtocol, opts.TransactionTimeout); err != nil {
			return fmt.Errorf("set transaction timeout: %w", err)
		}
	}
	if opts.StatementTimeout > 0 {
		if _, err := tx.Exec(ctx, statementTimeoutQuery, pgx.QueryExecModeSimpleProtocol, opts.StatementTimeout); err != nil {
			return fmt.Errorf("set statement timeout: %w", err)
		}
	}
	return nil
}
