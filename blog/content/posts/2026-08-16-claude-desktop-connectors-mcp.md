---
title: "Your Claude Desktop connectors are reusable MCP servers"
date: 2026-08-16T20:32:46Z
slug: "claude-desktop-connectors-mcp"
tags: ["mcp", "claude", "macos"]
draft: false
image: "images/library/patch-cables-01520a09.jpg"
imageCredit: "Photo: unknown photographer, CC0 (via Openverse)"
---

Claude Desktop connectors are plain MCP servers on disk. Claude Code can run
the same server from its own config. You install once, and both clients get
the integration. I connected Fantastical, my calendar app, to Claude Code
this way in about 10 minutes.

## The false start

My first plan was a third-party Fantastical bridge from an MCP directory
site. The network I was on killed that plan. A captive portal intercepted
TLS for the directory site, and curl got a certificate for
`secure-login.attwifi.com` instead. GitHub worked. The directory site did
not.

That failure led to a better question. I already had the Fantastical
connector installed in Claude Desktop. Where does Desktop keep the server it
runs?

## Where the servers live

Claude Desktop stores each installed connector here:

```
~/Library/Application Support/Claude/Claude Extensions/
```

Mine contained `ant.dir.gh.flexibits.fantastical-mcp`. That name was the
first good surprise: the connector is the official Fantastical MCP server
from Flexibits, not a community bridge.

Each connector folder has a `manifest.json`. The manifest tells you exactly
how Desktop launches the server:

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

There is no Desktop-specific magic in there. The `mcp_config` block is a
normal stdio MCP server invocation.

## Verify it before you wire it

A stdio MCP server must answer a JSON-RPC `initialize` request on stdin. One
trap: some servers exit when stdin closes. If you `printf` the request and
close the pipe, you get nothing back. Keep the pipe open for a few seconds:

```bash
( printf '%s\n' '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}'; sleep 3 ) \
  | "$HOME/Library/Application Support/Claude/Claude Extensions/ant.dir.gh.flexibits.fantastical-mcp/server/FantasticalMCP.app/Contents/MacOS/FantasticalMCP"
```

The server answered with its identity:

```json
{"id":1,"jsonrpc":"2.0","result":{"capabilities":{"tools":{"listChanged":true}},"protocolVersion":"2025-06-18","serverInfo":{"name":"Fantastical","version":"1.1.2"}}}
```

That is a working server, and I installed nothing new to get it.

## Point Claude Code at it

Claude Code reads project MCP servers from `.mcp.json` at the repo root. The
entry is the manifest's `mcp_config` with `${__dirname}` expanded to the
connector folder:

```json
"fantastical": {
  "command": "/Users/<you>/Library/Application Support/Claude/Claude Extensions/ant.dir.gh.flexibits.fantastical-mcp/server/FantasticalMCP.app/Contents/MacOS/FantasticalMCP",
  "args": []
}
```

Restart Claude Code and approve the new server. The session then has 5
calendar tools: `queryCalendars`, `queryCalendarItems`, `createCalendarItem`,
`modifyCalendarItem`, and `deleteCalendarItem`.

## Caveats

- The path only resolves on machines where the connector is installed. My
  `.mcp.json` lives in a repo that syncs between Macs. On a Mac without the
  connector, the server shows as failed and nothing else breaks.
- Updates flow through. The connector folder name has no version in it, so
  Desktop updates the binary in place and Claude Code picks it up.
- The Fantastical server is macOS-only and talks to the local Fantastical
  app. Other connectors vary — check the `platforms` field in the manifest.

## The general recipe

1. List `~/Library/Application Support/Claude/Claude Extensions/`.
2. Open the connector's `manifest.json` and find `server.mcp_config`.
3. Expand `${__dirname}` to the connector's absolute path.
4. Copy the command, args, and env into `.mcp.json`.
5. Test with the `initialize` one-liner before you restart.

Node-based connectors work the same way — the command is `node` plus a
script path instead of a binary. One install, both clients, and the vendor's
updates come along for free.
