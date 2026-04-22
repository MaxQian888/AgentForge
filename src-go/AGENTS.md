# src-go/AGENTS.md

## Go Backend Guidelines

### Structure & Organization

- Layered architecture: `cmd/` → `internal/handler` → `internal/service` → `internal/repository` → `internal/model`
- Reuse existing packages before introducing new top-level folders
- Plugin SDK lives in `plugin-sdk-go/`; built-in plugins in `plugins/`

### Commands

```bash
go test ./...                    # Run all tests
go test -race ./...              # Run with race detector
go vet ./...                     # Static analysis
golangci-lint run                # Full lint (if installed)
go run ./cmd/server              # Run server
go build ./cmd/server            # Build binary
```

### Code Style

- Package names: lowercase, no underscores
- Exported identifiers: PascalCase; unexported: camelCase
- Error handling: wrap with context using `fmt.Errorf("...: %w", err)`
- DTOs in `internal/model`; request/response structs in handler or a dedicated `dto` package
- Prefer interfaces for repository and service boundaries

### Testing

- Co-locate tests next to source: `foo.go` + `foo_test.go`
- Use `testify/assert` or `testify/require` for assertions
- Mock external dependencies; test services with mocked repositories
- Race detector mandatory for concurrency-sensitive packages (`internal/eventbus`, `internal/ws`, `internal/queue`)

### Database & Migrations

- Migrations in `migrations/` using sequential numeric prefixes
- Never modify existing migration files after they have been applied in any environment
- Backfill data via new migrations, not in application code at startup

### Auth & Security

- JWT middleware fails closed when Redis is unavailable
- Refresh-token revocation is permanent; do not silently bypass blacklist checks
- Secrets via environment variables; no hardcoded keys in source

### Commit Style

- Prefer Conventional Commits: `feat:`, `fix:`, `refactor:`, `chore:`, `ci:`, `docs:`
- Example: `fix(eventbus): eliminate close-while-send data race in Subscribe`
