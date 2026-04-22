# src-marketplace/CLAUDE.md

Standalone Go microservice for the AgentForge Marketplace.

## Overview

Publishes, discovers, installs, and reviews plugins, skills, and roles.

## Quick Commands

```bash
# Run (port 7781 by default)
go run ./cmd/server

# Test
go test ./...

# Build
go build ./cmd/server
```

## Structure

| Package | Responsibility |
|---------|---------------|
| `cmd/server` | Entry point |
| `internal/handler` | HTTP handlers (items, versions, reviews, admin) |
| `internal/service` | Business logic |
| `internal/repository` | Data access |
| `internal/model` | Domain models |
| `internal/config` | Configuration |
| `internal/i18n` | Internationalization |
| `migrations/` | Database migrations |

## Integration Notes

- Default port: `7781`. Do not reuse the IM Bridge port.
- Main Go backend bridges installs via `/api/v1/marketplace/install` and `/api/v1/marketplace/consumption`.
- Installs materialize into: plugins → plugin control plane, roles → repo-local roles store, skills → role skill catalog.
- Local side-load reuses the plugin local-install seam. Unsupported role/skill side-load flows are explicitly blocked.
