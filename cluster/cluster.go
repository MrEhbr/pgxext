package cluster

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/MrEhbr/pgxext/conn"
	"github.com/georgysavva/scany/pgxscan"
	"github.com/jackc/pgx/v4/pgxpool"
)

type (
	// Conn is a logical database with multiple underlying physical databases
	// forming a single primary multiple replica topology.
	// Reads(Get, Select) and writes(Exec, Tx) are automatically directed to the correct physical db.
	Conn interface {
		Select(ctx context.Context, dst interface{}, sql string, args ...interface{}) error
		Get(ctx context.Context, dst interface{}, sql string, args ...interface{}) error
		Exec(ctx context.Context, sql string, args ...interface{}) (int64, error)
		Tx(ctx context.Context, f func(n conn.Querier) error, opts ...conn.TxOption) error
		Primary() conn.Querier
		Replica() conn.Querier
		Ping(context.Context) error
		Close() error
	}

	ConnPicker func(db Conn, sql string) conn.Querier
)

var _ Conn = &cluster{}

type pdb struct {
	pool    *pgxpool.Pool
	querier conn.Querier
}

type cluster struct {
	picker  ConnPicker
	scanAPI *pgxscan.API
	pdbs    []*pdb
	count   uint64
}

// Open concurrently opens each underlying physical db.
// DSN must be valid according to https://www.postgresql.org/docs/current/libpq-connect.html#LIBPQ-CONNSTRING
// first being used as the primary and the rest as replica.
func Open(dsn []string, opts ...Option) (*cluster, error) {
	configs := make([]*pgxpool.Config, len(dsn))
	for i := range dsn {
		var err error
		configs[i], err = pgxpool.ParseConfig(dsn[i])
		if err != nil {
			return nil, fmt.Errorf("failed to parse config at index %d: %w", i+1, err)
		}
	}

	return NewFromConfigs(configs, opts...)
}

// NewFromConfigs concurrently opens each underlying physical db.
// first being used as the primary and the rest as replica.
func NewFromConfigs(config []*pgxpool.Config, opts ...Option) (*cluster, error) {
	clusterOpts := &Options{
		Picker:  defaultConnPicker,
		ScanAPI: pgxscan.DefaultAPI,
	}

	for _, o := range opts {
		o(clusterOpts)
	}

	db := &cluster{
		pdbs:    make([]*pdb, len(config)),
		picker:  clusterOpts.Picker,
		scanAPI: clusterOpts.ScanAPI,
	}

	err := scatter(len(db.pdbs), func(i int) error {
		c, err := pgxpool.ConnectConfig(context.Background(), config[i])
		if err != nil {
			return err
		}

		db.pdbs[i] = &pdb{
			pool:    c,
			querier: conn.WrapConn(c, db.ScanAPI()),
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return db, nil
}

// Close closes all physical databases concurrently, releasing any open resources.
func (conn *cluster) Close() error {
	return scatter(len(conn.pdbs), func(i int) error {
		conn.pdbs[i].pool.Close()
		return nil
	})
}

// Ping verifies if a connection to each physical database is still alive,
// establishing a connection if necessary.
func (conn *cluster) Ping(ctx context.Context) error {
	return scatter(len(conn.pdbs), func(i int) error {
		return conn.pdbs[i].pool.Ping(ctx)
	})
}

// Select multiple records.
// Select uses a replica by default.
// See Querier.Select for details.
func (conn *cluster) Select(ctx context.Context, dst interface{}, sql string, args ...interface{}) error {
	return conn.picker(conn, sql).Select(ctx, dst, sql, args...)
}

// Get retriave one row.
// Get uses a replica by default.
// See Querier.Get for details.
func (conn *cluster) Get(ctx context.Context, dst interface{}, sql string, args ...interface{}) error {
	return conn.picker(conn, sql).Get(ctx, dst, sql, args...)
}

// Exec executes a query on primary without returning any rows and return affected rows.
func (conn *cluster) Exec(ctx context.Context, sql string, args ...interface{}) (int64, error) {
	return conn.Primary().Exec(ctx, sql, args...)
}

// Tx starts a transaction on primary and calls f.
// See Querier.Tx for details.
func (conn *cluster) Tx(ctx context.Context, f func(n conn.Querier) error, opts ...conn.TxOption) error {
	return conn.Primary().Tx(ctx, f, opts...)
}

// Primary returns the primary physical database
func (conn *cluster) Primary() conn.Querier {
	return conn.pdbs[0].querier
}

// Replica returns one of the replica databases.
func (conn *cluster) Replica() conn.Querier {
	return conn.pdbs[conn.replica(len(conn.pdbs))].querier
}

func (conn *cluster) replica(n int) int {
	if n <= 1 {
		return 0
	}
	return int(1 + (atomic.AddUint64(&conn.count, 1) % uint64(n-1)))
}

func (conn *cluster) ScanAPI() *pgxscan.API {
	return conn.scanAPI
}
