# AgentForge Marketplace

A standalone Go microservice for AgentForge, handling publishing, discovery, installation, and reviews of plugins, skills, and roles.

## Tech Stack

- **Language**: Go 1.25+
- **Web Framework**: Echo v4
- **Database**: PostgreSQL
- **ORM**: GORM
- **Migrations**: golang-migrate

## Directory Structure

```
cmd/
  server/           # Service entry point
internal/
  handler/          # HTTP handlers (items, versions, reviews, admin)
  service/          # Business logic layer
  repository/       # Data access layer
  model/            # Domain models
  config/           # Configuration management
  i18n/             # Internationalization
migrations/         # Database migration scripts
```

## Quick Start

```bash
# Run directly (default port 7781)
go run ./cmd/server

# Run tests
go test ./...

# Build
go build ./cmd/server
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `SERVER_PORT` | Service port | `7781` |
| `POSTGRES_URL` | PostgreSQL connection string | - |
| `JWT_SECRET` | JWT signing secret | - |

## Core Concepts

- **Item**: A publishable entity in the marketplace. Types include `plugin`, `skill`, and `role`.
- **Version**: Version history for each item, supporting semantic versioning.
- **Review**: User ratings and comments on items.
- **Consumption**: The main backend bridges installation and typed consumption state via `/api/v1/marketplace/install` and `/api/v1/marketplace/consumption`.

## Installation Flow

Marketplace installations materialize into existing consumer seams:

- **Plugins** → Plugin control plane
- **Roles** → Repo-local roles store
- **Skills** → Authoritative role skill catalog

Local side-load in the marketplace workspace currently reuses the plugin local-install seam. Unsupported role/skill side-load flows remain explicitly blocked rather than pretending to succeed.

## Frontend Integration

- Frontend store: `lib/stores/marketplace-store.ts`
- Frontend page: `app/(dashboard)/marketplace/page.tsx`
- Frontend components: `components/marketplace/`
