## ADDED Requirements

### Requirement: OpenCode continuity snapshots preserve server-backed control bindings after pause
The TypeScript bridge SHALL persist enough OpenCode continuity metadata to service server-backed control operations after a task is paused, including the upstream session identity and any related control-plane state needed for messages, diffs, commands, shell execution, and revert operations. Those controls MUST continue to target the same upstream session lineage rather than requiring a fresh execute or implicit resume.

#### Scenario: Pause captures session binding for later control operations
- **WHEN** the Bridge pauses an active `opencode` task
- **THEN** the saved continuity snapshot retains the upstream OpenCode session identity required for messages, diff, command, shell, revert, and unrevert controls
- **THEN** later control routes can act on that same upstream session even though the active runtime has been released

#### Scenario: Control request uses the saved OpenCode session lineage
- **WHEN** a caller targets a paused `opencode` task with a canonical server-backed control operation
- **THEN** the Bridge uses the persisted continuity snapshot to resolve the original upstream session lineage
- **THEN** it SHALL NOT start a fresh OpenCode session or replay the original execute payload merely to satisfy that control request

### Requirement: OpenCode auth and permission interactions are task- or session-bound bridge state
The TypeScript bridge SHALL persist Bridge-owned identifiers for OpenCode auth and permission interactions separately from Claude callback state, and each pending interaction MUST remain bound to the originating provider or upstream session context until it resolves or expires.

#### Scenario: Pending OpenCode permission interaction stays bound to its session context
- **WHEN** the Bridge emits a permission request for an OpenCode session
- **THEN** the Bridge stores the request with the originating upstream session identity and permission identifier
- **THEN** a later permission response resolves against that same OpenCode session context

#### Scenario: Pending OpenCode auth interaction expires without leaking a false ready state
- **WHEN** a provider-auth interaction is started but never completed
- **THEN** the Bridge expires the pending interaction after its timeout window
- **THEN** the runtime catalog continues to report the provider as unauthenticated or auth-required rather than falsely promoting it to ready
