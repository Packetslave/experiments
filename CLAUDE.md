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

## blog/ — Hugo blog

Published at **https://packetslave.github.io/experiments/blog/**. Uses the `goodspace` theme in `blog/themes/goodspace/` — a local port of the GoodLayers GoodSpace WordPress theme (blog-with-right-sidebar and single-post templates), built for this site under a purchased ThemeForest license. See the theme's `README.md` for provenance and supported params. Unlike `docs/` (Tokyo Night), the blog intentionally uses the light GoodSpace design.

### Structure

```
blog/
  hugo.toml               # site config (baseURL, theme, menus, taxonomies)
  archetypes/default.md   # front matter template for new posts
  content/posts/          # one markdown file per post: YYYY-MM-DD-<slug>.md
  themes/goodspace/       # ported theme: templates + one hand-written CSS file
```

Do not add a root `blog/layouts/` directory — files there silently override the theme. Extend the theme in `blog/themes/goodspace/` instead. Optional front matter: `image` (featured image), `caption` (subtitle in the title bar).

### Posting

Use the `blog-post` skill (`.claude/skills/blog-post/SKILL.md`) — it covers front matter, file naming, and publishing. Short version: add a markdown file to `blog/content/posts/` on `main`; the front matter needs `title`, `date` (RFC 3339 UTC), `slug`, and optional `tags`. Posts with `draft: true` are excluded from the build.

**Writing style:** blog prose follows ASD-STE100 Simplified Technical English and the GOV.UK style guides. The skill has the distilled rules; consult the source guides for anything it doesn't cover.

### Deployment

The shared `.github/workflows/site.yml` workflow (see the docs/ section above) builds the blog with Hugo (pinned version, `--minify`) and deploys it to two targets. **Never deploy the blog by hand.**

- `deploy-pages` job: deploys to `blog/` on `gh-pages` together with the tools.
- `deploy-server` job: deploys to a personal server over Tailscale + rsync, using a forced-command (rrsync) SSH key and a build with the production `baseURL`. Gated behind the `SERVER_DEPLOY` repository variable; setup and required secrets/variables are documented in `blog/DEPLOY.md`.

### Local preview

```bash
cd blog && hugo server    # http://localhost:1313/experiments/blog/
hugo --minify             # production build into blog/public/ (gitignored)
```

### Adding a new tool

1. Create `docs/<tool-name>.html` following the shared theme and conventions above.
2. Add a card for it in `docs/index.html` (`.grid > .card`) — every tool must have a card, so the index stays a complete inventory.
3. Land both on `main` (directly or via a branch and merge). The "Deploy site" workflow publishes them; no manual `gh-pages` step.
