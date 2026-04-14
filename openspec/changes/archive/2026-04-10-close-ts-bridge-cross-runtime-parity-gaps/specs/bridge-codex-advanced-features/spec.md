## ADDED Requirements

### Requirement: Codex rollback is exposed through the canonical Bridge control plane
The Bridge SHALL resolve `/bridge/rollback` for Codex by using saved Codex thread continuity together with a bridge-owned rollback runner that drives the official Codex thread control surface. The runner MUST accept `checkpoint_id` or `turns`, update Codex continuity after a successful rewind, and return structured rollback errors when the thread cannot be rewound truthfully.

#### Scenario: Rollback existing Codex thread
- **WHEN** `/bridge/rollback` is called for a Codex task with saved thread continuity and a valid rollback target
- **THEN** the Bridge invokes the Codex rollback runner for that thread
- **THEN** the request returns success and the updated continuity remains resumable

#### Scenario: Rollback requested without Codex thread continuity
- **WHEN** `/bridge/rollback` is called for a Codex task that lacks saved thread continuity or a resolvable rollback target
- **THEN** the Bridge rejects the request with a structured runtime-specific rollback error
- **THEN** the error identifies the missing continuity prerequisite instead of a generic `unsupported_operation`
