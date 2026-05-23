# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this project.

## Project purpose

A React SPA scaffold built on the modern frontend stack:

- **Build:** Vite 8 + TypeScript 6, **pnpm** as the package manager
- **Styling:** TailwindCSS v4 + shadcn/ui (New York style, neutral palette)
- **Data/Routing:** TanStack Query v5, TanStack Router v1 (file-based)
- **Tests:** Vitest 4 (jsdom + Testing Library) for unit, Playwright 1.60 for e2e
- **Stories:** Storybook 10 (`@storybook/tanstack-react` framework)
- **Lint/Format:** oxlint + oxfmt (Rust-based, replacing the ESLint+Prettier default for speed; ESLint stays for TS-specific rules)

All commands below run from this directory (`webdev/`).

## Commands

```bash
pnpm dev                # start Vite dev server (http://localhost:5173)
pnpm build              # tsc -b && vite build
pnpm preview            # serve dist/

pnpm test               # vitest watch mode
pnpm test:run           # single run (CI)
pnpm test:ui            # @vitest/ui browser dashboard

pnpm e2e                # playwright test (auto-starts dev server via webServer in config)
pnpm e2e:ui             # playwright --ui

pnpm storybook          # storybook dev on :6006
pnpm build-storybook

pnpm lint               # oxlint (fast) THEN eslint (TS rules)
pnpm lint:ox            # oxlint only
pnpm format             # oxfmt src
pnpm format:check       # CI-safe check

# Single test
pnpm vitest run src/components/ui/button.test.tsx
pnpm playwright test e2e/home.spec.ts
```

### One-time setup before first `pnpm e2e`

Playwright browsers are not bundled — install chromium:

```bash
pnpm exec playwright install chromium
```

The Storybook component-test integration (browser-mode Vitest via `@storybook/addon-vitest`) also needs this. It was intentionally removed from `vite.config.ts` to keep `pnpm test:run` green without browsers; re-add the multi-project setup if you need it.

## Architecture

### Routing — file-based, auto-generated

`@tanstack/router-plugin/vite` watches `src/routes/` and writes `src/routeTree.gen.ts` on every Vite build / dev run. **Do not edit `routeTree.gen.ts` by hand** — it's committed only so cold builds work without a router pre-pass.

```
src/routes/__root.tsx   # root layout (nav, Outlet, devtools)
src/routes/index.tsx    # "/" route
src/routes/<path>.tsx   # new routes — file = URL segment
src/routeTree.gen.ts    # GENERATED — do not edit
```

`src/main.tsx` declares the `Register` module augmentation so the router is fully typed across the app — keep that block in sync if you ever swap router instances.

### Styling — TailwindCSS v4

There is **no `tailwind.config.js`**. v4 uses the `@tailwindcss/vite` plugin and configures everything in CSS:

- `src/index.css` imports Tailwind (`@import "tailwindcss"`), declares the dark variant, maps the shadcn CSS variables into `@theme inline`, and defines the light/dark color tokens in `:root` / `.dark`.
- Change theme colors by editing the `oklch(...)` values in that file, not by adding a JS config.

### shadcn/ui — manual setup, not CLI-managed

The shadcn CLI was **not** used to scaffold this project because (a) it requires network access to `ui.shadcn.com` (sandboxed env can't reach it) and (b) older CLI versions don't recognize TailwindCSS v4's CSS-only config. Components are copied in manually:

1. Find the component on https://ui.shadcn.com (New York style, neutral palette to match)
2. Create the file under `src/components/ui/`
3. Use `cn` from `@/lib/utils` and `cva` from `class-variance-authority`
4. Add a `.stories.tsx` alongside it for Storybook

`components.json` exists for editor tooling but `pnpm dlx shadcn add ...` will fail in this environment. The `Button` in `src/components/ui/button.tsx` is the canonical reference.

### Path alias `@/*`

`@` resolves to `src/`. It must be declared **in three places** and they must stay in sync:

- `vite.config.ts` → `resolve.alias` (runtime resolution)
- `tsconfig.app.json` → `paths` + `baseUrl: "."` (TS resolution)
- `components.json` → `aliases` (for any future shadcn CLI use)

The `ignoreDeprecations: "6.0"` in `tsconfig.app.json` silences a TS6 warning about `baseUrl`; it's required as long as `paths` is used.

### Test layout

- **Unit tests** (`*.test.ts(x)`) live next to source. Run by Vitest in jsdom. Setup in `src/test/setup.ts` (just imports `@testing-library/jest-dom`).
- **E2E tests** live in `e2e/`. `vite.config.ts` test block has `exclude: [..., 'e2e/**']` so Vitest doesn't try to load Playwright specs — that exclusion is load-bearing, do not remove.
- `playwright.config.ts` uses `webServer` to auto-start `pnpm dev` on port 5173.

### pnpm overrides for supply-chain policy

The user's environment enforces a `minimumReleaseAge` policy (rejects packages published in the last ~24h). `package.json` `pnpm.overrides` pins transitive deps that frequently fail this check:

```json
"overrides": {
  "tinyexec": "1.1.2",
  "tldts": "7.0.30",
  "tldts-core": "7.0.30"
}
```

When updating direct deps, if `pnpm install` rejects the lockfile, query `pnpm view <pkg> time --json` to find the latest version published before the cutoff, then pin direct deps to that version or add a new override entry. **Do not relax the policy** — it's enforced upstream.

### Storybook framework

Uses `@storybook/tanstack-react` (not `@storybook/react-vite`), which is the new TanStack-aware framework. `.storybook/preview.tsx` imports `../src/index.css` so stories get the TailwindCSS + shadcn theme; don't remove that import.
