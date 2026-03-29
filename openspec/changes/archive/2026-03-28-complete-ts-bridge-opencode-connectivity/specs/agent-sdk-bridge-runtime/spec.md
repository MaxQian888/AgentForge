## MODIFIED Requirements

### Requirement: Bridge enforces cancellation and preserves continuity state
The bridge SHALL abort active execution when a cancel request, runtime abort, or local budget exhaustion occurs, and it SHALL preserve truthful continuity metadata for runtimes that support future resume work. For `claude_code`, continuity metadata MAY remain bridge-local snapshots. For `opencode`, continuity metadata MUST include the upstream OpenCode session binding required to continue the same run instead of replaying the original execute payload.

#### Scenario: Explicit cancel stops the active runtime through its truthful control plane
- **WHEN** the Go orchestrator submits a cancel request for an active task
- **THEN** the bridge SHALL abort the corresponding runtime through the runtime-specific control plane
- **THEN** it SHALL emit a terminal cancellation or failure event for that task
- **THEN** the runtime SHALL be removed from the active pool after cleanup

#### Scenario: Paused OpenCode run remains resumable without prompt replay
- **WHEN** the bridge pauses an active `opencode` task
- **THEN** the saved continuity metadata SHALL include the bound upstream OpenCode session identity and latest known resume metadata
- **THEN** a later resume request SHALL continue that same upstream session instead of starting a fresh execute call from the original payload

#### Scenario: Budget exhaustion terminates execution locally
- **WHEN** the bridge detects that the task's accumulated spend has reached or exceeded the task budget during a runtime that is still executing
- **THEN** the bridge SHALL stop the active run without waiting for Go to issue a separate cancel request
- **THEN** it SHALL emit an error or terminal event that identifies budget exhaustion
- **THEN** it SHALL store the latest truthful continuity metadata permitted for that runtime
