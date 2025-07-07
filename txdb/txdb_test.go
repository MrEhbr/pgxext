package txdb

import (
	"context"
	"errors"
	"testing"

	"github.com/MrEhbr/pgxext/cluster"
	"github.com/MrEhbr/pgxext/conn"
	"github.com/jackc/pgx/v5"
	"github.com/matryer/is"
)

func TestTxDB(t *testing.T) {
	t.Run("beginOnce", func(t *testing.T) {
		conn.TestRunner().RunTest(t.Context(), t, func(ctx context.Context, t testing.TB, pgxConn *pgx.Conn) {
			its := is.New(t)

			db, err := cluster.Open([]string{pgxConn.Config().ConnString()})
			its.NoErr(err)

			txdb := New(db)
			defer txdb.Close()

			tx, err := txdb.beginOnce(ctx)
			its.NoErr(err)
			its.True(tx != nil)
			its.Equal(txdb.tx, tx)
			tx2, err := txdb.beginOnce(ctx)
			its.NoErr(err)
			its.Equal(tx2, tx)
		})
	})
	t.Run("changes visible within transaction", func(t *testing.T) {
		conn.TestRunner().RunTest(t.Context(), t, func(ctx context.Context, t testing.TB, pgxConn *pgx.Conn) {
			its := is.New(t)

			db, err := cluster.Open([]string{pgxConn.Config().ConnString()})
			its.NoErr(err)

			_, err = db.Exec(ctx, `CREATE TEMP TABLE foo (value TEXT)`)
			its.NoErr(err)

			txdb := New(db)
			defer txdb.Close()

			_, err = txdb.Exec(ctx, `INSERT INTO foo (value) VALUES ($1)`, "test")
			its.NoErr(err)

			var exists bool
			err = db.Get(ctx, &exists, `SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'foo')`)
			its.NoErr(err)
			its.True(!exists)
		})
	})

	t.Run("changes rolled back on Rollback", func(t *testing.T) {
		conn.TestRunner().RunTest(t.Context(), t, func(ctx context.Context, t testing.TB, pgxConn *pgx.Conn) {
			its := is.New(t)

			db, err := cluster.Open([]string{pgxConn.Config().ConnString()})
			its.NoErr(err)

			_, err = db.Exec(ctx, `CREATE TEMP TABLE foo_bar (value TEXT)`)
			its.NoErr(err)

			txdb := New(db)
			_, err = txdb.Exec(ctx, `INSERT INTO foo_bar (value) VALUES ($1)`, "should_rollback")
			its.NoErr(err)

			var count int
			err = txdb.Get(ctx, &count, `SELECT COUNT(*) FROM foo_bar WHERE value = $1`, "should_rollback")
			its.NoErr(err)
			its.Equal(count, 1)

			err = txdb.Rollback(ctx)
			its.NoErr(err)

			err = db.Get(ctx, &count, `SELECT COUNT(*) FROM foo_bar WHERE value = $1`, "should_rollback")
			its.NoErr(err)
			its.Equal(count, 0)
		})
	})

	t.Run("nested transactions with rollback", func(t *testing.T) {
		conn.TestRunner().RunTest(t.Context(), t, func(ctx context.Context, t testing.TB, pgxConn *pgx.Conn) {
			its := is.New(t)

			db, err := cluster.Open([]string{pgxConn.Config().ConnString()})
			its.NoErr(err)

			_, err = db.Exec(ctx, `CREATE TEMP TABLE foo_baz (value TEXT)`)
			its.NoErr(err)

			txdb := New(db)
			defer txdb.Close()

			_, err = txdb.Exec(ctx, `INSERT INTO foo_baz (value) VALUES ($1)`, "keep")
			its.NoErr(err)

			err = txdb.Tx(ctx, func(q conn.Querier) error {
				_, err = q.Exec(ctx, `INSERT INTO foo_baz (value) VALUES ($1)`, "discard")
				its.NoErr(err)
				return errors.New("rollback nested")
			})
			its.True(err != nil)

			var count int
			err = txdb.Get(ctx, &count, `SELECT COUNT(*) FROM foo_baz WHERE value = $1`, "keep")
			its.NoErr(err)
			its.Equal(count, 1)

			err = txdb.Get(ctx, &count, `SELECT COUNT(*) FROM foo_baz WHERE value = $1`, "discard")
			its.NoErr(err)
			its.Equal(count, 0)
		})
	})
}
