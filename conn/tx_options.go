package conn

import (
	"time"
)

const (
	transactionTimeoutQuery = "SET local idle_in_transaction_session_timeout = $1"
	statementTimeoutQuery   = "SET local statement_timeout = $1"
)

// Options for tx.
type TxOptions struct {
	TransactionTimeout int64
	StatementTimeout   int64
}

// Option func.
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
