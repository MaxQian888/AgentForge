# AgentForge Go Orchestrator

The core backend orchestration service for AgentForge, built with Go + Echo. Provides HTTP APIs, WebSocket real-time communication, job scheduling, plugin control plane, VCS integration, knowledge base management, and more.

## Tech Stack

- **Language**: Go 1.25+
- **Web Framework**: Echo v4
- **Database**: PostgreSQL (primary) + Redis (cache / token revocation)
- **ORM**: GORM
- **Messaging**: Internal EventBus (Pub/Sub)
- **WebSocket**: gorilla/websocket
- **Plugin Runtime**: WebAssembly (wazero)

## Directory Structure

```
cmd/
  server/                     # Main service entry point
  backfill-trigger-source/    # Trigger source backfill tool
  email-adapter/              # Email adapter
  generic-webhook-adapter/    # Generic webhook adapter
  github-actions-adapter/     # GitHub Actions adapter
  migrate-once/               # One-off migration scripts
  plugin-debugger/            # Plugin debugger
  review-escalation-flow/     # Review escalation flow
  standard-dev-flow/          # Standard development flow
  task-delivery-flow/         # Task delivery flow
  ...
internal/
  handler/      # HTTP handlers (REST API)
  service/      # Business logic layer
  repository/   # Data access layer
  model/        # Domain models
  middleware/   # Auth, CORS, rate limiting, etc.
  ws/           # WebSocket Hub
  plugin/       # Plugin control plane
  scheduler/    # Cron job scheduling
  role/         # Role management
  worktree/     # Git worktree management
  cost/         # Cost tracking
  memory/       # Project memory
  pool/         # Agent pool management
  trigger/      # Automation trigger engine
  automation/   # Declarative automation rules
  vcs/          # VCS provider registry (GitHub/GitLab/Gitea)
  knowledge/    # Knowledge asset management & vector search
  secrets/      # Per-project secret storage
  employee/     # Agent identity (employee) management
  adsplatform/  # Ads platform integration (Qianchuan)
  queue/        # Agent work queue & priority controls
  skills/       # Governed skill catalog
  document/     # Document management
  eventbus/     # Internal event bus
  instruction/  # Agent instruction / prompt management
  storage/      # Blob storage abstraction
  imcards/      # IM rich-card formatters
  integration/  # External integration trigger flow tests
  version/      # Service version metadata
pkg/            # Public packages
migrations/     # Database migration scripts
plugins/        # Built-in plugin examples
plugin-sdk-go/  # Go plugin SDK
```

## Quick Start

```bash
# Run directly (requires PostgreSQL + Redis)
go run ./cmd/server

# Build for current platform
go build ./cmd/server

# Run tests
go test ./...

# Database migrations (using golang-migrate)
migrate -path migrations -database "$POSTGRES_URL" up
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `POSTGRES_URL` | PostgreSQL connection string | - |
| `REDIS_URL` | Redis connection string | - |
| `JWT_SECRET` | JWT signing secret (required in production) | - |
| `JWT_ACCESS_TTL` | Access token TTL | `15m` |
| `JWT_REFRESH_TTL` | Refresh token TTL | `168h` |
| `ALLOW_ORIGINS` | CORS allowed origins | `http://localhost:3000` |
| `SERVER_PORT` | HTTP service port | `7777` |

> For the full configuration reference, see `src-go/.env.example`.

## Key Modules

### Authentication & Authorization
- JWT dual-token mechanism (Access + Refresh)
- Refresh token blacklist (Redis-backed)
- Project-level RBAC: owner / admin / editor / viewer

### Plugin System
- Supports WebAssembly runtime plugins
- Plugin SDK located in `plugin-sdk-go/`
- Control plane provides install, uninstall, restart, and status queries

### Trigger Engine (`trigger`)
- Supports CRON, Webhook, event subscription, and more trigger sources
- Idempotency guarantees and scheduling routing
- Dry-run mode support

### Knowledge Base (`knowledge`)
- Chunked ingestion
- Vector search
- Live-artifact materialization

### VCS Integration (`vcs`)
- Supports GitHub, GitLab, and Gitea
- Webhook routing and event handling
- Per-project VCS provider connection management
