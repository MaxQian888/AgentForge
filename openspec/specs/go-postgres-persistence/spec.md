# go-postgres-persistence Specification

## Purpose
Define the canonical Gorm-backed PostgreSQL persistence contract for AgentForge Go services and repositories so durable state, transactions, schema ownership, and error semantics stay consistent across backend domains.
## Requirements
### Requirement: Go backend uses a shared Gorm-backed PostgreSQL persistence handle
The system SHALL establish a shared Gorm-backed PostgreSQL handle for AgentForge Go services and SHALL bind Postgres-backed repositories to that shared persistence handle instead of `pgx`-specific repository dependencies.

#### Scenario: Server boots with PostgreSQL configured
- **WHEN** the Go server starts with a valid PostgreSQL connection string
- **THEN** it opens the shared PostgreSQL persistence handle through the canonical database package
- **AND** Postgres-backed repositories use that shared handle for their data access work

#### Scenario: PostgreSQL is unavailable at startup
- **WHEN** the Go server cannot establish the shared PostgreSQL persistence handle
- **THEN** Postgres-backed repository consumers receive the existing database-unavailable behavior instead of a partially initialized repository state

### Requirement: Postgres-backed domain state remains durable across backend restarts
The system SHALL store authoritative Postgres-backed domain state in PostgreSQL rather than in-memory repository implementations so that persisted records survive backend restarts and can be reloaded by the next process.

#### Scenario: Existing domain record survives a server restart
- **WHEN** a project, task, review, workflow, or other Postgres-backed record is created successfully
- **AND** the Go backend process restarts
- **THEN** the restarted backend can load the same record from PostgreSQL without reconstructing it from in-memory state

#### Scenario: Plugin registry state survives a server restart
- **WHEN** the platform registers or updates a plugin record that is meant to be authoritative for operators
- **AND** the Go backend process restarts
- **THEN** the plugin record and its persisted runtime metadata remain queryable from the authoritative Postgres-backed store

### Requirement: Multi-entity persistence operations remain atomic
The system SHALL execute persistence operations that span multiple Postgres records within one transaction so that dependent writes either all commit or all roll back.

#### Scenario: Child task creation succeeds atomically
- **WHEN** the backend creates multiple child tasks from one decomposition request
- **THEN** all child task records are committed together if the operation succeeds
- **AND** no partial child-task set is persisted if any write in the transaction fails

#### Scenario: Plugin state and audit writes fail together
- **WHEN** one plugin-management operation needs to persist registry state together with related instance or audit records
- **THEN** the backend commits all of those records together on success
- **AND** it does not leave a partially updated authoritative plugin state when one participating write fails

### Requirement: Schema lifecycle remains migration-owned
The system SHALL continue to manage PostgreSQL schema changes through versioned SQL migrations and MUST NOT mutate runtime schema through Gorm auto-migration during normal server startup.

#### Scenario: Server startup applies pending migrations before repository use
- **WHEN** the Go server starts and PostgreSQL is available
- **THEN** it runs the repository's pending SQL migrations before serving repository-backed requests
- **AND** the shared Gorm-backed persistence handle uses the migrated schema rather than attempting to create or alter tables itself

#### Scenario: No pending migration does not trigger runtime schema mutation
- **WHEN** the database schema is already up to date
- **THEN** normal server startup completes without issuing Gorm auto-migration changes

### Requirement: Gorm persistence preserves current Postgres field semantics
The system SHALL preserve the current Postgres-backed semantics for UUID identifiers, nullable foreign keys, JSON/JSONB payloads, array fields, timestamps, filters, and pagination when repository implementations move to the shared Gorm seam.

#### Scenario: Task detail preserves current structured fields
- **WHEN** a task record is loaded through the migrated persistence layer
- **THEN** the returned task data preserves labels, blocked-by links, planning timestamps, agent runtime metadata, and joined progress snapshot data with the same meaning as before the migration

#### Scenario: JSON and nullable fields round-trip without losing meaning
- **WHEN** a repository persists and reloads fields such as workflow transitions, trigger definitions, agent config, plugin permissions, or optional foreign keys
- **THEN** the migrated persistence layer preserves those JSON and nullable values without collapsing them into incorrect zero values or stringified placeholders

### Requirement: Repository-facing error semantics remain stable across the ORM migration
The system SHALL normalize persistence errors so service and handler layers continue to receive stable not-found, unavailable, and failed-write signals instead of Gorm-specific error leakage.

#### Scenario: Missing record still returns the repository not-found behavior
- **WHEN** a service asks a migrated repository for a record that does not exist
- **THEN** the repository returns the existing not-found behavior expected by its callers
- **AND** callers do not need to special-case raw Gorm errors to detect absence

#### Scenario: Write failure still surfaces as a repository failure
- **WHEN** a migrated repository cannot create, update, or delete a Postgres-backed record
- **THEN** the repository returns a stable failure signal to the caller
- **AND** service-layer error handling does not depend on ORM-specific error strings
