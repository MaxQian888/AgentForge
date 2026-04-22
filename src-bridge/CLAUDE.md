# src-bridge/CLAUDE.md

TypeScript/Bun agent bridge service for AgentForge.

## Overview

Hono-based service that adapts multiple agent runtimes through a unified ACP (Agent Communication Protocol) client.

## Quick Commands

```bash
# Uses Bun runtime (package manager: pnpm, lockfile present)
pnpm install

# Start dev
bun run src/server.ts

# Or via package scripts (check package.json)
```

## Structure

| Package | Responsibility |
|---------|---------------|
| `src/server.ts` | Entry point |
| `src/runtime/` | Runtime adapters (claude_code, codex, opencode, cursor, gemini, qoder, iflow) |
| `src/runtime/acp/` | ACP client integration: adapter-factory, connection-pool, multiplexed-client, capabilities, handlers (fs/terminal/permission/elicitation) |
| `src/handlers/` | Request handlers (execute, legacy adapters migrated to ACP) |
| `src/plugins/` | Plugin hosting |
| `src/mcp/` | MCP integration |
| `src/session/` | Session management |
| `src/review/` | Review pipeline |
| `src/schemas.ts` | Shared schemas |

## Notes

- Runtime adapters now use ACP as the primary integration path.
- pnpm-managed with its own lockfile.
