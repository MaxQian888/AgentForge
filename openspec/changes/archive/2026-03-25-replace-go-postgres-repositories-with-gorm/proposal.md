## Why

`src-go` currently uses `pgxpool`, a custom `DBTX` abstraction, and many hand-written SQL repositories for nearly every PostgreSQL-backed domain. That implementation no longer matches the PRD direction of `PostgreSQL + GORM/sqlx`, and it is already creating duplicated query patterns, inconsistent transaction handling, in-memory-only persistence seams such as the plugin registry, and growing maintenance cost as more backend capabilities land.

We need a single, authoritative Go persistence foundation now so upcoming backend work can build on one ORM-backed contract instead of extending the current mix of raw SQL repositories, special-case persistence rules, and partial in-memory state.

## What Changes

- Introduce a canonical Gorm-backed PostgreSQL access layer in `src-go` and make it the default persistence entrypoint for Go services and repositories.
- Replace the current raw-SQL Postgres repositories in `src-go/internal/repository` with Gorm-backed implementations for users, projects, members, sprints, tasks, task progress, agent runs, notifications, reviews, workflows, false positives, agent teams, and agent memory.
- Define shared persistence rules for context propagation, transactions, eager-loading/preloading, pagination, error mapping, and nullable/JSON/array field handling so all Postgres-backed domains behave consistently.
- Move plugin-related authoritative state off in-memory-only repository implementations and onto the shared PostgreSQL persistence seam so plugin control-plane work can rely on durable storage.
- Keep Redis/cache responsibilities separate from the Postgres migration, and keep SQL migrations under `golang-migrate` as the schema source of truth instead of introducing Gorm AutoMigrate into runtime startup.
- Update server wiring, tests, fixtures, and backend documentation so new Go code depends on the shared Gorm persistence contract rather than `pgx`-specific repository assumptions.

## Capabilities

### New Capabilities
- `go-postgres-persistence`: Define the canonical Gorm-backed PostgreSQL persistence contract for AgentForge Go services, repositories, transactions, schema authority, and durable domain storage.

### Modified Capabilities
- None.

## Impact

- Affected Go bootstrap and database code: `src-go/cmd/server/main.go`, `src-go/pkg/database/*`, `src-go/go.mod`, `src-go/go.sum`.
- Affected repository layer: `src-go/internal/repository/*.go`, especially all current Postgres-backed repositories plus the in-memory `plugin_registry.go` seam.
- Affected backend services and handlers that depend on repository contracts, transaction semantics, or persistence error mapping.
- Affected data model and verification surface: migration execution path, repository tests, integration tests, and any backend flows that currently assume `pgxpool`/`DBTX` behavior.
- New dependency surface: `gorm.io/gorm`, `gorm.io/driver/postgres`, and any small support package needed to preserve existing migration and transaction guarantees.
- Coordination impact: this change becomes the persistence foundation that plugin control-plane, task/workspace, review, and auth/session backend work should build on instead of adding more raw SQL seams.
