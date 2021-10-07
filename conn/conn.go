package conn

import (
	"context"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

var (
	_ PgxConn = &pgxpool.Pool{}
	_ PgxConn = &pgx.Conn{}
	_ PgxConn = pgx.Tx(nil)
)

type (
	PgxConn interface {
		Begin(context.Context) (pgx.Tx, error)
		BeginFunc(ctx context.Context, f func(pgx.Tx) error) (err error)
		Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error)
		Query(ctx context.Context, sql string, optionsAndArgs ...interface{}) (pgx.Rows, error)
		QueryRow(ctx context.Context, sql string, optionsAndArgs ...interface{}) pgx.Row
		QueryFunc(ctx context.Context, sql string, args, scans []interface{}, f func(pgx.QueryFuncRow) error) (pgconn.CommandTag, error)
		SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults
		CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error)
	}
)
