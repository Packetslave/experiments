# webdev — Architecture & Developer Guide

## Overview

This is a React SPA. The key insight for understanding how everything fits together is that **most of the "magic" happens at build time in Vite**, not at runtime. Three Vite plugins transform source files before a single byte reaches the browser:

1. **TanStackRouterVite** — scans `src/routes/` and writes `src/routeTree.gen.ts`
2. **@vitejs/plugin-react** — compiles JSX/TSX and enables Fast Refresh in dev
3. **@tailwindcss/vite** — processes `src/index.css` and emits utility classes on demand

Everything else — routing, data fetching, components — runs in the browser and is built on top of that compiled output.

---

## How the pieces connect

### Boot sequence (what happens when the browser loads the app)

```
index.html
  └── <script src="/src/main.tsx">
        ├── creates QueryClient          (TanStack Query)
        ├── creates router from routeTree (TanStack Router)
        └── renders:
              QueryClientProvider
                └── RouterProvider
                      └── __root.tsx (layout)
                            └── <Outlet /> → matched route component
```

`main.tsx` is the single composition root. It wires the two "providers" together: `QueryClientProvider` makes the query cache available to any component below it via `useQuery`/`useMutation`; `RouterProvider` owns navigation state and renders whatever route matches the current URL.

The `Register` module augmentation at the bottom of `main.tsx` is what makes `<Link to="...">` and `useNavigate()` type-safe everywhere in the app — TypeScript knows all valid routes because the router instance is registered globally there.

### How routing works

TanStack Router uses **file-based routing**. The Vite plugin watches `src/routes/` and regenerates `src/routeTree.gen.ts` whenever a file is added, removed, or renamed. You never edit `routeTree.gen.ts` — it's an artefact.

File → URL mapping:

| File | URL |
|------|-----|
| `src/routes/__root.tsx` | (layout wrapper for all routes) |
| `src/routes/index.tsx` | `/` |
| `src/routes/about.tsx` | `/about` |
| `src/routes/posts/$id.tsx` | `/posts/:id` |
| `src/routes/settings/index.tsx` | `/settings` |
| `src/routes/settings/profile.tsx` | `/settings/profile` |

`__root.tsx` always renders; it provides the persistent shell (nav, footer, etc.) and an `<Outlet />` where the matched child route appears. Every other route file exports `const Route = createFileRoute(path)({ component: ... })`.

### How styling works

TailwindCSS v4 has **no config file**. All configuration lives in `src/index.css`:

```
src/index.css
  @import "tailwindcss"           ← loads Tailwind engine
  @custom-variant dark (...)      ← defines how .dark class activates dark mode
  @theme inline { ... }           ← maps Tailwind color names → CSS variables
  :root { --background: ... }     ← light mode tokens (oklch values)
  .dark { --background: ... }     ← dark mode tokens
  @layer base { ... }             ← resets applied to all elements
```

The chain for a class like `bg-background`:
1. Tailwind sees `bg-background` used in a component
2. `@theme inline` says `--color-background` maps to `var(--background)`
3. `:root` defines `--background: oklch(1 0 0)` (white)
4. At runtime the browser resolves `bg-background` → `background-color: oklch(1 0 0)`

shadcn components use these semantic token names (e.g. `bg-primary`, `text-muted-foreground`, `border-input`) via `cva` so they automatically adapt to any theme you define in `:root`.

### How shadcn components are structured

Every component in `src/components/ui/` follows the same three-layer pattern:

```typescript
// 1. cva defines the variant map
const buttonVariants = cva('base-classes', {
  variants: { variant: { default: '...', destructive: '...' } },
  defaultVariants: { variant: 'default' },
})

// 2. The component merges props with variants via cn()
function Button({ className, variant, asChild, ...props }) {
  const Comp = asChild ? Slot : 'button'   // Radix Slot for polymorphism
  return <Comp className={cn(buttonVariants({ variant }), className)} {...props} />
}
```

