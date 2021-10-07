package txdb

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/MrEhbr/pgxext/cluster"
	"github.com/MrEhbr/pgxext/conn"
	"github.com/georgysavva/scany/pgxscan"
	"github.com/matryer/is"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
)

func TestIntegration_txdb(t *testing.T) {
	if strings.Contains(os.Getenv("OS"), "macos") {
		t.Skip("not supported on mac os runner")
	}

	var (
		db          cluster.Conn
		databaseUrl string
	)
	{
		// uses a sensible default on windows (tcp/http) and linux/osx (socket)
		pool, err := dockertest.NewPool("")
		if err != nil {
			if errors.Is(err, docker.ErrInvalidEndpoint) {
				t.Skip("docker endpoint not found")
			}

			t.Fatalf("Could not connect to docker: %s", err)
		}
		if _, err := pool.Client.Info(); err != nil {
			if errors.Is(err, docker.ErrConnectionRefused) {
				t.Skip("docker not running")
			}
		}

		// pulls an image, creates a container based on it and runs it
		resource, err := pool.RunWithOptions(&dockertest.RunOptions{
			Repository: "postgres",
			Tag:        "11",
			Env: []string{
				"POSTGRES_PASSWORD=secret",
				"POSTGRES_USER=user_name",
				"POSTGRES_DB=test",
				"listen_addresses = '*'",
			},
		}, func(config *docker.HostConfig) {
			// set AutoRemove to true so that stopped container goes away by itself
			config.AutoRemove = true
			config.RestartPolicy = docker.RestartPolicy{Name: "no"}
		})
		if err != nil {
			t.Fatalf("Could not start resource: %s", err)
		}

		databaseUrl = fmt.Sprintf("postgres://user_name:secret@%s/test?sslmode=disable", resource.GetHostPort("5432/tcp"))

		t.Logf("Connecting to database on url: %s", databaseUrl)

		resource.Expire(120) // Tell docker to hard kill the container in 120 seconds

		// exponential backoff-retry, because the application in the container might not be ready to accept connections yet
		pool.MaxWait = 120 * time.Second
		if err = pool.Retry(func() error {
			db, err = cluster.Open([]string{databaseUrl})
			if err != nil {
				return err
			}
			return db.Ping(context.Background())
		}); err != nil {
			t.Fatalf("Could not connect to postgres: %s", err)
		}

		t.Cleanup(func() {
			if err := pool.Purge(resource); err != nil {
				t.Fatalf("Could not purge resource: %s", err)
			}
		})
	}
	t.Run("New", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		is := is.New(t)

		sendBatch := func(conn conn.PgxConn) (time.Time, time.Time) {
			var t1, t2 time.Time
			is.NoErr(conn.QueryRow(ctx, "SELECT transaction_timestamp()").Scan(&t1))
			time.Sleep(100 * time.Millisecond)
			is.NoErr(conn.QueryRow(ctx, "SELECT transaction_timestamp()").Scan(&t2))

			return t1, t2
		}

		t.Run("check that transaction opens", func(t *testing.T) {
			t.Parallel()
			is := is.New(t)

			t1, t2 := sendBatch(db.Primary().Conn())
			is.True(!t1.Equal(t2)) // transaction_timestamp not in transaction must be not equal

			txdb := New(db, pgxscan.DefaultAPI)

			t1, t2 = sendBatch(txdb.Primary().Conn())
			is.True(t1.Equal(t2)) // transaction_timestamp in transaction must be equal
		})
	})

	t.Run("Rollback", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		is := is.New(t)

		_, err := db.Exec(ctx, `CREATE TABLE IF NOT EXISTS test(name TEXT)`)
		is.NoErr(err)

		t.Run("check that all changes to table rolled back", func(t *testing.T) {
			t.Parallel()
			is := is.New(t)

			txdb := New(db, pgxscan.DefaultAPI)
			t.Cleanup(func() {
				txdb.Rollback(context.Background())
			})

			name := "foo"
			_, err = txdb.Exec(ctx, `INSERT INTO test(name) VALUES($1)`, name)
			is.NoErr(err)
			var exists bool
			is.NoErr(txdb.Get(ctx, &exists, `SELECT COUNT(1) > 0 FROM test WHERE name = $1`, name))
			is.True(exists)
			is.NoErr(txdb.Rollback(ctx))

			is.NoErr(db.Get(ctx, &exists, `SELECT COUNT(1) > 0 FROM test WHERE name = $1`, name))
			is.True(!exists)
		})
	})

	t.Run("Close", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		is := is.New(t)

		_, err := db.Exec(ctx, `CREATE TABLE IF NOT EXISTS test_close(name TEXT)`)
		is.NoErr(err)

		t.Run("check that close rolledback changes", func(t *testing.T) {
			t.Parallel()
			is := is.New(t)

			tdb, err := cluster.Open([]string{databaseUrl})
			is.NoErr(err)
			txdb := New(tdb, pgxscan.DefaultAPI)

			t.Cleanup(func() {
				txdb.Rollback(context.Background())
			})

			name := "foo"
			_, err = txdb.Exec(ctx, `INSERT INTO test_close(name) VALUES($1)`, name)
			is.NoErr(err)
			var exists bool
			is.NoErr(txdb.Get(ctx, &exists, `SELECT COUNT(1) > 0 FROM test_close WHERE name = $1`, name))
			is.True(exists)
			is.NoErr(txdb.Close())

			tdb, err = cluster.Open([]string{databaseUrl})
			is.NoErr(err)

			defer tdb.Close()

			is.NoErr(tdb.Get(ctx, &exists, `SELECT COUNT(1) > 0 FROM test_close WHERE name = $1`, name))
			is.True(!exists)
		})
	})

	t.Run("check nested transactions", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		is := is.New(t)

		txdb := New(db, pgxscan.DefaultAPI)

		_, err := db.Exec(ctx, `CREATE TABLE IF NOT EXISTS test_nested(name TEXT)`)
		is.NoErr(err)

		foo, bar := "foo", "bar"
		_, err = txdb.Exec(ctx, `INSERT INTO test_nested(name) VALUES($1)`, foo)
		is.NoErr(err)
		var exists bool
		is.NoErr(txdb.Get(ctx, &exists, `SELECT COUNT(1) > 0 FROM test_nested WHERE name = $1`, foo))
		is.True(exists)

		txdb.Tx(ctx, func(q conn.Querier) error {
			_, err := q.Exec(ctx, `INSERT INTO test_nested(name) VALUES($1)`, bar)
			is.NoErr(err)
			var exists bool
			is.NoErr(q.Get(ctx, &exists, `SELECT COUNT(1) > 0 FROM test_nested WHERE name = $1`, bar))
			is.True(exists)
			return errors.New("rollback")
		})

		is.NoErr(txdb.Get(ctx, &exists, `SELECT COUNT(1) > 0 FROM test_nested WHERE name = $1`, bar))
		is.True(!exists)

		is.NoErr(txdb.Get(ctx, &exists, `SELECT COUNT(1) > 0 FROM test_nested WHERE name = $1`, foo))
		is.True(exists)
	})
}
