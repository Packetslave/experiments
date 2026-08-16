# ofinbox

A keyboard-driven TUI for processing your OmniFocus inbox, built with
[Bubble Tea](https://github.com/charmbracelet/bubbletea) and Lip Gloss. It
steps through inbox items one at a time so you can complete, drop, file,
tag, flag, or schedule each one with a single keystroke.

Instead of depending on a third-party `of` CLI, it talks to OmniFocus
directly through `osascript` with embedded JXA scripts — nothing to install
beyond the binary itself.

## Requirements

- macOS with OmniFocus 3 or 4 installed
- Go 1.24+ to build

The first run will prompt for an Automation permission ("Terminal wants to
control OmniFocus") — grant it, or fix it later in **System Settings →
Privacy & Security → Automation**.

On any platform (including Linux) you can try the UI with built-in sample
data: `ofinbox -demo`.

## Build & run

```bash
make build          # builds bin/ofinbox
./bin/ofinbox       # process the real inbox (macOS)
./bin/ofinbox -demo # sample data, works anywhere
make test           # unit tests (no OmniFocus needed)
```

## Keys

| Key | Action |
| --- | --- |
| `j` / `k` (or arrows) | next / previous item |
| `g` / `G` (or home/end) | first / last item |
| `c` | mark complete |
| `d` | drop |
| `f` | file to a project (type-to-filter picker) |
| `t` | add a tag (type-to-filter picker; repeat for more tags) |
| `!` | toggle flag |
| `enter` | open an action group's subtasks in the queue |
| `e` | edit title |
| `s` | set defer date |
| `u` | set due date |
| `r` | reload from OmniFocus |
| `q` | quit |

Completing, dropping, or filing an item removes it from the session and
counts toward the "processed" tally in the header.

### Action groups (items with subtasks)

An item with children shows a `▸ N subtasks` badge. All actions treat the
group as one unit — `f` files the whole subtree intact, which is usually
what a multi-part capture wants. `enter` instead fetches the children
(lazily, one osascript round trip for that group) and splices them into
the queue ahead of the parent, each labeled `under "…"`, so you can
process them individually; the parent comes up last as a normal item.
Because OmniFocus cascades complete/drop to the whole subtree, `c` and
`d` on a group must be pressed twice to confirm. `r` collapses any
opened groups (it reloads the top-level inbox).

### Dates

The defer/due prompts accept quick shorthand: `today`, `tomorrow`, `fri`
(next Friday), `3d`, `2w`, `2026-08-20`, `2026-08-20 14:30`, or `14:30`
(today). An empty entry clears the date. Bare dates default to 8:00 for
defer and 17:00 for due.

## Design notes

- `internal/omnifocus` defines a small `Client` interface with two
  implementations: `OsascriptClient` (embedded JXA scripts run via
  `osascript -l JavaScript`, see `internal/omnifocus/scripts/`) and
  `DemoClient` (in-memory, used by `-demo` and the tests).
- `internal/tui` holds the Bubble Tea model. All mutations run as async
  commands against the `Client`; on success the local copy of the task is
  updated (or removed) without a full reload, so the loop stays fast.
- Reads happen once at startup (`r` refreshes) — inbox, projects, and tags
  are each a single osascript round trip returning JSON. The scripts fetch
  properties columnar-style (one Apple Event per property for the whole
  list) rather than per-task; per-task access costs one Apple Event per
  property per task and takes ~45s on a ~200-item inbox versus ~0.2s.
- There is no undo; `d` uses OmniFocus's "dropped" status rather than
  deletion, so nothing is destroyed irrecoverably.
