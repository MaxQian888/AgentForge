# src-go/CLAUDE.md

Go orchestrator backend for AgentForge.

## Overview

Echo-framework HTTP server with layered architecture: handler → service → repository → model.

## Quick Commands

```bash
# Run (requires PostgreSQL + Redis)
go run ./cmd/server

# Build
go build ./cmd/server

# Test
go test ./...

# Dev via pnpm
pnpm build:backend:dev      # current platform only
pnpm build:backend          # cross-platform
```

## Structure

| Package | Responsibility |
|---------|---------------|
| `cmd/server` | Entry point |
| `internal/handler` | HTTP handlers |
| `internal/service` | Business logic |
| `internal/repository` | Data access |
| `internal/model` | Domain models |
| `internal/middleware` | Auth, CORS, rate limiting |
| `internal/ws` | WebSocket hub |
| `internal/plugin` | Plugin control plane (WASM, MCP, catalog, trust gates) |
| `internal/scheduler` | Job scheduling |
| `internal/role` | Role management |
| `internal/worktree` | Git worktree management |
| `internal/cost` | Cost tracking |
| `internal/memory` | Project memory |
| `internal/pool` | Agent pool management |
| `internal/trigger` | Automation trigger engine |
| `internal/automation` | Declarative automation rules |
| `internal/vcs` | VCS provider registry (GitHub, GitLab, Gitea) |
| `internal/knowledge` | Knowledge asset management |
| `internal/secrets` | Per-project secret storage |
| `internal/employee` | Agent identity management |
| `internal/adsplatform` | Ads-platform provider registry |
| `internal/queue` | Agent work queue |
| `internal/skills` | Governed skill catalog |
| `internal/document` | Document management |
| `internal/eventbus` | Internal pub/sub bus |
| `internal/instruction` | Agent instruction/prompt management |
| `internal/storage` | Blob storage abstraction |
| `internal/imcards` | IM rich-card formatters |
| `internal/imconfig` | IM provider configuration |
| `internal/version` | Service version metadata |
| `internal/integration` | External integration trigger tests |
| `internal/workflow` | Workflow engine (DAG evaluation) |
| `internal/bridge` | Bridge integration |
| `internal/log` | Structured logging |

## Environment

Create `.env` for local overrides:
- `POSTGRES_URL`
- `REDIS_URL`
- `JWT_SECRET` (min 32 chars in production)
- `JWT_ACCESS_TTL=15m`
- `JWT_REFRESH_TTL=168h`
- `ALLOW_ORIGINS`

## Auth Notes

- Refresh/logout revocation and blacklist-backed checks fail closed when Redis is unavailable.
- Marketplace install/consumption bridged via `/api/v1/marketplace/install` and `/api/v1/marketplace/consumption`.
