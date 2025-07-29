# pgxext

[![CI](https://github.com/MrEhbr/pgxext/actions/workflows/ci.yml/badge.svg)](https://github.com/MrEhbr/pgxext/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://github.com/MrEhbr/pgxext/blob/main/COPYRIGHT)
[![Go Report Card](https://goreportcard.com/badge/github.com/MrEhbr/pgxext)](https://goreportcard.com/report/github.com/MrEhbr/pgxext)
[![GoDoc](https://godoc.org/github.com/MrEhbr/pgxext?status.svg)](https://godoc.org/github.com/MrEhbr/pgxext)

**pgxext** is a comprehensive collection of PostgreSQL extensions for the [pgx](https://github.com/jackc/pgx) v5 driver, designed to simplify database operations, cluster management, and testing workflows.

## Features

- **Cluster Management**: Primary-replica topology with automatic read/write routing
- **Enhanced Querying**: Simplified scanning with struct binding support
- **Transaction Management**: Advanced transaction handling with timeout controls
- **Testing Utilities**: Transaction-based testing for isolated test environments

## Installation

```bash
go get github.com/MrEhbr/pgxext
```

## Architecture

pgxext is organized into three main packages:

- **cluster/** - Primary-replica database abstraction
- **conn/** - Enhanced querying & transactions
- **txdb/** - Testing utilities

## Packages

### cluster - Primary-Replica Database Management

Abstracts primary-replica physical database topologies as a single logical database. Automatically routes reads to replicas and writes to primary with round-robin load balancing.

### conn - Enhanced Database Querying

Simplifies querying and scanning with automatic struct binding, transaction context management, and configurable timeouts.

### txdb - Transaction-Based Testing

Single transaction-based database wrapper for fast, isolated functional tests without database reloads.

## Quick Start

See the [`examples/`](examples/) directory for complete working examples:

- [`examples/cluster/`](examples/cluster/) - Primary-replica cluster usage
- [`examples/conn/`](examples/conn/) - Enhanced querying and transactions
- [`examples/txdb/`](examples/txdb/) - Testing with transaction isolation

## Testing

### Running Tests

```bash
# Set up test database
export PGXEXT_TEST_DATABASE_DSN="postgres://user:password@localhost:5432/testdb"

# Run tests with coverage
just test

# Start development database
just dev
```

### Integration Tests

Tests require a PostgreSQL database. If `PGXEXT_TEST_DATABASE_DSN` is not set, integration tests are automatically skipped.

## Development Commands

This project uses [Just](https://github.com/casey/just) as a command runner:

- `just test` - Run tests with race detection and coverage
- `just lint` - Run golangci-lint with comprehensive rules
- `just fmt` - Format code using golines and gofumpt
- `just dev` - Start development database

## Contributing

We welcome contributions! Please:

1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Run `just lint` and `just test`
5. Submit a pull request

## License

© 2025 [Alexey Burmistrov](https://github.com/MrEhbr)

Licensed under the [Apache License, Version 2.0](https://www.apache.org/licenses/LICENSE-2.0) ([`LICENSE`](LICENSE)). See the [`COPYRIGHT`](COPYRIGHT) file for more details.

`SPDX-License-Identifier: Apache-2.0`
