package cluster

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"testing/quick"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/jackc/pgmock"
	"github.com/jackc/pgproto3/v2"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/matryer/is"
)

func testDatabase(t testing.TB, steps ...pgmock.Step) (string, func() error) {
	script := &pgmock.Script{
		Steps: append(
			[]pgmock.Step{
				pgmock.ExpectAnyMessage(&pgproto3.StartupMessage{ProtocolVersion: pgproto3.ProtocolVersionNumber, Parameters: map[string]string{}}),
				pgmock.SendMessage(&pgproto3.AuthenticationOk{}),
				pgmock.SendMessage(&pgproto3.ParameterStatus{Name: "client_encoding", Value: "UTF8"}),
				pgmock.SendMessage(&pgproto3.ParameterStatus{Name: "standard_conforming_strings", Value: "on"}),
				pgmock.SendMessage(&pgproto3.BackendKeyData{ProcessID: 0, SecretKey: 0}),
				pgmock.SendMessage(&pgproto3.ReadyForQuery{TxStatus: 'I'}),
			}, steps...),
	}

	ln, err := net.Listen("tcp", "127.0.0.1:")
	if err != nil {
		t.Fatal(err)
	}

	serverErrChan := make(chan error, 1)
	go func() {
		defer close(serverErrChan)

		conn, err := ln.Accept()
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()

		err = conn.SetDeadline(time.Now().Add(time.Second))
		if err != nil {
			t.Fatal(err)
		}

		err = script.Run(pgproto3.NewBackend(pgproto3.NewChunkReader(conn), conn))
		if err != nil && !errors.Is(err, os.ErrDeadlineExceeded) {
			t.Fatal(err)
		}
	}()

	parts := strings.Split(ln.Addr().String(), ":")
	return fmt.Sprintf("sslmode=disable prefer_simple_protocol=true host=%s port=%s", parts[0], parts[1]), ln.Close
}

func TestOpen(t *testing.T) {
	t.Run("multiple configs, one with error", func(t *testing.T) {
		is := is.New(t)
		db, err := Open([]string{"host=127.0.0.1", "host=127.0.0.2", "foobar"})

		is.True(err != nil)                               // expcect error when one config is invalid
		is.True(strings.Contains(err.Error(), "index 3")) // there must config index in error
		is.True(db == nil)                                // db must be nil
	})
	t.Run("multiple configs", func(t *testing.T) {
		is := is.New(t)

		one, oneClose := testDatabase(t, pgmock.ExpectMessage(&pgproto3.Terminate{}))
		defer oneClose()
		two, twoClose := testDatabase(t, pgmock.ExpectMessage(&pgproto3.Terminate{}))
		defer twoClose()
		three, threeClose := testDatabase(t, pgmock.ExpectMessage(&pgproto3.Terminate{}))
		defer threeClose()
		db, err := Open([]string{one, two, three})
		is.NoErr(err)
		defer db.Close()

		is.Equal(len(db.pdbs), 3) // must be 3 connections
	})
}

func TestNewFromConfigs(t *testing.T) {
	fn := func(dsn string) *pgxpool.Config {
		cfg, _ := pgxpool.ParseConfig(dsn)
		return cfg
	}

	t.Run("multiple configs", func(t *testing.T) {
		is := is.New(t)

		one, oneClose := testDatabase(t, pgmock.ExpectMessage(&pgproto3.Terminate{}))
		defer oneClose()
		two, twoClose := testDatabase(t, pgmock.ExpectMessage(&pgproto3.Terminate{}))
		defer twoClose()
		three, threeClose := testDatabase(t, pgmock.ExpectMessage(&pgproto3.Terminate{}))
		defer threeClose()
		db, err := NewFromConfigs([]*pgxpool.Config{fn(one), fn(two), fn(three)})
		is.NoErr(err)
		defer db.Close()

		is.Equal(len(db.pdbs), 3) // must be 3 connections
	})
}

func TestClose(t *testing.T) {
	t.Run("positive", func(t *testing.T) {
		is := is.New(t)

		one, oneClose := testDatabase(t, pgmock.ExpectMessage(&pgproto3.Terminate{}))
		defer oneClose()
		two, twoClose := testDatabase(t, pgmock.ExpectMessage(&pgproto3.Terminate{}))
		defer twoClose()
		three, threeClose := testDatabase(t, pgmock.ExpectMessage(&pgproto3.Terminate{}))
		defer threeClose()
		db, err := Open([]string{one, two, three})
		is.NoErr(err)

		is.Equal(len(db.pdbs), 3) // must be 3 connections
		db.Close()

		err = db.Ping(context.Background())
		is.True(err != nil)
		merr, ok := err.(*multierror.Error)
		is.True(ok)             // error must be multierr
		is.Equal(merr.Len(), 3) // all 3 db must return ping error
		for _, v := range merr.Errors {
			is.True(strings.Contains(v.Error(), "closed pool"))
		}
	})
}

func Test_replica(t *testing.T) {
	is := is.New(t)

	db := &Cluster{}
	last := -1

	err := quick.Check(func(n int) bool {
		index := db.replica(n)
		if n <= 1 {
			return index == 0
		}

		result := index > 0 && index < n && index != last
		last = index

		return result
	}, nil)
	is.NoErr(err)
}
