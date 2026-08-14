# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository purpose

A playground for distributed systems algorithm experiments and browser-based utilities. Contains a Go gRPC module (`grpc/`), a collection of static web tools (`docs/`) published to GitHub Pages, and a Hugo blog (`blog/`) deployed to the same Pages site. Additional experiments may be added as sibling directories at the repo root.

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
  index.html               # landing page listing all tools
  json-formatter.html      # JSON pretty-printer with syntax highlighting
  regex-tester.html        # live regex matching with groups and flags
  blackjack-trainer.html   # basic strategy practice with session stats
  retirement-planner.html  # retirement savings projection with charts
```

### GitHub Pages setup

- Served from the `gh-pages` branch, root folder. A `.nojekyll` file at the root bypasses Jekyll so files are served as-is.
- **`main` is the source of truth and `gh-pages` is build output — never edit or push to `gh-pages` by hand.** `.github/workflows/site.yml` rebuilds the entire branch on every push to `main` that touches `docs/**` or `blog/**`: it copies `docs/` to the root and builds the Hugo blog into `blog/`. Anything on `gh-pages` that the workflow did not produce is deleted on the next deploy, so hand-pushed files will be lost.
- To go live, land changes on `main` (directly or by merging a branch). To redeploy without a change, trigger "Deploy site" manually via `workflow_dispatch`.
- There is no build step for tools — edit the HTML files directly.

### Conventions

- Each tool is a **single self-contained HTML file** with embedded CSS and JS — no bundler, no dependencies.
- All pages share a Tokyo Night dark theme (`--bg: #1a1b26`, `--accent: #7aa2f7`, etc.) via CSS custom properties defined in `:root`.
- Every page includes `<link rel="icon" href="data:image/svg+xml,...">` with an inline SVG favicon (accent-blue rounded square) to prevent 404s in request logs. Add this to any new page.
- Tool pages link back to the index via `<a class="back" href="index.html">← Tools</a>` in the header.

### Adding a new tool

1. Create `docs/<tool-name>.html` following the shared theme and conventions above.
2. Add a card for it in `docs/index.html` (`.grid > .card`) — every tool must have a card, so the index stays a complete inventory.
3. Land both on `main` (directly or via a branch and merge). The "Deploy site" workflow publishes them; no manual `gh-pages` step.

## blog/ — Hugo blog

Published at **https://packetslave.github.io/experiments/blog/**. Uses the `goodspace` theme in `blog/themes/goodspace/` — a local Hugo port of the GoodSpace WordPress theme (purchased license). The theme's `README.md` documents provenance, site params, and front matter. Intentionally a light design, unlike the Tokyo Night `docs/` tools.

```
blog/
  hugo.toml               # site config (baseURL, theme, menus, taxonomies)
  archetypes/default.md   # front matter template for new posts
  content/posts/          # one markdown file per post: YYYY-MM-DD-<slug>.md
  content/links/          # link posts (twitter-style link sharing, /links/)
  content/thoughts/       # tweet-length thoughts (home feed only)
  static/images/posts/    # hero images (B&W, 1280x480, CC0/public domain)
  static/images/library/  # reusable hero image library + manifest.json
  themes/goodspace/       # ported theme: templates + one hand-written CSS file
```

The "Fetch hero image" workflow (`.github/workflows/fetch-hero.yml` + `scripts/fetch_hero.py`) pulls CC0/public-domain images from Openverse into the library and attaches them to posts; the `blog-post` skill documents when and how to trigger it.

**Posting: use the `blog-post` skill.** It owns all posting guidelines — writing style, front matter, hero image sourcing/licensing/processing, and the publish flow (commit to `main`; CI deploys).

**Deployment:** `.github/workflows/site.yml` deploys on every push to `main` — GitHub Pages always, plus a personal server over Tailscale + rsync when the `SERVER_DEPLOY` repository variable is enabled (setup in `blog/DEPLOY.md`). **Never deploy by hand.**

**Theme changes:** edit `blog/themes/goodspace/`. Do not add a root `blog/layouts/` directory — files there silently override the theme.

Local preview:

```bash
cd blog && hugo server    # http://localhost:1313/experiments/blog/
hugo --minify             # production build into blog/public/ (gitignored)
```
