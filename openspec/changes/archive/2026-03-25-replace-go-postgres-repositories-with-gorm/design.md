## Context

`src-go` today is built around `pgxpool`, `pgx.Tx`, and a shared `DBTX` interface declared in the repository layer. Every PostgreSQL-backed repository then hand-writes SQL for CRUD, filtering, pagination, joins, and some transaction handling. Examples already visible in the current codebase include:

- `src-go/pkg/database/postgres.go` returns `*pgxpool.Pool` as the canonical database handle.
- `src-go/internal/repository/user_repo.go` defines the shared `DBTX` contract used by most Postgres repositories.
- `src-go/internal/repository/task_repo.go` contains large hand-written query strings, manual scan helpers, and a custom `pgx.Tx` transaction path for `CreateChildren(...)`.
- `src-go/internal/repository/plugin_registry.go` is still in-memory only, which conflicts with the PRD and plugin-system design documents that expect plugin state to be registered, reconciled, and queryable from the database.
- `src-go/migrations/*` and `src-go/pkg/database/migrate.go` already establish SQL migrations as the existing schema authority.

At the same time, `docs/PRD.md` explicitly recommends `PostgreSQL + GORM/sqlx` for the Go stack, and `docs/part/PLUGIN_SYSTEM_DESIGN.md` expects plugin install/uninstall, instance state, and audit records to be database-backed. Upcoming backend work on plugin control plane, task workspace, review, and session continuity will keep increasing the cost of the current raw-SQL approach if we do not first standardize the persistence layer.

## Goals / Non-Goals

**Goals:**
- Establish one canonical Gorm-backed PostgreSQL access layer for `src-go`.
- Replace all current Postgres-backed repository implementations with Gorm-backed repositories while preserving existing service and handler contracts.
- Keep migrations authoritative through versioned SQL files and `golang-migrate`.
- Provide a shared approach for transactions, error mapping, pagination, joins/preloads, and JSON/array/nullable field mapping.
- Remove in-memory-only authoritative persistence seams, especially for plugin registry state.
- Leave the codebase with a migration plan that can be implemented domain-by-domain without losing behavioral parity.

**Non-Goals:**
- Replacing Redis/cache access or changing auth-token cache behavior.
- Redesigning frontend APIs, TS bridge contracts, or WebSocket payloads unless a persistence contract requires a narrow alignment fix.
- Introducing `AutoMigrate`, runtime schema drift, or a schema-first rewrite that ignores the current SQL migration history.
- Reworking unrelated business logic purely for style reasons.
- Replacing every raw SQL statement in the entire repository on day one if a tiny number must temporarily remain as controlled escape hatches for parity or performance.

## Decisions

### 1. `src-go/pkg/database` becomes the single entrypoint for shared Gorm setup, while migrations remain a separate startup step

The Go server will open PostgreSQL through a shared `*gorm.DB` configured for the Postgres dialect. `main.go` will continue to run SQL migrations before wiring repositories, but repository constructors will stop depending on `*pgxpool.Pool` / `DBTX` and instead depend on the shared Gorm handle or a transaction-bound clone of it.

This keeps database bootstrap centralized and prevents every service from inventing its own way to open sessions, configure logging, or start transactions.

Alternatives considered:
- Keep `pgxpool` and add more helper wrappers: rejected because it preserves the current hand-written SQL sprawl and does not move the implementation toward the PRD target.
- Switch to `sqlx`: viable in theory, but still leaves most query construction and mapping manual. The requested direction is to introduce Gorm and perform a full replacement of the current raw-SQL repository path.

### 2. Gorm persistence structs stay separate from the existing domain/API structs

The current `internal/model` package carries domain objects that are already consumed by handlers, services, and JSON responses. This design will not turn those structs into direct Gorm models. Instead, repository-owned persistence structs will carry Gorm table/column metadata, and repositories will map between persistence structs and the existing domain models.

That separation keeps ORM concerns from leaking into API payloads, reduces the risk of accidental schema coupling, and makes it easier to preserve existing JSON field names and validation behavior.

Alternatives considered:
- Add Gorm tags directly to `internal/model`: rejected because it tightly couples HTTP/domain representation to database shape and makes future schema changes harder to isolate.
- Introduce a fully generic repository layer: rejected because the current domains have distinct filters, joins, and transaction semantics that still need explicit modeling.

### 3. Repository public contracts stay stable first; the migration happens behind them

Where possible, repository constructor names and service-facing method signatures should remain stable so the migration stays focused on persistence rather than forcing broad handler/service rewrites. The main shape change is the repository dependency type: repository internals move from `DBTX` + raw SQL to Gorm sessions, but callers should continue to ask for `Create`, `GetByID`, `List`, `Update`, and other domain methods instead of learning Gorm directly.

This minimizes the blast radius and lets us convert one domain slice at a time while keeping business logic readable.

Alternatives considered:
- Rewrite services to issue Gorm queries directly: rejected because it would collapse repository boundaries and spread ORM details across the codebase.
- Replace the repository layer with one mega store object: rejected because it would increase coupling across unrelated domains.

### 4. Transactions are standardized around a shared `WithTx` / rebind pattern

Current transaction handling is inconsistent; `task_repo.go` manually detects a `Begin(context.Context)` capability and manages `pgx.Tx` itself. The Gorm migration will standardize multi-record writes around a shared transaction helper in the database layer, and repositories that need atomic multi-entity writes will support rebinding onto a transaction-scoped `*gorm.DB`.

