package main

import (
	"context"
	"log"
	"time"

	"github.com/MrEhbr/pgxext/conn"
	"github.com/georgysavva/scany/pgxscan"
	"github.com/jackc/pgx/v4/pgxpool"
)

func main() {
	ctx := context.Background()
	db, err := pgxpool.Connect(ctx, "postgres://user:secret@host:5432/mydb")
	if err != nil {
		log.Fatal(err)
	}

	pingCtx, pingCancel := context.WithTimeout(ctx, time.Second)
	defer pingCancel()

	if err := db.Ping(pingCtx); err != nil {
		log.Fatalf("Database is unreachable: %s", err)
	}

	wrapped := conn.WrapConn(db, pgxscan.DefaultAPI)

	var count int
	err = wrapped.Get(ctx, &count, "SELECT COUNT(*) FROM table")
	if err != nil {
		log.Fatalf("failed to get: %s", err)
	}

	var rows []string
	err = wrapped.Select(ctx, &rows, "SELECT something FROM table")
	if err != nil {
		log.Fatalf("failed to select: %s", err)
	}

	_, err = wrapped.Exec(ctx, "UPDATE table SET something = 1")
	if err != nil {
		log.Fatalf("failed to update: %s", err)
	}

	// Transaction will be canceled if update took to long
	err = wrapped.Tx(ctx, func(q conn.Querier) error {
		_, err = q.Exec(ctx, "UPDATE table SET something = 1")
		return err
	}, conn.StatementTimeout(time.Second))

	tx, err := wrapped.Conn(ctx).Begin(ctx)
	if err != nil {
		log.Fatalf("failed to start transaction: %s", err)
	}

	// Put a transaction in the context, so that all subsequent calls use the transaction
	txCtx := conn.NewTxContext(ctx, tx)
	if _, err := wrapped.Exec(txCtx, "UPDATE table SET something = 1"); err != nil {
		_ = tx.Rollback(ctx)
		log.Fatalf("failed to exec: %s", err)
	}
	if err := wrapped.Get(txCtx, &count, "SELECT COUNT(*) FROM table"); err != nil {
		_ = tx.Rollback(ctx)
		log.Fatalf("failed to get: %s", err)
	}

	if err := tx.Commit(ctx); err != nil {
		log.Fatalf("failed to commit transaction: %s", err)
	}
}
