# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository layout

Each top-level directory is an independent project with its own toolchain, language, and conventions. Treat them as separate codebases — do not share dependencies, build systems, or assume conventions transfer between them.

| Directory | Stack | Project guide |
|-----------|-------|---------------|
| `grpc/`   | Go + gRPC | [`grpc/CLAUDE.md`](grpc/CLAUDE.md) |
| `webdev/` | Vite + React + TypeScript | [`webdev/CLAUDE.md`](webdev/CLAUDE.md) |

Before working on a project, read its CLAUDE.md. New experiments should be added as sibling directories with their own CLAUDE.md.
