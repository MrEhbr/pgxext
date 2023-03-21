# pgxext

[![Go](https://github.com/MrEhbr/pgxext/actions/workflows/go.yml/badge.svg)](https://github.com/MrEhbr/pgxext/actions/workflows/go.yml)
[![License](https://img.shields.io/badge/license-Apache--2.0%20%2F%20MIT-%2397ca00.svg)](https://github.com/MrEhbr/pgxext/blob/master/COPYRIGHT)
[![codecov](https://codecov.io/gh/MrEhbr/pgxext/branch/master/graph/badge.svg)](https://codecov.io/gh/MrEhbr/pgxext)
![Made by Alexey Burmistrov](https://img.shields.io/badge/made%20by-Alexey%20Burmistrov-blue.svg?style=flat)

pgxext is a set of libraries for pgx

## Install

### Using go

```console
go get -u github.com/MrEhbr/pgxext/...
```

## Package cluster

`cluster` is a library that abstracts access to master-slave physical db servers topologies as a single logical database

### Usage

```golang
package main

import (
 "context"
 "log"
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

 db, err := cluster.Open(dsns)
 if err != nil {
  log.Fatal(err)
 }

 ctx := context.Background()

 pingCtx, pingCancel := context.WithTimeout(ctx, time.Second)
 defer pingCancel()

 if err := db.Ping(pingCtx); err != nil {
  log.Fatalf("Some databases is unreachable: %s", err)
 }

 // Read queries are directed to replica with Get and Select.
 // Always use Get or Select for SELECTS
 // Load distribution is round-robin.
 var count int
 err = db.Get(ctx, &count, "SELECT COUNT(*) FROM table")
 if err != nil {
  log.Fatalf("failed to get: %s", err)
 }

 // Write queries are directed to the primary with Exec.
 // Always use Exec for INSERTS, UPDATES
 _, err = db.Exec(ctx, "UPDATE table SET something = 1")
 if err != nil {
  log.Fatalf("failed to update: %s", err)
 }

 // If needed, one can access the PgxConn to call pgx methods directly such as SendBatch, CopyFrom ... .
 conn := db.Primary().Conn()

 // If needed, one can access the primary or a replica explicitly.
 primary, replica := db.Primary(), db.Replica()
}
```

## Package conn

`conn` is a library that simplify querying and scanning rows

### Usage

```golang
package main

import (
 "context"
 "log"
 "time"

 "github.com/MrEhbr/pgxext/conn"
 "github.com/georgysavva/scany/pgxscan"
 "github.com/jackc/pgx/v4/pgxpool"
)

func main() {
 ctx := context.Background()
 db, err := pgxpool.Connect(ctx, "postgres://user:secret@host:5432/mydb")
 if err != nil {
  log.Fatal(err)
 }

 pingCtx, pingCancel := context.WithTimeout(ctx, time.Second)
 defer pingCancel()

 if err := db.Ping(pingCtx); err != nil {
  log.Fatalf("Database is unreachable: %s", err)
 }

 wrapped := conn.WrapConn(db, pgxscan.DefaultAPI)

 var count int
 err = wrapped.Get(ctx, &count, "SELECT COUNT(*) FROM table")
 if err != nil {
  log.Fatalf("failed to get: %s", err)
 }

 var rows []string
 err = wrapped.Select(ctx, &rows, "SELECT something FROM table")
 if err != nil {
  log.Fatalf("failed to select: %s", err)
 }

 _, err = wrapped.Exec(ctx, "UPDATE table SET something = 1")
 if err != nil {
  log.Fatalf("failed to update: %s", err)
 }

 // Transaction will be canceled if update took to long
 err = wrapped.Tx(ctx, func(q conn.Querier) error {
  _, err = q.Exec(ctx, "UPDATE table SET something = 1")
  return err
 }, conn.StatementTimeout(time.Second))

tx, err := wrapped.Conn(ctx).Begin(ctx)
 if err != nil {
  log.Fatalf("failed to start transaction: %s", err)
 }

 // Put a transaction in the context, so that all subsequent calls use the transaction
 txCtx := conn.NewTxContext(ctx, tx)
 if _, err := wrapped.Exec(txCtx, "UPDATE table SET something = 1"); err != nil {
  _ = tx.Rollback(ctx)
  log.Fatalf("failed to exec: %s", err)
 }
 if err := wrapped.Get(txCtx, &count, "SELECT COUNT(*) FROM table"); err != nil {
  _ = tx.Rollback(ctx)
  log.Fatalf("failed to get: %s", err)
 }

 if err := tx.Commit(ctx); err != nil {
  log.Fatalf("failed to commit transaction: %s", err)
 }
}
```

## Package txdb

 `txdb` is a single transaction based pgxext.Cluster. When the connection is opened, it starts a transaction and all operations performed on this cluster will be within that transaction.

Why is it useful. A very basic use case would be if you want to make functional tests you can prepare a test database and within each test you do not have to reload a database. All tests are isolated within transaction and though, performs fast.

### Usage

```golang
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
```

## License

© 2020 [Alexey Burmistrov]

Licensed under the [Apache License, Version 2.0](https://www.apache.org/licenses/LICENSE-2.0) ([`LICENSE`](LICENSE)). See the [`COPYRIGHT`](COPYRIGHT) file for more details.

`SPDX-License-Identifier: Apache-2.0`
