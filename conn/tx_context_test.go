package conn

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/matryer/is"
)

func TestTxContext(t *testing.T) {
	t.Run("NewTxContext with nil transaction", func(t *testing.T) {
		is := is.New(t)

		ctx := t.Context()
		result := NewTxContext(ctx, nil)

		is.True(result == ctx)
	})

	t.Run("NewTxContext with transaction", func(t *testing.T) {
		is := is.New(t)

		tx := &pgxpool.Tx{}
		ctxWithTx := NewTxContext(t.Context(), tx)

		v, ok := TxFromContext(ctxWithTx)
		is.True(ok)
		is.True(v == tx)
	})

	t.Run("TxFromContext with no transaction", func(t *testing.T) {
		is := is.New(t)

		ctx := t.Context()
		tx, ok := TxFromContext(ctx)

		is.True(!ok)
		is.True(tx == nil)
	})

	t.Run("TxFromContext with wrong type in context", func(t *testing.T) {
		is := is.New(t)

		ctx := context.WithValue(t.Context(), txKey, "not a transaction")

		tx, ok := TxFromContext(ctx)

		is.True(!ok)
		is.True(tx == nil)
	})
}
