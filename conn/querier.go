package conn

import (
	"context"

	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/jackc/pgx/v5"
)

type Querier interface {
	Select(ctx context.Context, dst any, sql string, args ...any) error
	Get(ctx context.Context, dst any, sql string, args ...any) error
	Exec(ctx context.Context, sql string, args ...any) (int64, error)
	Tx(ctx context.Context, f func(q Querier) error, opts ...TxOption) error
	Conn(ctx context.Context) PgxConn
}

var _ Querier = &wrappedConn{}

type wrappedConn struct {
	conn    PgxConn
	scanAPI *pgxscan.API
}

func WrapConn(conn PgxConn, scanAPI *pgxscan.API) *wrappedConn {
	return &wrappedConn{
		conn:    conn,
		scanAPI: scanAPI,
	}
}

// Select iterates all rows to the end. After iterating it closes the rows.
// It expects that destination should be a slice. For each row it scans data and appends it to the destination slice.
// Select supports both types of slices: slice of structs by a pointer and slice of structs by value,
// Before starting, Select resets the destination slice,
// so if it's not empty it will overwrite all existing elements.
func (n *wrappedConn) Select(ctx context.Context, dst interface{}, sql string, args ...interface{}) error {
	rows, err := n.Conn(ctx).Query(ctx, sql, args...)
	if err != nil {
		return err
	}

	return n.scanAPI.ScanAll(dst, rows)
}

// Get iterates all rows to the end and makes sure that there was exactly one row
// otherwise it returns an error.
// It scans data from single row into the destination.
func (n *wrappedConn) Get(ctx context.Context, dst interface{}, sql string, args ...interface{}) error {
	rows, err := n.Conn(ctx).Query(ctx, sql, args...)
	if err != nil {
		return err
	}

	return n.scanAPI.ScanOne(dst, rows)
}

// Exec executes a query without returning any rows and return affected rows.
func (n *wrappedConn) Exec(ctx context.Context, sql string, args ...interface{}) (int64, error) {
	res, err := n.Conn(ctx).Exec(ctx, sql, args...)
	if err != nil {
		return 0, err
	}

	return res.RowsAffected(), nil
}

// Tx starts a transaction and calls f. If f does not return an error the transaction is committed.
// If f returns an error the transaction is rolled back.
func (n *wrappedConn) Tx(ctx context.Context, f func(q Querier) error, opts ...TxOption) error {
	txOpts := &TxOptions{}
	for _, o := range opts {
		o(txOpts)
	}
	err := pgx.BeginFunc(ctx, n.Conn(ctx), func(txx pgx.Tx) error {
		if err := txOpts.Apply(ctx, txx); err != nil {
			return err
		}

		return f(WrapConn(txx, n.scanAPI))
	})

	return err
}

func (n *wrappedConn) Conn(ctx context.Context) PgxConn {
	if conn, ok := TxFromContext(ctx); ok {
		return conn
	}

	return n.conn
}
