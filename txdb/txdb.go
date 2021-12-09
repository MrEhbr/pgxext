package txdb

import (
	"context"
	"fmt"
	"sync"

	"github.com/MrEhbr/pgxext/cluster"
	"github.com/MrEhbr/pgxext/conn"
	"github.com/georgysavva/scany/pgxscan"
	"github.com/jackc/pgx/v4"
)

var (
	_ Conn         = &txdbCluster{}
	_ cluster.Conn = &txdbCluster{}
)

type (
	Conn interface {
		cluster.Conn
		Rollback(context.Context) error
	}
	txdbCluster struct {
		txLock sync.Mutex

		tx      pgx.Tx
		cluster cluster.Conn
		scanAPI *pgxscan.API
	}
)

func New(cluster *cluster.Cluster) *txdbCluster {
	return &txdbCluster{cluster: cluster, scanAPI: cluster.ScanAPI()}
}

// Close rollback current transaction and close physical connection
func (c *txdbCluster) Close() error {
	if err := c.Rollback(context.Background()); err != nil {
		return err
	}

	return c.cluster.Close()
}

func (c *txdbCluster) Rollback(ctx context.Context) error {
	c.txLock.Lock()
	defer c.txLock.Unlock()

	if c.tx != nil {
		if err := c.tx.Rollback(ctx); err != nil {
			return fmt.Errorf("failed to rollback transaction: %w", err)
		}

		c.tx = nil
	}

	return nil
}

func (c *txdbCluster) Ping(ctx context.Context) error {
	return c.cluster.Ping(ctx)
}

func (c *txdbCluster) Select(ctx context.Context, dst interface{}, sql string, args ...interface{}) error {
	c.txLock.Lock()
	defer c.txLock.Unlock()

	tx, err := c.beginOnce(ctx)
	if err != nil {
		return err
	}
	return conn.WrapConn(tx, c.scanAPI).Select(ctx, dst, sql, args...)
}

func (c *txdbCluster) Get(ctx context.Context, dst interface{}, sql string, args ...interface{}) error {
	c.txLock.Lock()
	defer c.txLock.Unlock()

	tx, err := c.beginOnce(ctx)
	if err != nil {
		return err
	}

	return conn.WrapConn(tx, c.scanAPI).Get(ctx, dst, sql, args...)
}

func (c *txdbCluster) Tx(ctx context.Context, f func(n conn.Querier) error, opts ...conn.TxOption) error {
	c.txLock.Lock()
	defer c.txLock.Unlock()

	tx, err := c.beginOnce(ctx)
	if err != nil {
		return err
	}

	return conn.WrapConn(tx, c.scanAPI).Tx(ctx, f, opts...)
}

func (c *txdbCluster) Begin(ctx context.Context) (pgx.Tx, error) {
	c.txLock.Lock()
	defer c.txLock.Unlock()
	return c.beginOnce(ctx)
}

func (c *txdbCluster) Exec(ctx context.Context, sql string, args ...interface{}) (int64, error) {
	c.txLock.Lock()
	defer c.txLock.Unlock()

	tx, err := c.beginOnce(ctx)
	if err != nil {
		return 0, err
	}
	return conn.WrapConn(tx, c.scanAPI).Exec(ctx, sql, args...)
}

func (c *txdbCluster) Primary() conn.Querier {
	c.txLock.Lock()
	defer c.txLock.Unlock()

	tx, err := c.beginOnce(context.Background())
	if err != nil {
		panic(err)
	}

	return conn.WrapConn(tx, c.scanAPI)
}

func (c *txdbCluster) Replica() conn.Querier {
	return c.Primary()
}

func (c *txdbCluster) beginOnce(ctx context.Context) (pgx.Tx, error) {
	if c.tx == nil {
		tx, err := c.cluster.Primary().Conn().Begin(ctx)
		if err != nil {
			return nil, err
		}

		c.tx = tx
	}
	return c.tx, nil
}
