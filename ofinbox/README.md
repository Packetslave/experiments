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
make build           # builds bin/ofinbox
./bin/ofinbox        # process the real inbox (macOS)
./bin/ofinbox -demo  # sample data, works anywhere
./bin/ofinbox -serve # phone web app + REST API instead of the TUI
make test            # unit tests (no OmniFocus needed)
```

## Keys

| Key | Action |
| --- | --- |
| `j` / `k` (or arrows) | next / previous item |
| `g` / `G` (or home/end) | first / last item |
| `c` | mark complete |
| `d` | drop |
| `f` | file to a project (type-to-filter picker) |
| `l` | file a link item to the links project (see below) |
| `o` | open a link item in the default browser |
| `t` | add a tag (type-to-filter picker; repeat for more tags) |

In the `f` and `t` pickers, typing a name that doesn't exactly match an
existing project or tag adds a `+ new …` row at the bottom of the list;
selecting it creates the project (top level, active) or tag (top level)
and files/tags the item with it in one step.
| `!` | toggle flag |
| `L` | toggle links-only filter (see below) |
| `enter` | open an action group's subtasks in the queue |
| `e` | edit title |
| `s` | set defer date |
| `u` | set due date |
| `r` | reload from OmniFocus |
| `q` | quit |

Completing, dropping, or filing an item removes it from the session and
counts toward the "processed" tally in the header.

### Link items

Links dropped into the inbox get special handling. An item counts as a
link — and shows a `🔗 link` badge — when its title or its note, trimmed,
is exactly one URL: a URL-only note qualifies with or without a title,
and a URL-only title qualifies regardless of the note. Prose around the
URL disqualifies it. `l` files a link in one keystroke: it adds the
`NoAction` tag and moves the item to the "Links to Review" project
(preferring the one in the "Personal" folder). Both names are hardcoded
for now and will become preferences later. `o` opens a link in the
default browser (via `open` on macOS, `xdg-open` elsewhere) without
touching the item, so you can peek before deciding what to do with it.

`L` toggles a links-only filter for batch-processing captured links:
navigation (`j`/`k`/`g`/`G`) skips everything that isn't a link, the
header shows a `🔗 links only` badge, and the position line counts links
(`link 2 of 5 · 12 in inbox`). The rest of the inbox is untouched —
non-link items are just hidden until you press `L` again. Processing the
last link shows "No links in the inbox" rather than dropping you back
into the full list.

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

## Phone interface (`-serve`)

`ofinbox -serve` runs an HTTP server instead of the TUI: a thin REST API
over the same client operations, plus an embedded single-file web app
built for one-handed phone use. It binds `127.0.0.1:4747` by default
(`-addr` to change); `-demo -serve` serves the sample data on any
platform, which is also how the frontend is developed and tested.

The app shows the inbox one card at a time — Complete / Drop / File
large, Tag / Flag / Defer / Due below, ‹ › to skip — with the TUI's
link handling (one-tap "→ Links to Review" quick-file, Open, links-only
filter), type-to-filter pickers with `+ new …` creation, and quick-pick
date chips (Today, Tomorrow, This weekend, Next week, +1 week) backed by
the native date picker. Action groups act as one unit and ask for a
confirming second tap on complete/drop. When the queue is done you get
the processed tally. There is no undo, matching the TUI: both complete
and drop are recoverable inside OmniFocus.

Intended deployment is an always-on Mac reachable over Tailscale only:

```bash
ofinbox -serve &
tailscale serve --bg localhost:4747
```

giving `https://<host>.<tailnet>.ts.net/` with a real certificate and no
auth code — the tailnet is the security boundary. On iOS, Share → "Add
to Home Screen" installs it as a standalone app named "Inbox". Nothing
host-specific is baked in; moving hosts means building the binary,
granting the Automation permission once, and repeating the
`tailscale serve` mapping.

API sketch (all JSON): `GET /api/inbox|projects|tags`;
`POST /api/tasks/{id}/complete|drop|move|tag|flag|defer|due`;
`POST /api/projects` and `POST /api/tags` to create. Mutations return
204; a task that no longer exists is 410 (the app just advances);
malformed bodies are 400; OmniFocus failures are 502. The server holds a
mutex across client calls so concurrent requests never interleave
osascript Apple Events, and every inbox fetch is a fresh read — the
phone keeps its own queue for the session like the TUI does.

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
