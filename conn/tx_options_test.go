package conn

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/matryer/is"
)

func TestTxOptions(t *testing.T) {
	const checkQuery = `
SELECT
  CURRENT_SETTING('idle_in_transaction_session_timeout') AS idle_in_txn_timeout,
  CURRENT_SETTING('statement_timeout')                   AS stmt_timeout;
`
	type optsRow struct {
		IdleInTxnTimeout string `db:"idle_in_txn_timeout"`
		StmtTimeout      string `db:"stmt_timeout"`
	}

	t.Run("options applied", func(t *testing.T) {
		TestRunner().RunTest(t.Context(), t, func(ctx context.Context, t testing.TB, conn *pgx.Conn) {
			its := is.New(t)
			querier := WrapConn(conn, pgxscan.DefaultAPI)
			var (
				idleInTxnTimeout = time.Second
				statementTimeout = 2 * time.Second
			)

			err := querier.Tx(t.Context(), func(q Querier) error {
				var opts optsRow
				err := q.Get(ctx, &opts, checkQuery)
				its.NoErr(err)
				its.Equal(opts.IdleInTxnTimeout, idleInTxnTimeout.String())
				its.Equal(opts.StmtTimeout, statementTimeout.String())

				return nil
			}, TransactionTimeout(idleInTxnTimeout), StatementTimeout(statementTimeout))

			its.NoErr(err)
		})
	})
	t.Run("options not applied", func(t *testing.T) {
		TestRunner().RunTest(t.Context(), t, func(ctx context.Context, t testing.TB, conn *pgx.Conn) {
			its := is.New(t)
			querier := WrapConn(conn, pgxscan.DefaultAPI)
			err := querier.Tx(t.Context(), func(q Querier) error {
				var opts optsRow
				err := q.Get(ctx, &opts, checkQuery)
				its.NoErr(err)
				its.Equal(opts.IdleInTxnTimeout, "0")
				its.Equal(opts.StmtTimeout, "0")

				return nil
			}, TransactionTimeout(0), StatementTimeout(0))

			its.NoErr(err)
		})
	})

	t.Run("timeouts", func(t *testing.T) {
		t.Run("transaction timeout", func(t *testing.T) {
			TestRunner().RunTest(t.Context(), t, func(ctx context.Context, t testing.TB, conn *pgx.Conn) {
				its := is.New(t)
				querier := WrapConn(conn, pgxscan.DefaultAPI)
				err := querier.Tx(ctx, func(_ Querier) error {
					time.Sleep(100 * time.Millisecond)
					return nil
				}, TransactionTimeout(50*time.Millisecond))

				its.True(err != nil)
				var pgErr *pgconn.PgError
				its.True(errors.As(err, &pgErr)) // must be pg error
				its.True(pgErr.Code == "25P03")  // 25P03 - idle_in_transaction_session_timeout error code
			})
		})
		t.Run("statement timeout", func(t *testing.T) {
			TestRunner().RunTest(t.Context(), t, func(ctx context.Context, t testing.TB, conn *pgx.Conn) {
				its := is.New(t)
				querier := WrapConn(conn, pgxscan.DefaultAPI)

				err := querier.Tx(ctx, func(q Querier) error {
					_, err := q.Exec(ctx, "SELECT pg_sleep(1)")
					return err
				}, StatementTimeout(50*time.Millisecond))

				its.True(err != nil)
				var pgErr *pgconn.PgError
				its.True(errors.As(err, &pgErr)) // must be pg error
				its.True(pgErr.Code == "57014")  // 57014 - query_canceled error code
			})
		})
	})
}
