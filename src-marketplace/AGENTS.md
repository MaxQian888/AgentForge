# src-marketplace/AGENTS.md

## Marketplace Service Guidelines

### Structure & Organization

- Echo framework, layered: `cmd/server` → `internal/handler` → `internal/service` → `internal/repository` → `internal/model`
- Standalone microservice; do not import main backend packages directly

### Commands

```bash
go test ./...
go run ./cmd/server    # port 7781 by default
go build ./cmd/server
```

### Code Style

- Follow Go conventions: lowercase packages, PascalCase exports, camelCase internals
- Error wrapping: `fmt.Errorf("...: %w", err)`
- HTTP handlers: bind → validate → call service → return standardized response

### Testing

- Co-located `*_test.go` files
- Mock repository layer for service tests
- Handler tests should cover status codes and error paths

### Domain Rules

- Default port `7781`; never reuse IM Bridge port
- Marketplace items: plugins, skills, roles
- Installs are bridged through main backend; this service does not directly touch consumer storage
- Reviews are per-version, not per-item

### Migrations

- Sequential numeric prefixes in `migrations/`
- Immutable after deployment to any environment

### Commit Style

- Conventional Commits: `feat:`, `fix:`, `refactor:`, `docs:`
