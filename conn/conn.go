package conn

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	_ PgxConn = &pgxpool.Pool{}
	_ PgxConn = &pgx.Conn{}
	_ PgxConn = pgx.Tx(nil)
)

type (
	PgxConn interface {
		Begin(context.Context) (pgx.Tx, error)
		Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
		Query(ctx context.Context, sql string, optionsAndArgs ...any) (pgx.Rows, error)
		QueryRow(ctx context.Context, sql string, optionsAndArgs ...any) pgx.Row
		SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults
		CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error)
	}
)
