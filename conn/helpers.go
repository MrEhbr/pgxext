package conn

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxtest"
)

func TestRunner() *pgxtest.ConnTestRunner {
	return &pgxtest.ConnTestRunner{
		CreateConfig: func(_ context.Context, t testing.TB) *pgx.ConnConfig {
			databaseDSN := os.Getenv("PGXEXT_TEST_DATABASE_DSN")
			if databaseDSN == "" {
				t.Skipf("PGXEXT_TEST_DATABASE_DSN environment variable is not set")
			}

			config, err := pgx.ParseConfig(databaseDSN)
			if err != nil {
				t.Fatalf("ParseConfig failed: %v", err)
			}
			return config
		},
		AfterConnect: func(_ context.Context, _ testing.TB, _ *pgx.Conn) {},
		AfterTest:    func(_ context.Context, _ testing.TB, _ *pgx.Conn) {},
		CloseConn: func(ctx context.Context, t testing.TB, conn *pgx.Conn) {
			err := conn.Close(ctx)
			if err != nil {
				t.Errorf("Close failed: %v", err)
			}
		},
	}
}
