package conn

import (
	"context"
	"testing"

	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/jackc/pgx/v5"
	"github.com/matryer/is"
)

func TestTxManager(t *testing.T) {
	t.Run("commit transaction", func(t *testing.T) {
		TestRunner().RunTest(t.Context(), t, func(ctx context.Context, t testing.TB, conn *pgx.Conn) {
			its := is.New(t)
			querier := WrapConn(conn, pgxscan.DefaultAPI)
			txMgr := NewTxManager(querier)

			_, err := querier.Exec(ctx, `CREATE TEMP TABLE foo (value TEXT)`)
			its.NoErr(err)

			txCtx, commit, rollback, txErr := txMgr.NewTx(ctx)
			its.NoErr(txErr)

			_, err = querier.Exec(txCtx, "INSERT INTO foo (value) VALUES ($1)", "bar")
			its.NoErr(err)

			err = commit()
			its.NoErr(err)

			var count int
			err = querier.Get(ctx, &count, "SELECT COUNT(*) FROM foo WHERE value = $1", "bar")
			its.NoErr(err)
			its.Equal(count, 1)

			_ = rollback()
		})
	})

	t.Run("rollback transaction", func(t *testing.T) {
		TestRunner().RunTest(t.Context(), t, func(ctx context.Context, t testing.TB, conn *pgx.Conn) {
			its := is.New(t)
			querier := WrapConn(conn, pgxscan.DefaultAPI)
			txMgr := NewTxManager(querier)

			_, err := querier.Exec(ctx, `CREATE TEMP TABLE foo (value TEXT)`)
			its.NoErr(err)

			txCtx, commit, rollback, txErr := txMgr.NewTx(ctx)
			its.NoErr(txErr)

			_, err = querier.Exec(txCtx, "INSERT INTO foo (value) VALUES ($1)", "baz")
			its.NoErr(err)

			err = rollback()
			its.NoErr(err)

			var count int
			err = querier.Get(ctx, &count, "SELECT COUNT(*) FROM foo WHERE value = $1", "baz")
			its.NoErr(err)
			its.Equal(count, 0)

			_ = commit()
		})
	})
}
