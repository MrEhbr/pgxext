package conn

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/georgysavva/scany/pgxscan"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/matryer/is"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
)

func TestQuerier(t *testing.T) {
	t.Parallel()
	var (
		db          *pgxpool.Pool
		databaseDSN string
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
		if _, err = pool.Client.Info(); err != nil {
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

		databaseDSN = fmt.Sprintf("postgres://user_name:secret@%s/test?sslmode=disable", resource.GetHostPort("5432/tcp"))

		t.Logf("Connecting to database on url: %s", databaseDSN)

		resource.Expire(120) // Tell docker to hard kill the container in 120 seconds

		// exponential backoff-retry, because the application in the container might not be ready to accept connections yet
		pool.MaxWait = 120 * time.Second
		if err = pool.Retry(func() error {
			db, err = pgxpool.Connect(t.Context(), databaseDSN)
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

	tableSQL := `create table if not exists %s
(
    id        serial primary key,
    t_zone    timestamptz,
    t         timestamp,
    slice     TEXT[],
    json      json,
    jsonb     jsonb,
    json_text text,
    str       text,
    float     float8,
    int       int
);
  `

	type foobar struct {
		Foo string `db:"foo"`
		Bar string `db:"bar"`
	}
	type row struct {
		ID           int             `db:"id"`
		TimeWithZone time.Time       `db:"t_zone"`
		Time         time.Time       `db:"t"`
		JSON         foobar          `db:"json"`
		JSONB        foobar          `db:"jsonb"`
		JSONText     json.RawMessage `db:"json_text"`
		String       string          `db:"str"`
		Slice        []string        `db:"slice"`
		Float        float64         `db:"float"`
		Int          int             `db:"int"`
	}

	loc, locErr := time.LoadLocation("Europe/Moscow")
	if locErr != nil {
		t.Fatalf("failed to load time location: %s", locErr)
	}

	now := time.Now().Round(time.Second)
	testRow := row{
		TimeWithZone: now.In(loc),
		Time:         now.UTC(),
		JSON: foobar{
			Foo: "foo",
			Bar: "bar",
		},
		JSONB: foobar{
			Foo: "foo",
			Bar: "bar",
		},
		JSONText: []byte(`{"foo": "foo", "bar": "bar"}`),
		String:   "foo",
		Slice:    []string{"foo", "bar"},
		Float:    3.14,
		Int:      1,
	}

	t.Run("Get", func(t *testing.T) {
		t.Parallel()
		is := is.New(t)

		querier := WrapConn(db, pgxscan.DefaultAPI)
		tblName := "test_get"
		_, execErr := querier.Exec(t.Context(), fmt.Sprintf(tableSQL, tblName))
		is.NoErr(execErr) // table creation
		t.Cleanup(func() {
			_, err := querier.Exec(context.Background(), fmt.Sprintf(`DROP TABLE IF EXISTS %s`, tblName)) //nolint: usetesting
			is.NoErr(err)
		})

		_ = testRow
		insertSQL := fmt.Sprintf(`
    INSERT INTO %s (t_zone, t, slice, json, jsonb, json_text, str, float, int)
    VALUES (
      $1,
      $2,
      $3,
      $4,
      $5,
      $6,
      $7,
      $8,
      $9
     )
    `, tblName)
		n, execErr := querier.Exec(t.Context(), insertSQL,
			testRow.TimeWithZone,
			testRow.Time,
			testRow.Slice,
			testRow.JSON,
			testRow.JSONB,
			string(testRow.JSONText),
			testRow.String,
			testRow.Float,
			testRow.Int,
		)
		is.NoErr(execErr) // insert row
		is.True(n == 1)   // row inserted

		var got row
		is.NoErr(querier.Get(t.Context(), &got, fmt.Sprintf("SELECT * FROM %s LIMIT 1", tblName)))
		if diff := cmp.Diff(testRow, got, cmpopts.IgnoreFields(row{}, "ID")); diff != "" {
			t.Errorf("querier.Get() mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("Select", func(t *testing.T) {
		t.Parallel()
		is := is.New(t)

		querier := WrapConn(db, pgxscan.DefaultAPI)
		tblName := "test_select"
		_, err := querier.Exec(t.Context(), fmt.Sprintf(tableSQL, tblName))
		is.NoErr(err) // table creation
		t.Cleanup(func() {
			_, err = querier.Exec(context.Background(), fmt.Sprintf(`DROP TABLE IF EXISTS %s`, tblName)) //nolint: usetesting
			is.NoErr(err)
		})

		_ = testRow
		insertSQL := fmt.Sprintf(`
    INSERT INTO %s (t_zone, t, slice, json, jsonb, json_text, str, float, int)
    VALUES (
      $1,
      $2,
      $3,
      $4,
      $5,
      $6,
      $7,
      $8,
      $9
     )
    `, tblName)
		for range 5 {
			n, execErr := querier.Exec(t.Context(), insertSQL,
				testRow.TimeWithZone,
				testRow.Time,
				testRow.Slice,
				testRow.JSON,
				testRow.JSONB,
				string(testRow.JSONText),
				testRow.String,
				testRow.Float,
				testRow.Int,
			)
			is.NoErr(execErr) // insert row
			is.True(n == 1)   // row inserted
		}

		var got []row
		is.NoErr(querier.Select(t.Context(), &got, fmt.Sprintf("SELECT * FROM %s", tblName)))
		for i, v := range got {
			is.Equal(v.ID, i+1)
			if diff := cmp.Diff(testRow, v, cmpopts.IgnoreFields(row{}, "ID")); diff != "" {
				t.Errorf("querier.Select() mismatch (-want +got):\n%s", diff)
			}
		}
	})

	t.Run("Tx", func(t *testing.T) {
		t.Parallel()
		is := is.New(t)

		querier := WrapConn(db, pgxscan.DefaultAPI)

		t.Run("TransactionTimeout", func(t *testing.T) {
			t.Parallel()
			is := is.New(t)

			err := querier.Tx(t.Context(), func(_ Querier) error {
				time.Sleep(100 * time.Millisecond)
				return nil
			}, TransactionTimeout(50*time.Millisecond))

			is.True(err != nil) // timeout idle error
			var pgErr *pgconn.PgError
			is.True(errors.As(err, &pgErr)) // must be pg error
			is.True(pgErr.Code == "25P03")  // 25P03 - idle_in_transaction_session_timeout error code
		})

		t.Run("StatementTimeout", func(t *testing.T) {
			t.Parallel()
			is := is.New(t)

			start := time.Now()
			err := querier.Tx(t.Context(), func(q Querier) error {
				_, execErr := q.Exec(t.Context(), "SELECT pg_sleep(1)")
				return execErr
			}, StatementTimeout(50*time.Millisecond))

			is.True(time.Since(start) < 100*time.Millisecond)
			is.True(err != nil) // timeout idle error
			var pgErr *pgconn.PgError
			is.True(errors.As(err, &pgErr)) // must be pg error
			is.True(pgErr.Code == "57014")  // 57014 - query_canceled error code
		})
	})

	t.Run("Conn", func(t *testing.T) {
		t.Parallel()
		querier := WrapConn(db, pgxscan.DefaultAPI)

		t.Run("no transaction in ctx", func(t *testing.T) {
			t.Parallel()
			is := is.New(t)
			ctx := t.Context()

			conn := querier.Conn(ctx)
			is.True(conn == db) // must be original database connection
		})

		t.Run("transaction in ctx", func(t *testing.T) {
			t.Parallel()
			is := is.New(t)
			tx, txErr := querier.Conn(t.Context()).Begin(t.Context())
			is.NoErr(txErr)
			ctx := NewTxContext(t.Context(), tx)
			conn := querier.Conn(ctx)
			is.True(conn == tx) // must be transaction connection
		})
	})
}
