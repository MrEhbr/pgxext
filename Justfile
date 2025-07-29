export CGO_ENABLED := "0"

# Show available targets
help:
    @just --list

# Golang

# Generate
generate target="./...":
    @go generate -x {{target}}

# Run tests
test target="./..." *opts="-v -test.timeout=1m -cover":
    CGO_ENABLED=1 go tool gotestsum --format-hide-empty-pkg -- -race {{target}} {{opts}}

# Lint code
lint target="./..." *opts="-v":
    go tool golangci-lint run --fix {{opts}} {{target}}

# Format code
fmt target="./...":
    @go tool golines -m 180 -w `go list -f '{{{{.Dir}}' {{target}} | grep -v mocks`
    @go tool gofumpt -extra -l -w `go list -f '{{{{.Dir}}' {{target}} | grep -v mocks`

# Tidy dependencies
tidy:
    @go mod tidy

# Download dependencies
deps:
    @go mod download

# Start development database and export credentials
dev:
    @if ! docker ps --filter name=pgxext-dev --format "{{{{.Names}}}}" | grep -q pgxext-dev; then docker run --rm -d --name pgxext-dev -e POSTGRES_PASSWORD=dev -p 5433:5432 postgres:17; fi
    @echo "export PGXEXT_TEST_DATABASE_DSN=\"postgres://postgres:dev@localhost:5433/postgres?sslmode=disable\""
