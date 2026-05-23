# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this project.

## Project purpose

A Go gRPC service skeleton. The echo service is a working example — copy its shape when adding new services.

All commands below run from this directory (`grpc/`).

## Build & run

```bash
make build          # builds bin/server and bin/client
make generate       # regenerates gen/ from proto sources (requires protoc — see below)
make tidy           # go mod tidy
make clean          # removes bin/

./bin/server                          # listens on :50051
./bin/client -msg "hello"             # echoes against localhost:50051
./bin/client -addr host:port -msg "x" # custom address
```

## Tests

```bash
go test ./...                          # all tests
go test ./internal/echo/ -v            # integration test with verbose output
go test ./internal/echo/ -run TestEchoIntegration/unicode  # single subtest
```

The integration test in `internal/echo/server_test.go` starts a real gRPC server bound to `127.0.0.1:0` (OS-assigned port) so it never conflicts with running services.

## Proto regeneration

Generated files in `gen/` are committed, so `go build` works without protoc. Regenerate only when `.proto` files change:

```bash
# One-time plugin install:
go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.6
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1
export PATH=$PATH:$(go env GOPATH)/bin

make generate
```

## Architecture

```
api/echo/v1/echo.proto      # source of truth for the service contract
gen/echo/v1/                # committed generated stubs (do not edit by hand)
internal/echo/server.go     # EchoServiceServer implementation
cmd/server/main.go          # server binary
cmd/client/main.go          # CLI client binary
```

**Adding a new service:** define a `.proto` in `api/<name>/v1/`, implement the server interface in `internal/<name>/`, register it in `cmd/server/main.go`, and run `make generate`.

**Adding interceptors** (logging, tracing, etc.): pass `grpc.ChainUnaryInterceptor(...)` to `grpc.NewServer()` in `cmd/server/main.go`.

The client uses `grpc.NewClient` (not the deprecated `grpc.Dial`) with explicit `insecure.NewCredentials()`. New clients should follow the same pattern.
