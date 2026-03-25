# Go Persistence Guide

## Purpose

This guide defines the canonical PostgreSQL persistence path for `src-go` after the Gorm cutover.
It aligns the Go backend with the PRD direction of `PostgreSQL + GORM/sqlx` while keeping SQL migrations as the only schema authority.

## Default Path

For new backend persistence work in `src-go`, use this stack by default:

1. Open PostgreSQL through `src-go/pkg/database/postgres.go`.
2. Run schema changes through `src-go/pkg/database/migrate.go` and versioned SQL under `src-go/migrations`.
3. Pass the shared `*gorm.DB` from `src-go/cmd/server/main.go` into repository constructors.
4. Keep ORM table mappings inside repository-owned persistence structs in `src-go/internal/repository/persistence_models.go`.
5. Keep service and handler layers dependent on repository contracts, not on Gorm directly.

## Schema Ownership

`golang-migrate` remains the only schema authority.

- Additive and rollback-aware schema changes go in `src-go/migrations/*.sql`.
- Runtime code must not call `AutoMigrate`.
- Gorm models must follow the migrated schema, not invent it.
- If a feature needs new tables, indexes, or constraints, land the SQL migration first and then map it in repository persistence structs.

## Repository Rules

- Repositories may use Gorm query builders or controlled `Raw(...)` / `Exec(...)` calls when that is the clearest way to preserve behavior.
- Domain and API structs in `src-go/internal/model` must stay free of Gorm tags.
- Multi-record writes should use `src-go/pkg/database/tx.go` plus repository rebinding helpers instead of ad-hoc transaction code.
- Durable control-plane state belongs in PostgreSQL, not process-local memory, when the state must survive restart.

## Escape Hatch Policy

Raw SQL is allowed only as a narrow escape hatch.

Use raw SQL when at least one of these is true:

- The query is materially clearer as SQL than as chained Gorm calls.
- The query depends on PostgreSQL-specific operators, joins, or projections that would become harder to maintain if forced through the ORM.
- The repository needs parity with an existing query shape and the Gorm version would add behavioral risk.

When using the escape hatch:

- Keep the raw SQL inside the repository layer.
- Reuse shared field adapters and error normalization helpers from `src-go/internal/repository`.
- Cover the behavior with repository contract tests or focused integration-style tests.
- Do not reintroduce `DBTX`, `pgxpool`, or service-layer SQL access as a shortcut.

## Verification Expectations

For persistence changes:

- Run the narrowest relevant `go test` target first.
- Re-run affected repository, service, handler, and server packages after the change lands.
- Run a broader `go test ./...` sweep in `src-go` when the persistence cutover for the touched seam is complete.

## Current Foundation

The current Gorm-backed foundation now covers the existing PostgreSQL repositories, including plugin control-plane persistence, scheduler persistence, and agent-pool queue persistence.
Future backend work should extend this shared seam instead of creating parallel Postgres access paths.
