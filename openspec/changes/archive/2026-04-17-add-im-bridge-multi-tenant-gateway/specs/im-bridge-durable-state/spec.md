## ADDED Requirements

### Requirement: Durable state schema SHALL include session, intent, and reply-binding tables

The Bridge's durable state store SHALL provision three additional tables alongside the existing dedupe and rate-limit tables: `session_history`, `intent_cache`, and `reply_target_binding`. These tables MUST live in the same SQLite database file referenced by `IM_BRIDGE_STATE_DIR/state.db`, MUST use `tenant_id` as a primary-key prefix on every row, and MUST be created or migrated idempotently on bridge startup so operators do not have to manage schema versions by hand.

#### Scenario: Fresh state.db is provisioned with all new tables
- **WHEN** the bridge starts against an empty `state.db`
- **THEN** it creates `session_history`, `intent_cache`, and `reply_target_binding` tables with `tenant_id` as the primary-key prefix
- **AND** subsequent startups are no-ops against those tables

#### Scenario: Existing state.db is upgraded in place
- **WHEN** the bridge starts against a `state.db` created by a prior version that only had dedupe and rate tables
- **THEN** the missing session tables are created without touching the existing data
- **AND** startup logs the migration outcome with a clear "added N tables" summary

#### Scenario: Migration failure keeps the bridge from accepting traffic
- **WHEN** the schema migration fails (for example due to a corrupt SQLite file)
- **THEN** the bridge exits non-zero with an actionable error and does not enter the ready state
- **AND** no session write path is exposed to handlers

### Requirement: Session persistence workload SHALL share the state store's operational rules

Writes and reads against the session tables SHALL use the same SQLite connection pool, busy-timeout, and background cleanup scheduler as dedupe and rate-limit tables. Cleanup of session tables MUST follow the same general rule: bounded periodic sweeps, independent per-table TTL configuration, and explicit operator signaling when a table grows beyond the configured soft cap. Session cleanup MUST NOT block dedupe or rate-limit writes for more than the shared busy-timeout, and the cleanup worker MUST surface per-table timing through the existing state-store diagnostics.

#### Scenario: Cleanup sweeps sessions alongside dedupe without starving writes
- **WHEN** the cleanup worker runs a periodic sweep that touches all five tables
- **THEN** session cleanup completes within the shared busy-timeout budget and yields back to the pool
- **AND** concurrent dedupe writes remain unblocked beyond `busy_timeout`

#### Scenario: Soft cap on a session table produces the same warning surface
- **WHEN** `session_history` exceeds its configured soft cap
- **THEN** the cleanup worker emits the same warning path used by dedupe growth warnings
- **AND** the operator diagnostics surface the session-table warning alongside any existing dedupe or rate warnings

#### Scenario: Explicit in-memory fallback disables session persistence too
- **WHEN** `IM_DISABLE_DURABLE_STATE=true`
- **THEN** session history, intent cache, and reply-target bindings also fall back to in-memory storage with the same warning signaling as the existing dedupe fallback
- **AND** the warning names every subsystem that is running in memory, not just dedupe
