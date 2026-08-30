---
title: "Your Claude Desktop connectors are reusable MCP servers"
date: 2026-08-16T20:32:46Z
slug: "claude-desktop-connectors-mcp"
tags: ["mcp", "claude", "macos"]
draft: false
image: "images/library/patch-cables-01520a09.jpg"
imageCredit: "Photo: unknown photographer, CC0 (via Openverse)"
---

It turns out Claude Desktop connectors are just plain MCP servers sitting on
disk, and Claude Code can run the same server from its own config. Install
once and both clients get the integration. I had Fantastical, my calendar
app, talking to Claude Code in about 10 minutes.

## Where the servers live

I already had the Fantastical connector installed in Claude Desktop, which
raised the obvious question: where does Desktop keep the server it actually
runs? The answer is that each installed connector lives here:

```
~/Library/Application Support/Claude/Claude Extensions/
```

Mine contained `ant.dir.gh.flexibits.fantastical-mcp`, and that name was the
first pleasant surprise: this is Flexibits' official Fantastical MCP server,
not a community bridge.

Each connector folder has a `manifest.json` that spells out exactly how
Desktop launches the server:

```json
"server": {
  "type": "binary",
  "entry_point": "server/FantasticalMCP.app/Contents/MacOS/FantasticalMCP",
  "mcp_config": {
    "command": "${__dirname}/server/FantasticalMCP.app/Contents/MacOS/FantasticalMCP",
    "args": [],
    "env": {}
  }
}
```

There's no Desktop-specific magic in there. The `mcp_config` block is a
normal stdio MCP server invocation.

## Verify it before you wire it

A stdio MCP server has to answer a JSON-RPC `initialize` request on stdin.
One trap: some servers exit as soon as stdin closes, so if you `printf` the
request and close the pipe, you get nothing back. Keep the pipe open for a
few seconds:

```bash
( printf '%s\n' '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}'; sleep 3 ) \
  | "$HOME/Library/Application Support/Claude/Claude Extensions/ant.dir.gh.flexibits.fantastical-mcp/server/FantasticalMCP.app/Contents/MacOS/FantasticalMCP"
```

The server answered with its identity:

```json
{"id":1,"jsonrpc":"2.0","result":{"capabilities":{"tools":{"listChanged":true}},"protocolVersion":"2025-06-18","serverInfo":{"name":"Fantastical","version":"1.1.2"}}}
```

That's a working server, and I didn't install a thing to get it.

## Point Claude Code at it

Claude Code reads project MCP servers from `.mcp.json` at the repo root. The
entry is just the manifest's `mcp_config` with `${__dirname}` expanded to
the connector folder:

```json
"fantastical": {
  "command": "/Users/<you>/Library/Application Support/Claude/Claude Extensions/ant.dir.gh.flexibits.fantastical-mcp/server/FantasticalMCP.app/Contents/MacOS/FantasticalMCP",
  "args": []
}
```

Restart Claude Code, approve the new server, and the session picks up 5
calendar tools: `queryCalendars`, `queryCalendarItems`, `createCalendarItem`,
`modifyCalendarItem`, and `deleteCalendarItem`.

## Caveats

- The path only resolves on machines where the connector is installed. My
  `.mcp.json` lives in a repo that syncs between Macs; on a Mac without the
  connector, the server just shows as failed and nothing else breaks.
- Updates flow through. The connector folder name has no version in it, so
  Desktop updates the binary in place and Claude Code picks it up for free.
- The Fantastical server is macOS-only and talks to the local Fantastical
  app. Other connectors vary — check the `platforms` field in the manifest.

## The general recipe

1. List `~/Library/Application Support/Claude/Claude Extensions/`.
2. Open the connector's `manifest.json` and find `server.mcp_config`.
3. Expand `${__dirname}` to the connector's absolute path.
4. Copy the command, args, and env into `.mcp.json`.
5. Test with the `initialize` one-liner before you restart anything.

Node-based connectors work the same way — the command is `node` plus a
script path instead of a binary. One install, both clients, and the vendor's
updates come along for the ride.
