## MODIFIED Requirements

### Requirement: Bridge execute requests accept normalized role execution profiles from Go
The TypeScript bridge SHALL treat `role_config` in execute requests as a normalized execution profile produced by the Go role-loading pipeline rather than as a raw Role YAML document. The bridge MUST apply the projected role persona, tool allowlist, bridge-consumable tool plugin identifiers, injected knowledge context, output filters, and permission constraints without needing to read YAML files, resolve inheritance, or interpret PRD-only role metadata locally.

#### Scenario: Expanded normalized role execution profile is honored
- **WHEN** the Go orchestrator submits an execute request with a valid normalized `role_config` that includes `allowed_tools`, `tools`, `knowledge_context`, and `output_filters`
- **THEN** the bridge uses that projected configuration when composing the effective system prompt, tool or plugin selection, and output filtering behavior for the runtime
- **THEN** execution uses the projected runtime-facing values instead of silently dropping the advanced fields

#### Scenario: Bridge does not need direct YAML access
- **WHEN** the bridge receives a valid execute or resume request whose role was resolved from YAML by Go
- **THEN** the bridge executes the task without reading the roles directory or parsing the source YAML itself
- **THEN** unsupported PRD-only sections remain outside the bridge request contract

### Requirement: Resolved runtime identity remains visible through bridge status metadata
The bridge SHALL expose the resolved `runtime`, `provider`, and `model` in the status metadata it returns to Go for active or paused runs, and it SHALL also expose the execution-context identity needed for diagnostics, including the selected `role_id` when present plus any validated team context bound to that runtime. This metadata MUST remain stable across pause and resume flows so backend persistence and operator-facing summaries never need to re-infer runtime or team phase identity from legacy fallback rules.

#### Scenario: Status query returns execution identity for a team coder run
- **WHEN** the Go bridge client requests the status of an active run executing through `codex` as a team coder
- **THEN** the status payload includes the resolved `runtime`, `provider`, `model`, `role_id` when present, `team_id`, and `team_role=coder`
- **THEN** the backend can persist or render that identity without re-inferring it from unrelated run records

#### Scenario: Paused run keeps the same resolved execution identity
- **WHEN** the bridge reports status for a paused or resumable run
- **THEN** the status metadata continues to return the same runtime, provider, model, and validated role or team context that were used to start the run
- **THEN** resume or summary flows do not need to guess the execution identity from legacy fallback rules
