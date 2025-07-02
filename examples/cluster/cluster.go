package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/MrEhbr/pgxext/cluster"
)

func main() {
	// The first DSN is assumed to be the primary and all
	// other to be replica
	dsns := []string{
		"postgres://user:secret@primary:5432/mydb",
		"postgres://user:secret@replica-01:5432/mydb",
		"postgres://user:secret@replica-02:5432/mydb",
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	db, err := cluster.Open(dsns)
	if err != nil {
		logger.Error("failed to open cluster", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()

	pingCtx, pingCancel := context.WithTimeout(ctx, time.Second)

	if pingErr := db.Ping(pingCtx); pingErr != nil {
		logger.Error("Some databases is unreachable", "error", pingErr)
		pingCancel()
		os.Exit(1)
	}

	// Read queries are directed to replica with Get and Select.
	// Always use Get or Select for SELECTS
	// Load distribution is round-robin.
	var count int
	err = db.Get(ctx, &count, "SELECT COUNT(*) FROM table")
	if err != nil {
		logger.Error("failed to get", "error", err)
		os.Exit(1)
	}

	// Write queries are directed to the primary with Exec.
	// Always use Exec for INSERTS, UPDATES
	_, err = db.Exec(ctx, "UPDATE table SET something = 1")
	if err != nil {
		logger.Error("failed to update", "error", err)
		os.Exit(1)
	}

	// If needed, one can access the PgxConn to call pgx methods directly such as SendBatch, CopyFrom ... .
	conn := db.Primary().Conn(ctx)
	_ = conn
	// If needed, one can access the primary or a replica explicitly.
	primary, replica := db.Primary(), db.Replica()
	_, _ = primary, replica
}
