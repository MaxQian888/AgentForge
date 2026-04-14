## MODIFIED Requirements

### Requirement: Attachment support in ExecuteRequest
The Bridge SHALL accept an optional `attachments` field in ExecuteRequest containing an array of `{ type: "image" | "file"; path: string; mime_type?: string }`. Each runtime adapter MUST either map attachments to its native mechanism or reject the execute request before upstream execution begins with an explicit runtime/input error. Claude Code SHALL map supported attachments through its SDK input mechanism. Codex SHALL pass supported image attachments via `--image` flags. OpenCode SHALL translate supported attachments into official session or prompt parts and SHALL NOT silently drop unsupported attachment types or file payloads.

#### Scenario: Runtime maps a supported attachment
- **WHEN** ExecuteRequest includes an attachment that the selected runtime supports natively
- **THEN** the runtime adapter passes that attachment through the runtime's official attachment mechanism
- **THEN** execution begins with the attachment still available to the agent

#### Scenario: Runtime cannot honor the requested attachment input
- **WHEN** ExecuteRequest includes an attachment type or payload that the selected runtime cannot represent truthfully
- **THEN** the Bridge rejects the execute request before upstream execution starts
- **THEN** the returned error identifies the runtime, the `attachments` field, and the unsupported or missing prerequisite

### Requirement: Session rollback operation
The Bridge SHALL expose `POST /bridge/rollback` accepting `{ task_id, checkpoint_id?: string, turns?: number }`. The Bridge MUST resolve rollback through runtime-specific continuity and upstream-native controls instead of static unsupported defaults. For Claude Code, this SHALL call `query.rewindFiles()` when checkpointed continuity exists. For Codex, this SHALL use the saved thread continuity and the bridge-owned rollback runner. For OpenCode, this SHALL translate the rollback request into continuity-backed revert or unrevert operations. If the task lacks the required continuity or rollback target, the Bridge MUST return a structured degraded or unsupported error that identifies the runtime and missing prerequisite.

#### Scenario: Rollback to checkpoint for Claude Code
- **WHEN** `/bridge/rollback` is called with `{ task_id, checkpoint_id: "msg-uuid-42" }` for a Claude Code task with file checkpointing enabled
- **THEN** the Bridge calls `query.rewindFiles("msg-uuid-42")`
- **THEN** the request returns success without falling back to a generic unsupported response

#### Scenario: Rollback uses runtime continuity for a non-Claude runtime
- **WHEN** `/bridge/rollback` is called for a Codex or OpenCode task that has continuity with a resolvable rollback target
- **THEN** the Bridge delegates to that runtime's official thread or session rollback path
- **THEN** the returned status reflects the runtime-specific result instead of blanket unsupported behavior

#### Scenario: Rollback target cannot be resolved
- **WHEN** `/bridge/rollback` is called for a runtime whose continuity lacks a resolvable checkpoint or revert target
- **THEN** the Bridge rejects the request with a structured runtime-specific rollback error
- **THEN** the error includes the runtime, support state, and reason code for the missing prerequisite

### Requirement: Web search toggle in ExecuteRequest
The Bridge SHALL accept an optional `web_search?: boolean` in ExecuteRequest. For Codex, this SHALL add the `--search` flag to the CLI command. For Claude Code and OpenCode, the adapter MUST only enable web search through an official runtime tool or configuration surface for the selected runtime and provider. If no truthful mapping exists for the selected runtime/provider combination, the Bridge MUST reject the request before execution instead of silently treating web search as implied by unrelated tool allowances.

#### Scenario: Runtime maps web search request
- **WHEN** ExecuteRequest includes `web_search: true` for a runtime and provider that publish web-search support
- **THEN** the runtime adapter enables search through the runtime's official search mechanism
- **THEN** the run begins with web-search support still reflected in runtime capability metadata

#### Scenario: Runtime does not publish web search support
- **WHEN** ExecuteRequest includes `web_search: true` for a runtime or provider that does not publish truthful search support
- **THEN** the Bridge rejects the request before execution starts
- **THEN** the error identifies `web_search` as the rejected field and does not silently drop the request intent

### Requirement: Environment variable injection
The Bridge SHALL accept an optional `env?: Record<string, string>` in ExecuteRequest. For Claude Code, this SHALL map to the `env` option. For Codex, this SHALL map to process environment overrides. For OpenCode, the Bridge MUST propagate env overrides through official session or config metadata when the selected server exposes that capability. If the selected runtime cannot guarantee environment delivery truthfully, the Bridge MUST reject the request during preflight instead of dropping the overrides.

#### Scenario: Custom environment variables are injected through a supported runtime path
- **WHEN** ExecuteRequest includes `env: { "DATABASE_URL": "postgres://..." }` for a runtime that publishes env support
- **THEN** the runtime adapter injects the environment variables through the runtime's official execution context
- **THEN** execution starts with the same env intent preserved in runtime handling

#### Scenario: Runtime cannot preserve env overrides truthfully
- **WHEN** ExecuteRequest includes `env` for a runtime or provider that lacks an official env path
- **THEN** the Bridge rejects the request before upstream prompt submission
- **THEN** the runtime catalog reports the same env capability as unsupported or degraded with an actionable reason

### Requirement: Cross-runtime interaction controls publish truthful support state
The Bridge SHALL publish every advanced interaction control with an explicit support state that matches the runtime catalog and route behavior. For each runtime-specific control or input surface, the published state MUST distinguish `supported`, `degraded`, and `unsupported`, and MUST be derived from the same runtime/preflight rules used by execute validation and route handlers. Published support MUST account for readiness, provider-auth or config prerequisites, and continuity-dependent controls such as rollback.

#### Scenario: Route is invoked for a supported interaction control
- **WHEN** a caller invokes a lifecycle or interaction control that the selected runtime publishes as supported
- **THEN** the Bridge SHALL execute the canonical control path for that runtime
- **THEN** the returned status and diagnostics SHALL remain consistent with the capability metadata published in the runtime catalog

#### Scenario: Route is invoked for an unsupported interaction control
- **WHEN** a caller invokes a lifecycle or interaction control that the selected runtime publishes as unsupported
- **THEN** the Bridge SHALL reject the request with a structured error that identifies the runtime, operation, support state, and reason code
- **THEN** it SHALL NOT silently drop the control or pretend the request completed successfully

#### Scenario: Execute preflight and catalog share degraded reasoning
- **WHEN** a parity-sensitive input or control is unavailable because provider auth, runtime config, or continuity state is missing
- **THEN** the runtime catalog publishes the same degraded or unsupported reason code that execute preflight or route handlers return
- **THEN** upstream consumers can suppress or gate the request before execution without guessing
