package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/MrEhbr/pgxext/conn"
	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	ctx := context.Background()
	db, err := pgxpool.New(ctx, "postgres://user:secret@host:5432/mydb")
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}

	pingCtx, pingCancel := context.WithTimeout(ctx, time.Second)

	if pingErr := db.Ping(pingCtx); pingErr != nil {
		logger.Error("Database is unreachable", "error", pingErr)
		pingCancel()
		os.Exit(1)
	}

	wrapped := conn.WrapConn(db, pgxscan.DefaultAPI)

	var count int
	err = wrapped.Get(ctx, &count, "SELECT COUNT(*) FROM table")
	if err != nil {
		logger.Error("failed to get", "error", err)
		os.Exit(1)
	}

	var rows []string
	err = wrapped.Select(ctx, &rows, "SELECT something FROM table")
	if err != nil {
		logger.Error("failed to select", "error", err)
		os.Exit(1)
	}

	_, err = wrapped.Exec(ctx, "UPDATE table SET something = 1")
	if err != nil {
		logger.Error("failed to update", "error", err)
		os.Exit(1)
	}

	// Transaction will be canceled if update took to long
	err = wrapped.Tx(ctx, func(q conn.Querier) error {
		_, err = q.Exec(ctx, "UPDATE table SET something = 1")
		return err
	}, conn.StatementTimeout(time.Second))

	tx, err := wrapped.Conn(ctx).Begin(ctx)
	if err != nil {
		logger.Error("failed to start transaction", "error", err)
		os.Exit(1)
	}

	// Put a transaction in the context, so that all subsequent calls use the transaction
	txCtx := conn.NewTxContext(ctx, tx)
	if _, execErr := wrapped.Exec(txCtx, "UPDATE table SET something = 1"); execErr != nil {
		_ = tx.Rollback(ctx)
		logger.Error("failed to exec", "error", execErr)
		os.Exit(1)
	}
	if getErr := wrapped.Get(txCtx, &count, "SELECT COUNT(*) FROM table"); getErr != nil {
		_ = tx.Rollback(ctx)
		logger.Error("failed to get", "error", getErr)
		os.Exit(1)
	}

	if commitErr := tx.Commit(ctx); commitErr != nil {
		logger.Error("failed to commit transaction", "error", commitErr)
		os.Exit(1)
	}
}
