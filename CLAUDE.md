# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository purpose

A playground for distributed systems algorithm experiments and browser-based utilities. Contains a Go gRPC module (`grpc/`) and a collection of static web tools (`docs/`) published to GitHub Pages. Additional experiments may be added as sibling directories at the repo root.

## grpc/ module

All commands below run from `grpc/`.

### Build & run

```bash
make build          # builds bin/server and bin/client
make generate       # regenerates gen/ from proto sources (requires protoc — see below)
make tidy           # go mod tidy
make clean          # removes bin/

./bin/server                          # listens on :50051
./bin/client -msg "hello"             # echoes against localhost:50051
./bin/client -addr host:port -msg "x" # custom address
```

### Tests

```bash
go test ./...                          # all tests
go test ./internal/echo/ -v            # integration test with verbose output
go test ./internal/echo/ -run TestEchoIntegration/unicode  # single subtest
```

The integration test in `internal/echo/server_test.go` starts a real gRPC server bound to `127.0.0.1:0` (OS-assigned port) so it never conflicts with running services.

### Proto regeneration

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
grpc/
  api/echo/v1/echo.proto      # source of truth for the service contract
  gen/echo/v1/                # committed generated stubs (do not edit by hand)
  internal/echo/server.go     # EchoServiceServer implementation
  cmd/server/main.go          # server binary
  cmd/client/main.go          # CLI client binary
```

**Adding a new service:** define a `.proto` in `api/<name>/v1/`, implement the server interface in `internal/<name>/`, register it in `cmd/server/main.go`, and run `make generate`.

**Adding interceptors** (logging, tracing, etc.): pass `grpc.ChainUnaryInterceptor(...)` to `grpc.NewServer()` in `cmd/server/main.go`.

The client uses `grpc.NewClient` (not the deprecated `grpc.Dial`) with explicit `insecure.NewCredentials()`. New clients should follow the same pattern.

## docs/ — web tools (GitHub Pages)

Static single-page browser utilities, published at **https://packetslave.github.io/experiments/**.

### Structure

```
docs/
  index.html            # landing page listing all tools
  json-formatter.html   # JSON pretty-printer with syntax highlighting
```

### GitHub Pages setup

- Served from the `gh-pages` branch, root folder.
- A `.nojekyll` file at the `gh-pages` root bypasses Jekyll so files are served as-is.
- The `docs/` folder on `main`/feature branches is the source of truth; changes must also be pushed to `gh-pages` to go live.
- There is no build step — edit the HTML files directly.

### Pushing changes live

Because this environment doesn't have `gh` CLI access, use the `mcp__github__create_or_update_file` or `mcp__github__push_files` MCP tools to commit directly to `gh-pages`. Always provide the current blob SHA when updating an existing file (`git rev-parse gh-pages:<path>` won't work for remote-only branches; fetch the SHA via `mcp__github__get_file_contents` instead).

### Conventions

- Each tool is a **single self-contained HTML file** with embedded CSS and JS — no bundler, no dependencies.
- All pages share a Tokyo Night dark theme (`--bg: #1a1b26`, `--accent: #7aa2f7`, etc.) via CSS custom properties defined in `:root`.
- Every page includes `<link rel="icon" href="data:image/svg+xml,...">` with an inline SVG favicon (accent-blue rounded square) to prevent 404s in request logs. Add this to any new page.
- Tool pages link back to the index via `<a class="back" href="index.html">← Tools</a>` in the header.

### Adding a new tool

1. Create `docs/<tool-name>.html` following the shared theme and conventions above.
2. Add a card for it in `docs/index.html` (`.grid > .card`).
3. Commit both to the feature branch.
4. Push both to `gh-pages` using the MCP file tools.
