## 1. Persistence Foundation And Bootstrap

- [x] 1.1 Add `gorm.io/gorm` and `gorm.io/driver/postgres` to `src-go/go.mod`/`go.sum`, and introduce canonical Gorm bootstrap helpers under `src-go/pkg/database`.
- [x] 1.2 Preserve `golang-migrate` as the schema authority, update startup sequencing in `src-go/cmd/server/main.go`, and ensure migrations still run before repository-backed traffic uses PostgreSQL.
- [x] 1.3 Add shared persistence utilities for transaction execution, repository rebinding, and stable error normalization so migrated repositories do not leak ORM-specific behavior.

## 2. Persistence Models And Schema Coverage

- [x] 2.1 Introduce repository-owned Gorm persistence structs and mapping helpers for the current Postgres-backed domains without coupling ORM tags directly into `src-go/internal/model`.
- [x] 2.2 Implement shared field adapters for UUIDs, nullable foreign keys, JSON/JSONB payloads, arrays, timestamps, pagination, and joined projection data used by the current repositories.
- [x] 2.3 Add any missing SQL migrations needed for plugin persistence tables and other schema gaps that block full replacement of in-memory or raw-SQL persistence seams.

## 3. Migrate Lower-Complexity PostgreSQL Repositories

- [x] 3.1 Migrate the `user`, `project`, `member`, and `sprint` repositories to the shared Gorm persistence seam while preserving their current service-facing behavior.
- [x] 3.2 Migrate the `notification`, `workflow`, `false_positive`, and `review_aggregation` repositories to the shared Gorm persistence seam.
- [x] 3.3 Update any affected handlers, services, and repository tests so the lower-complexity domains no longer depend on `DBTX`, `pgxpool`, or raw SQL expectations.

## 4. Migrate Complex And Transactional PostgreSQL Repositories

- [x] 4.1 Migrate `task_repo.go`, including list/detail queries, planning fields, joined progress snapshot reads, and child-task creation with atomic transaction behavior.
- [x] 4.2 Migrate `task_progress_repo.go`, `agent_run_repo.go`, and `review_repo.go`, preserving current filters, summary queries, and multi-record write semantics.
- [x] 4.3 Migrate `agent_team_repo.go` and `agent_memory_repo.go`, and align any remaining repository callers that still assume `pgx`-specific transaction or query behavior.

## 5. Replace In-Memory Plugin Persistence With PostgreSQL

- [x] 5.1 Replace `src-go/internal/repository/plugin_registry.go` with a PostgreSQL-backed implementation on the shared Gorm seam.
- [x] 5.2 Persist plugin registry records, runtime instance state, and any required audit/event data through transactional repository operations instead of process memory.
- [x] 5.3 Update plugin services, handlers, and server wiring so plugin-management flows rely on durable Postgres-backed state after restart.

## 6. Verification, Test Migration, And Regression Coverage

- [x] 6.1 Replace brittle `pgxmock`-style SQL-string assertions with repository contract tests, service fakes, and targeted Postgres-backed integration tests for the migrated domains.
- [x] 6.2 Add or update high-risk regression coverage for task transactions, complex field round-trips, plugin persistence durability, and repository error normalization.
- [x] 6.3 Run focused `go test` coverage for migrated packages, then run broader `go test ./...` in `src-go` once the full persistence cutover is complete.

## 7. Cleanup, Documentation, And Adoption Guardrails

- [x] 7.1 Remove obsolete `DBTX`, `pgx.Tx`, and raw repository plumbing that is no longer needed after the Gorm cutover.
- [x] 7.2 Refresh backend documentation to describe the canonical Gorm persistence path, migration ownership rules, and the approved escape-hatch policy for rare raw SQL cases.
- [x] 7.3 Audit remaining Go code for direct Postgres access outside the shared persistence seam and close any stragglers so future backend work builds on the new foundation by default.
