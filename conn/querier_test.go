package conn

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/google/go-cmp/cmp"
	"github.com/jackc/pgx/v5"
	"github.com/matryer/is"
)

func TestQuerier(t *testing.T) {
	t.Run("Select", func(t *testing.T) {
		TestRunner().RunTest(t.Context(), t, func(ctx context.Context, t testing.TB, conn *pgx.Conn) {
			its := is.New(t)
			querier := WrapConn(conn, pgxscan.DefaultAPI)

			rows := []string{}
			err := querier.Select(ctx, &rows, `SELECT * FROM (VALUES ('one'), ('two')) AS t(value)`)
			its.NoErr(err)
			its.Equal(len(rows), 2)
			its.Equal(rows, []string{"one", "two"})
		})
	})

	t.Run("Get", func(t *testing.T) {
		TestRunner().RunTest(t.Context(), t, func(ctx context.Context, t testing.TB, conn *pgx.Conn) {
			its := is.New(t)
			querier := WrapConn(conn, pgxscan.DefaultAPI)

			var value string
			err := querier.Get(ctx, &value, `SELECT 'single_value'`)
			its.NoErr(err)
			its.Equal(value, "single_value")
		})
	})

	t.Run("Exec", func(t *testing.T) {
		TestRunner().RunTest(t.Context(), t, func(ctx context.Context, t testing.TB, conn *pgx.Conn) {
			its := is.New(t)
			querier := WrapConn(conn, pgxscan.DefaultAPI)

			_, err := querier.Exec(ctx, `CREATE TEMP TABLE test_table (id SERIAL PRIMARY KEY, value TEXT)`)
			its.NoErr(err)

			result, err := querier.Exec(ctx, `INSERT INTO test_table (value) VALUES ('test1'), ('test2')`)
			its.NoErr(err)
			its.Equal(result, int64(2))
		})
	})

	t.Run("Tx", func(t *testing.T) {
		t.Run("Commit", func(t *testing.T) {
			TestRunner().RunTest(t.Context(), t, func(ctx context.Context, t testing.TB, conn *pgx.Conn) {
				its := is.New(t)
				querier := WrapConn(conn, pgxscan.DefaultAPI)
				err := querier.Tx(ctx, func(q Querier) error {
					_, err := q.Exec(ctx, `CREATE TEMP TABLE test_tx_table (value TEXT) ON COMMIT DROP`)
					its.NoErr(err)

					return err
				})
				its.NoErr(err)

				var exists bool
				err = querier.Get(ctx, &exists, `SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'test_tx_table')`)
				its.NoErr(err)
				its.True(!exists)
			})
		})
		t.Run("Rollback", func(t *testing.T) {
			TestRunner().RunTest(t.Context(), t, func(ctx context.Context, t testing.TB, conn *pgx.Conn) {
				its := is.New(t)
				querier := WrapConn(conn, pgxscan.DefaultAPI)
				err := querier.Tx(ctx, func(q Querier) error {
					_, err := q.Exec(ctx, `CREATE TEMP TABLE test_tx_table (value TEXT)`)
					its.NoErr(err)

					return errors.New("force rollback")
				})
				its.True(err != nil)

				var exists bool
				err = querier.Get(ctx, &exists, `SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'test_tx_table')`)
				its.NoErr(err)
				its.True(!exists)
			})
		})
	})

	t.Run("Conn", func(t *testing.T) {
		t.Run("conn from struct", func(t *testing.T) {
			TestRunner().RunTest(t.Context(), t, func(ctx context.Context, t testing.TB, conn *pgx.Conn) {
				its := is.New(t)
				querier := WrapConn(conn, pgxscan.DefaultAPI)

				its.Equal(conn, querier.Conn(ctx))
			})
		})

		t.Run("conn from context", func(t *testing.T) {
			TestRunner().RunTest(t.Context(), t, func(ctx context.Context, t testing.TB, conn *pgx.Conn) {
				its := is.New(t)
				querier := WrapConn(conn, pgxscan.DefaultAPI)
				tx, err := conn.Begin(ctx)
				its.NoErr(err)
				t.Cleanup(func() {
					_ = tx.Rollback(ctx)
				})

				txCtx := NewTxContext(ctx, tx)

				its.Equal(tx, querier.Conn(txCtx))
			})
		})
	})

	t.Run("types scan", func(t *testing.T) {
		tests := []struct {
			pgType   string
			param    any
			result   any
			expected any
		}{
			{"text", "test string", "", "test string"},
			{"int4", int32(42), int32(0), int32(42)},
			{"int8", int64(1234567890), int64(0), int64(1234567890)},
			{"bigint", int64(1234567890), int64(0), int64(1234567890)},
			{"float4", float32(3.14), float32(0), float32(3.14)},
			{"float8", 3.14, float64(0), float64(3.14)},
			{"bool", true, false, true},
			{"date", time.Date(2023, 10, 1, 0, 0, 0, 0, time.UTC), time.Time{}, time.Date(2023, 10, 1, 0, 0, 0, 0, time.UTC)},
			{"timestamp", time.Date(2023, 10, 1, 12, 0, 0, 0, time.UTC), time.Time{}, time.Date(2023, 10, 1, 12, 0, 0, 0, time.UTC)},
			{"timestamptz", time.Date(2023, 10, 1, 12, 0, 0, 0, time.FixedZone("", 3*60*60)), time.Time{}, time.Date(2023, 10, 1, 12, 0, 0, 0, time.FixedZone("", 3*60*60))},
			{"jsonb", map[string]any{"key": "value"}, map[string]any{}, map[string]any{"key": "value"}},
			{"json", map[string]any{"key": "value"}, map[string]any{}, map[string]any{"key": "value"}},
			{"bytea", []byte{0x01, 0x02, 0x03}, []byte{}, []byte{0x01, 0x02, 0x03}},
			{"uuid::text", "550e8400-e29b-41d4-a716-446655440000", "", "550e8400-e29b-41d4-a716-446655440000"},
		}

		TestRunner().RunTest(t.Context(), t, func(ctx context.Context, t testing.TB, conn *pgx.Conn) {
			its := is.New(t)
			querier := WrapConn(conn, pgxscan.DefaultAPI)

			for _, tt := range tests {
				sql := fmt.Sprintf(`SELECT $1::%s`, tt.pgType)

				result := tt.result
				err := querier.Get(ctx, &result, sql, tt.param)
				its.NoErr(err)

				if diff := cmp.Diff(tt.expected, result); diff != "" {
					t.Errorf("%s: mismatch (-want +got):\n%s", tt.pgType, diff)
				}
			}
		})
	})

	t.Run("struct scan", func(t *testing.T) {
		type NestedStruct struct {
			X int `db:"x"`
		}

		type TestStruct struct {
			Int64Field   int64          `db:"int64_field"`
			StringField  string         `db:"string_field"`
			BoolField    bool           `db:"bool_field"`
			TimeField    time.Time      `db:"time_field"`
			TimeTZField  time.Time      `db:"timetz_field"`
			StringsField []string       `db:"strings_field"`
			JSONField    map[string]any `db:"json_field"`
			JSONBField   map[string]any `db:"jsonb_field"`
			StructField  NestedStruct   `db:"struct_field"`
		}

		TestRunner().RunTest(t.Context(), t, func(ctx context.Context, t testing.TB, conn *pgx.Conn) {
			its := is.New(t)
			querier := WrapConn(conn, pgxscan.DefaultAPI)

			const sql = `SELECT
				123::bigint as int64_field,
				'test'::text as string_field,
				true::bool as bool_field,
				'2023-10-01T12:00:00Z'::timestamp as time_field,
				'2023-10-01T12:00:00+03:00'::timestamptz as timetz_field,
				ARRAY['a', 'b']::text[] as strings_field,
				'{"x": 1}'::jsonb as json_field,
        '{"x": 2}'::jsonb as jsonb_field,
        '{"x": 3}'::jsonb as struct_field
      `

			expected := TestStruct{
				Int64Field:   123,
				StringField:  "test",
				BoolField:    true,
				TimeField:    time.Date(2023, 10, 1, 12, 0, 0, 0, time.UTC),
				TimeTZField:  time.Date(2023, 10, 1, 12, 0, 0, 0, time.FixedZone("", 3*60*60)),
				StringsField: []string{"a", "b"},
				JSONField:    map[string]any{"x": float64(1)},
				JSONBField:   map[string]any{"x": float64(2)},
				StructField:  NestedStruct{X: 3},
			}

			var result TestStruct
			err := querier.Get(ctx, &result, sql)
			its.NoErr(err)

			if diff := cmp.Diff(expected, result); diff != "" {
				t.Errorf("struct scan mismatch (-want +got):\n%s", diff)
			}
		})
	})
}