`cn()` in `src/lib/utils.ts` is `clsx` + `tailwind-merge`: it concatenates class names and resolves Tailwind conflicts (so passing `className="bg-red-500"` correctly overrides the component's default `bg-primary`).

`asChild` (from Radix `Slot`) lets you use a component's styles on a different element: `<Button asChild><a href="/foo">Link</a></Button>` renders an `<a>` with button styles.

### How TypeScript is structured

There are two tsconfigs, composed by the root `tsconfig.json`:

| Config | Covers | Key types |
|--------|--------|-----------|
| `tsconfig.app.json` | `src/**` | DOM, vite/client, vitest/globals, path alias `@/*` |
| `tsconfig.node.json` | `vite.config.ts` | Node, no DOM |

The split exists because `vite.config.ts` runs in Node and shouldn't see DOM types, while app code runs in the browser and shouldn't see Node types. `pnpm build` runs `tsc -b` (project references mode) which compiles both in one pass before Vite bundles.

### How linting is split

Two linters run in sequence via `pnpm lint`:

- **oxlint** runs first — fast Rust linter, catches common JS/TS issues, ignores `routeTree.gen.ts`
- **eslint** runs second — slower but understands TypeScript types, React Hooks rules, and Storybook conventions

`oxfmt` is a separate formatter (`pnpm format`). It formats `src/` only and uses double quotes (the style it enforced when it was first run on this codebase).

---

## Common tasks

### Add a new page

1. Create `src/routes/your-page.tsx`:

```typescript
import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/your-page')({
  component: YourPage,
})

function YourPage() {
  return <main className="p-8"><h1 className="text-3xl font-bold">Your Page</h1></main>
}
```

2. Run `pnpm dev` (or `pnpm build`) — the router plugin regenerates `routeTree.gen.ts` automatically.
3. Link to it from anywhere: `<Link to="/your-page">Go</Link>`. TypeScript will error if the path doesn't exist.

For a **nested route**, create `src/routes/parent/child.tsx` and ensure `src/routes/parent.tsx` (or `src/routes/parent/index.tsx`) renders `<Outlet />`.

For a **dynamic segment**, name the file `$paramName.tsx` and access it with `Route.useParams()`.

### Add a new shadcn/ui component

> The shadcn CLI cannot run in this environment (requires network access to ui.shadcn.com). Copy manually.

1. Find the component at https://ui.shadcn.com (use New York style).
2. Create `src/components/ui/component-name.tsx` and paste the source.
3. Adjust imports: change any `@/lib/utils` references (already correct), install any new Radix packages the component needs (`pnpm add @radix-ui/react-<name>`).
4. Add a story alongside it (see below).

Radix packages provide the accessible, unstyled primitive — the shadcn file adds the Tailwind classes on top. They always come as a pair.

### Add a TanStack Query hook

Create `src/hooks/useThings.ts`:

```typescript
import { useQuery } from '@tanstack/react-query'

async function fetchThings(): Promise<Thing[]> {
  const res = await fetch('/api/things')
  if (!res.ok) throw new Error('Failed to fetch')
  return res.json()
}

export function useThings() {
  return useQuery({
    queryKey: ['things'],
    queryFn: fetchThings,
  })
}
```

Use in a component:

```typescript
const { data, isPending, error } = useThings()
```

Query keys are how TanStack Query identifies and invalidates cache entries. Use arrays: `['things']` for a list, `['things', id]` for a single item. For mutations, use `useMutation` and call `queryClient.invalidateQueries({ queryKey: ['things'] })` in `onSuccess` to refetch.

### Add a route with data loading

TanStack Router's `loader` runs before the component renders, avoiding the "fetch on render" waterfall:

```typescript
export const Route = createFileRoute('/posts')({
  loader: async ({ context: { queryClient } }) => {
    await queryClient.ensureQueryData({ queryKey: ['posts'], queryFn: fetchPosts })
  },
  component: Posts,
})

function Posts() {
  const { data } = useSuspenseQuery({ queryKey: ['posts'], queryFn: fetchPosts })
  // data is guaranteed non-null here
}
```

The `queryClient` is available in loaders because it's passed as router context in `main.tsx` (`createRouter({ routeTree, context: { queryClient } })`).

### Write a unit test

Create `src/components/ui/your-component.test.tsx` next to the component:

```typescript
import { render, screen } from '@testing-library/react'
import { userEvent } from '@testing-library/user-event'
import { YourComponent } from './your-component'

describe('YourComponent', () => {
  it('does the thing', async () => {
    const user = userEvent.setup()
    render(<YourComponent />)
    await user.click(screen.getByRole('button', { name: 'Submit' }))
    expect(screen.getByText('Success')).toBeInTheDocument()
  })
})
```

Run the file in isolation: `pnpm vitest run src/components/ui/your-component.test.tsx`

The test runs in jsdom (a simulated DOM, not a real browser). `@testing-library/jest-dom` matchers like `toBeInTheDocument()` and `toHaveClass()` are available globally because `src/test/setup.ts` imports it and `vite.config.ts` lists that file in `setupFiles`.

### Write an e2e test

Create `e2e/your-feature.spec.ts`:

```typescript
import { test, expect } from '@playwright/test'

test('user can submit the form', async ({ page }) => {
  await page.goto('/your-page')
  await page.getByLabel('Name').fill('Alice')
  await page.getByRole('button', { name: 'Submit' }).click()
  await expect(page.getByText('Welcome, Alice')).toBeVisible()
})
```

Run it: `pnpm e2e` (Playwright starts the dev server automatically via `webServer` in `playwright.config.ts`). For a single file: `pnpm playwright test e2e/your-feature.spec.ts`.

**Prerequisite:** Chromium must be installed once: `pnpm exec playwright install chromium`.

E2e tests talk to a real running app over HTTP. They're slower and more brittle than unit tests but catch integration problems that jsdom can't — use them for critical user journeys, not component internals.

### Write a Storybook story

Create `src/components/ui/your-component.stories.tsx`:

```typescript
import type { Meta, StoryObj } from '@storybook/tanstack-react'
import { YourComponent } from './your-component'

const meta: Meta<typeof YourComponent> = {
  component: YourComponent,
  tags: ['autodocs'],       // generates a Props table automatically
}
export default meta

type Story = StoryObj<typeof YourComponent>

export const Default: Story = { args: { label: 'Click me' } }
export const Disabled: Story = { args: { label: 'Click me', disabled: true } }
```

Run Storybook: `pnpm storybook` (opens on port 6006).

Stories use the `@storybook/tanstack-react` framework. TanStack Router and Query contexts are available inside stories if needed. The `tags: ['autodocs']` line generates a documentation page from prop types automatically.

### Change the colour theme

All colours are defined in `src/index.css` as `oklch()` values. `oklch` is a perceptually uniform colour space — the three numbers are **Lightness** (0–1), **Chroma** (0–0.4), **Hue** (0–360).

To change the primary colour to blue:

```css
:root {
  --primary: oklch(0.3 0.15 240);           /* dark blue */
  --primary-foreground: oklch(0.98 0 0);    /* near-white */
}
.dark {
  --primary: oklch(0.75 0.12 240);          /* lighter blue for dark mode */
  --primary-foreground: oklch(0.1 0 0);     /* near-black */
}
```

Every shadcn component that uses `bg-primary` or `text-primary` updates automatically — no other files need touching.

The `--radius` variable (currently `0.625rem`) controls the corner rounding across all components: `--radius-sm/md/lg/xl` are computed from it in `@theme inline`.

---

## Dependency map

```
index.html
└── src/main.tsx
      ├── src/index.css ──────────────── @tailwindcss/vite processes this
      ├── src/routeTree.gen.ts ────────── generated by TanStackRouterVite plugin
      │     └── imports src/routes/*.tsx
      ├── @tanstack/react-query ────────  QueryClient / useQuery
      └── @tanstack/react-router ───────  createRouter / RouterProvider

src/routes/__root.tsx
└── src/components/ui/*.tsx
      ├── @radix-ui/* ─────────────────  accessible primitives
      ├── class-variance-authority ────  variant logic (cva)
      └── src/lib/utils.ts (cn) ───────  clsx + tailwind-merge

Test toolchain (dev-only, never in the bundle):
  Vitest ←─ vite.config.ts test block ←─ src/test/setup.ts
  Playwright ←─ playwright.config.ts ←─ e2e/*.spec.ts
  Storybook ←─ .storybook/main.ts ←─ src/**/*.stories.tsx
               .storybook/preview.tsx (imports index.css for theming)
```