This is especially important for:
- child-task creation and decomposition persistence,
- task / sprint / project cost rollups,
- plugin registry + instance + audit writes,
- future review aggregation flows.

Alternatives considered:
- Keep ad-hoc transaction code in each repository: rejected because it recreates the same inconsistency the migration is trying to remove.
- Hide all transactions inside services without repository support: rejected because repositories still need a clean way to participate in the same DB transaction.

### 5. SQL migrations remain the only schema authority, including new plugin tables

The repository already embeds versioned SQL migrations, and `docs/part/PLUGIN_SYSTEM_DESIGN.md` describes plugin-related tables that are not yet present on disk. This change will keep schema evolution in SQL migrations, including any new plugin registry / plugin instance / plugin event tables needed to eliminate in-memory persistence seams.

Gorm models therefore follow the migrated schema; runtime startup MUST NOT call `AutoMigrate`.

Alternatives considered:
- Mix `golang-migrate` with Gorm `AutoMigrate`: rejected because dual schema authorities create drift and make rollback/audit much harder.
- Convert all migration management to Gorm: rejected because the repo already has versioned SQL history and needs explicit control over Postgres-specific types, indexes, triggers, and future partitions.

### 6. Testing moves from SQL-string expectation tests toward repository contract tests and Postgres-backed integration checks

A large portion of the current repository test strategy is tied to `pgxmock` and exact SQL strings. That is not a good long-term fit for a Gorm-backed repository layer. The migration will keep service-level logic tests at the interface boundary, but repository verification will shift toward behavior-oriented contract tests and targeted Postgres integration tests for the complex domains that use arrays, JSONB, joins, or transactions.

Complex cases that should keep integration-style coverage include tasks, task progress snapshots, agent runs, workflows, reviews, and plugin persistence. Simpler service orchestration tests should use fakes or narrow repository interfaces rather than ORM internals.

Alternatives considered:
- Preserve `pgxmock` assertions by forcing Gorm SQL output to match old raw SQL: rejected because it couples tests to an implementation detail we are intentionally replacing.
- Rely only on unit tests with no database coverage: rejected because Postgres-specific field mapping is central to this migration.

## Risks / Trade-offs

- [Repository parity regressions during migration] -> Mitigation: migrate domain-by-domain, keep public repository contracts stable, and run focused regression tests after each slice.
- [Gorm field mapping drift for JSONB, arrays, UUIDs, and nullable joins] -> Mitigation: introduce explicit persistence structs, add round-trip tests for complex fields, and keep Postgres-backed integration coverage for high-risk domains.
- [Active plugin-control-plane work may overlap with plugin persistence changes] -> Mitigation: treat this change as the persistence foundation and make plugin table/repository work land on the shared Gorm seam instead of inventing a separate storage path.
- [Transaction behavior could subtly change when moving off `pgx.Tx`] -> Mitigation: standardize a shared transaction helper early and verify all-or-nothing behavior on task and plugin flows before broad rollout.
- [Migration effort is large because almost every Go repository is affected] -> Mitigation: sequence the work into bootstrap, simple CRUD domains, complex transactional domains, then plugin persistence and cleanup; do not try to flip everything in one opaque patch.
- [A few query paths may still need raw SQL for parity or performance] -> Mitigation: allow narrow, documented escape hatches inside Gorm-backed repositories instead of treating “zero raw SQL” as more important than correctness.

## Migration Plan

1. Add Gorm dependencies and shared database bootstrap/helpers under `src-go/pkg/database`, while preserving the existing migration runner.
2. Introduce repository-owned persistence structs and mapping helpers for current Postgres-backed domains.
3. Migrate lower-complexity repositories first (`user`, `project`, `member`, `sprint`, `notification`, `workflow`, `false_positive`, `review_aggregation`) to validate the shared patterns.
4. Migrate higher-complexity repositories next (`task`, `task_progress`, `agent_run`, `review`, `agent_team`, `agent_memory`) with transaction and preload coverage.
5. Replace the in-memory plugin registry seam with PostgreSQL-backed persistence, adding any missing plugin tables through SQL migrations.
6. Update service wiring, tests, docs, and any remaining callers that still assume `pgx`-specific behavior.
7. Remove obsolete `DBTX`/`pgx` repository plumbing once all Postgres-backed domains are on the shared Gorm seam.

Rollback strategy:
- If migration work stalls mid-way, revert the server wiring and repository constructors to the last fully working `pgx` path before merging incomplete domain slices.
- New SQL migrations should be additive and reversible where practical, so rollback can disable new persistence consumers without leaving the server unable to start.
- If plugin persistence cannot land safely in the same implementation window, keep the migration tables but defer the plugin repository cutover behind the still-working repository seam rather than shipping two authoritative stores.

## Open Questions

- Should this change standardize on pure Gorm for all Postgres repositories, or explicitly reserve a small `gorm + raw SQL` pattern for the handful of queries whose parity is hard to express cleanly through the ORM?
- Do we want a shared repository factory / unit-of-work helper in this change, or is a lightweight `WithTx` plus repository rebinding approach enough for the first full migration?
- Should plugin persistence be implemented inside this same change end-to-end, or treated as the first consumer change immediately after the shared Gorm foundation lands?
