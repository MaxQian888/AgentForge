# im-bridge-session-persistence Specification

## Purpose
Define how IM Bridge persists NLU session history, intent-classification caches, and reply-target bindings across process restarts and deployments — all scoped per tenant — so long-running conversations, inference caches, and asynchronous delivery targets survive bridge lifecycle events without cross-tenant leakage or unbounded growth.

## Requirements

### Requirement: Session history SHALL survive bridge restart per tenant and session key

IM Bridge SHALL persist NLU session history to the shared durable state store (the same SQLite database used by `im-bridge-durable-state`) under a composite key of `(tenantId, sessionKey)`. Writes MUST capture the raw content plus `occurred_at` in milliseconds, and reads MUST return the most-recent N entries ordered by `occurred_at DESC`. The runtime MUST replace the previous in-memory `historyBySession` map with the persistent store so that restart does not reset NLU context for an active conversation.

#### Scenario: NLU history survives a restart
- **WHEN** a user in tenant `acme` asks three questions in a chat, the bridge records each into session history, and then the process restarts
- **THEN** the next NLU inference for the same `(tenantId, sessionKey)` reads all three prior entries in chronological order
- **AND** the inference uses that context without an empty cold-start window

#### Scenario: Session history is scoped to a tenant
- **WHEN** tenant `acme` and tenant `beta` share an overlapping `sessionKey` value (for example the same IM user id seen under both tenants)
- **THEN** each tenant's history read returns only its own entries
- **AND** no cross-tenant entries leak between the two sessions

#### Scenario: History is capped by LRU retention
- **WHEN** a session accumulates more than the configured `IM_SESSION_HISTORY_LIMIT` entries (default 100)
- **THEN** the oldest entries are pruned so the stored length stays at or below the limit
- **AND** reads continue to return the most-recent window without duplication

### Requirement: Intent cache SHALL persist inference results per tenant and text hash

The bridge SHALL persist intent-classification results under `(tenantId, textHash)` with an `intent`, a `confidence` score, and a `cached_at` timestamp. Reads MUST return a hit only when the cache entry is within the configured TTL (default 24 hours). The cache MUST NOT be shared across tenants even when two tenants would hash the same text to the same digest.

#### Scenario: Cached intent skips re-inference within TTL
- **WHEN** tenant `acme` requests inference for text `deploy prod` at T0 and again at T+10m
- **THEN** the second call returns the stored intent and confidence without invoking the underlying classifier
- **AND** the audit log records `metadata.intent_cache=hit`

#### Scenario: Expired intent entry triggers re-inference
- **WHEN** the cached entry's `cached_at` is older than `IM_INTENT_CACHE_TTL` (default 24h)
- **THEN** the next read returns a miss
- **AND** the classifier is invoked again; the new result replaces the stale entry

#### Scenario: Intent cache does not leak across tenants
- **WHEN** tenants `acme` and `beta` both classify the same text within the TTL window
- **THEN** each tenant's cache read targets only its own row
- **AND** a mutation under `acme` does not affect `beta`'s cached confidence

### Requirement: Reply target bindings SHALL persist until the owning entity expires

The bridge SHALL persist `ReplyTarget` bindings under `(tenantId, bindingId)` where `bindingId` identifies the owning business entity (task id, agent run id, review id). Each binding MUST store the serialized reply target JSON, a `created_at`, and an `expires_at` aligned to the business entity's TTL (default 7 days for tasks and agent runs). Lookups by `bindingId` MUST return the stored target until either the TTL expires or the business entity is explicitly unbound.

#### Scenario: Long-running task delivers progress to the original chat after restart
- **WHEN** a `/task create` under tenant `acme` binds a reply target, then the bridge restarts before the backend emits progress
- **THEN** the bridge reads the stored binding on the next delivery and uses the preserved reply target
- **AND** the progress update arrives in the same chat and thread as the originating command

#### Scenario: Expired binding is cleaned up
- **WHEN** a binding's `expires_at` has passed by at least the sweep interval
- **THEN** the next cleanup pass removes the row
- **AND** subsequent lookups return a miss rather than a stale target

#### Scenario: Explicit unbind removes the row immediately
- **WHEN** the engine completes the owning entity and calls `ReplyBinding.Delete(tenantId, bindingId)`
- **THEN** subsequent lookups for that `bindingId` miss
- **AND** the deletion is reflected across any replicas sharing the state volume

### Requirement: Session store writes SHALL remain bounded and operator-visible

The session store SHALL expose operational counters for approximate row count, oldest entry age, and last cleanup run per table, and the bridge MUST surface those counters through the existing operator status snapshot. The cleanup worker MUST run on a bounded schedule (default 5 minutes) and MUST log warnings when any table exceeds its configured soft cap so operators can detect runaway growth before it harms latency.

#### Scenario: Status snapshot exposes session store counters
- **WHEN** an operator reads `GET /api/v1/im/bridge/status`
- **THEN** the response includes per-table row counts, oldest entry age, and last cleanup timestamp for `session_history`, `intent_cache`, and `reply_target_binding`
- **AND** the fields remain present even when a table is empty

#### Scenario: Soft cap breach emits a warning
- **WHEN** `session_history` exceeds the configured soft cap (default 100000 rows)
- **THEN** the cleanup worker emits a warning log and an audit event `direction=internal action=session_store_soft_cap`
- **AND** the operator snapshot surfaces the warning until the table returns below the cap
