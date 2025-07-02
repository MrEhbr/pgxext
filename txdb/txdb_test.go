package txdb

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/MrEhbr/pgxext/cluster"
	"github.com/MrEhbr/pgxext/conn"
	"github.com/matryer/is"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
)

func TestIntegration_txdb(t *testing.T) {
	t.Parallel()
	var (
		db          *cluster.Cluster
		databaseDSN string
	)
	{
		// uses a sensible default on windows (tcp/http) and linux/osx (socket)
		pool, poolErr := dockertest.NewPool("")
		if poolErr != nil {
			if errors.Is(poolErr, docker.ErrInvalidEndpoint) {
				t.Skip("docker endpoint not found")
			}

			t.Fatalf("Could not connect to docker: %s", poolErr)
		}
		if _, err := pool.Client.Info(); err != nil {
			if errors.Is(err, docker.ErrConnectionRefused) {
				t.Skip("docker not running")
			}
		}

		// pulls an image, creates a container based on it and runs it
		resource, err := pool.RunWithOptions(&dockertest.RunOptions{
			Repository: "postgres",
			Tag:        "17-alpine",
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

		databaseDSN = fmt.Sprintf("postgres://user_name:secret@%s/test?sslmode=disable", resource.GetHostPort("5432/tcp"))

		t.Logf("Connecting to database on url: %s", databaseDSN)

		resource.Expire(120) // Tell docker to hard kill the container in 120 seconds

		// exponential backoff-retry, because the application in the container might not be ready to accept connections yet
		pool.MaxWait = 120 * time.Second
		if err = pool.Retry(func() error {
			db, err = cluster.Open([]string{databaseDSN})
			if err != nil {
				return err
			}
			return db.Ping(t.Context())
		}); err != nil {
			t.Fatalf("Could not connect to postgres: %s", err)
		}

		t.Cleanup(func() {
			if err = pool.Purge(resource); err != nil {
				t.Fatalf("Could not purge resource: %s", err)
			}
		})
	}
	t.Run("New", func(t *testing.T) {
		t.Parallel()
		is := is.New(t)

		sendBatch := func(conn conn.PgxConn) (time.Time, time.Time) {
			var t1, t2 time.Time
			is.NoErr(conn.QueryRow(t.Context(), "SELECT transaction_timestamp()").Scan(&t1))
			time.Sleep(100 * time.Millisecond)
			is.NoErr(conn.QueryRow(t.Context(), "SELECT transaction_timestamp()").Scan(&t2))

			return t1, t2
		}

		t.Run("check that transaction opens", func(t *testing.T) {
			t.Parallel()
			is := is.New(t)

			t1, t2 := sendBatch(db.Primary().Conn(t.Context()))
			is.True(!t1.Equal(t2)) // transaction_timestamp not in transaction must be not equal

			txdb := New(db)

			t1, t2 = sendBatch(txdb.Primary().Conn(t.Context()))
			is.True(t1.Equal(t2)) // transaction_timestamp in transaction must be equal
		})
	})

	t.Run("Rollback", func(t *testing.T) {
		t.Parallel()
		is := is.New(t)

		_, err := db.Exec(t.Context(), `CREATE TABLE IF NOT EXISTS test(name TEXT)`)
		is.NoErr(err)

		t.Run("check that all changes to table rolled back", func(t *testing.T) {
			t.Parallel()
			is := is.New(t)

			txdb := New(db)
			t.Cleanup(func() {
				txdb.Rollback(t.Context())
			})

			name := "foo"
			_, err = txdb.Exec(t.Context(), `INSERT INTO test(name) VALUES($1)`, name)
			is.NoErr(err)
			var exists bool
			is.NoErr(txdb.Get(t.Context(), &exists, `SELECT COUNT(1) > 0 FROM test WHERE name = $1`, name))
			is.True(exists)
			is.NoErr(txdb.Rollback(t.Context()))

			is.NoErr(db.Get(t.Context(), &exists, `SELECT COUNT(1) > 0 FROM test WHERE name = $1`, name))
			is.True(!exists)
		})
	})

	t.Run("Close", func(t *testing.T) {
		t.Parallel()
		is := is.New(t)

		_, err := db.Exec(t.Context(), `CREATE TABLE IF NOT EXISTS test_close(name TEXT)`)
		is.NoErr(err)

		t.Run("check that close rolledback changes", func(t *testing.T) {
			t.Parallel()
			is := is.New(t)

			tdb, openErr := cluster.Open([]string{databaseDSN})
			is.NoErr(openErr)
			txdb := New(tdb)

			t.Cleanup(func() {
				txdb.Rollback(t.Context())
			})

			name := "foo"
			_, err = txdb.Exec(t.Context(), `INSERT INTO test_close(name) VALUES($1)`, name)
			is.NoErr(err)
			var exists bool
			is.NoErr(txdb.Get(t.Context(), &exists, `SELECT COUNT(1) > 0 FROM test_close WHERE name = $1`, name))
			is.True(exists)
			is.NoErr(txdb.Close())

			tdb, err = cluster.Open([]string{databaseDSN})
			is.NoErr(err)

			defer tdb.Close()

			is.NoErr(tdb.Get(t.Context(), &exists, `SELECT COUNT(1) > 0 FROM test_close WHERE name = $1`, name))
			is.True(!exists)
		})
	})

	t.Run("check nested transactions", func(t *testing.T) {
		t.Parallel()
		is := is.New(t)

		txdb := New(db)

		_, err := db.Exec(t.Context(), `CREATE TABLE IF NOT EXISTS test_nested(name TEXT)`)
		is.NoErr(err)

		foo, bar := "foo", "bar"
		_, err = txdb.Exec(t.Context(), `INSERT INTO test_nested(name) VALUES($1)`, foo)
		is.NoErr(err)
		var existsFooFirst bool
		is.NoErr(txdb.Get(t.Context(), &existsFooFirst, `SELECT COUNT(1) > 0 FROM test_nested WHERE name = $1`, foo))
		is.True(existsFooFirst)

		txdb.Tx(t.Context(), func(q conn.Querier) error {
			_, err = q.Exec(t.Context(), `INSERT INTO test_nested(name) VALUES($1)`, bar)
			is.NoErr(err)
			var exists bool
			is.NoErr(q.Get(t.Context(), &exists, `SELECT COUNT(1) > 0 FROM test_nested WHERE name = $1`, bar))
			is.True(exists)
			return errors.New("rollback")
		})

		var existsBar bool
		is.NoErr(txdb.Get(t.Context(), &existsBar, `SELECT COUNT(1) > 0 FROM test_nested WHERE name = $1`, bar))
		is.True(!existsBar)

		var existsFoo bool
		is.NoErr(txdb.Get(t.Context(), &existsFoo, `SELECT COUNT(1) > 0 FROM test_nested WHERE name = $1`, foo))
		is.True(existsFoo)
	})
}
