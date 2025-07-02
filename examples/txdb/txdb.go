package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/MrEhbr/pgxext/cluster"
	"github.com/MrEhbr/pgxext/txdb"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	// The first DSN is assumed to be the primary and all
	// other to be replica
	dsns := []string{
		"postgres://user:secret@primary:5432/mydb",
		"postgres://user:secret@replica-01:5432/mydb",
		"postgres://user:secret@replica-02:5432/mydb",
	}

	db, err := cluster.Open(dsns)
	if err != nil {
		logger.Error("failed to open cluster", "error", err)
		os.Exit(1)
	}

	txdb := txdb.New(db)

	ctx := context.Background()
	pingCtx, pingCancel := context.WithTimeout(ctx, time.Second)

	if pingErr := db.Ping(pingCtx); pingErr != nil {
		logger.Error("Some databases is unreachable", "error", pingErr)
		_ = txdb.Close()
		pingCancel()

		os.Exit(1)
	}

	_, err = db.Exec(ctx, `INSERT INTO foo(bar) VALUES("baz")`)
	if err != nil {
		logger.Error("failed to insert", "error", err)
		_ = txdb.Close()
		pingCancel()

		os.Exit(1)
	}
}
