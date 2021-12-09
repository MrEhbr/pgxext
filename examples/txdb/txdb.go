package txdb

import (
	"context"
	"log"
	"time"

	"github.com/MrEhbr/pgxext/cluster"
	"github.com/MrEhbr/pgxext/txdb"
)

func main() {
	// The first DSN is assumed to be the primary and all
	// other to be replica
	dsns := []string{
		"postgres://user:secret@primary:5432/mydb",
		"postgres://user:secret@replica-01:5432/mydb",
		"postgres://user:secret@replica-02:5432/mydb",
	}

	db, err := cluster.Open(dsns)
	if err != nil {
		log.Fatal(err)
	}

	txdb := txdb.New(db)
	defer txdb.Close()

	ctx := context.Background()
	pingCtx, pingCancel := context.WithTimeout(ctx, time.Second)
	defer pingCancel()

	if err := db.Ping(pingCtx); err != nil {
		log.Fatalf("Some databases is unreachable: %s", err)
	}

	_, err = db.Exec(ctx, `INSERT INTO foo(bar) VALUES("baz")`)
	if err != nil {
		log.Fatalf("failed to insert: %s", err)
	}
}
