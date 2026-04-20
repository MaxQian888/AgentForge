## ADDED Requirements

### Requirement: Plugin manifest `workflow` step accepts an engine discriminator

Workflow plugin manifest validation SHALL accept an optional `targetKind` discriminator on any step whose action is `workflow`, with supported values `plugin` (default, existing behavior) and `dag`. A `workflow` step MAY declare either the legacy `pluginId` shape (implicit `targetKind='plugin'`) or the new engine-aware shape with `targetKind` and `targetWorkflowId`. Validation MUST reject a step that mixes both shapes inconsistently.

#### Scenario: Legacy plugin_id step shape continues to validate
- **WHEN** a manifest declares a `workflow` step with `pluginId` (or `plugin_id`) and no `targetKind`
- **THEN** manifest validation passes and the step defaults to `targetKind='plugin'`

#### Scenario: New DAG target shape validates
- **WHEN** a manifest declares a `workflow` step with `targetKind='dag'` and a well-formed `targetWorkflowId` UUID
- **THEN** manifest validation passes

#### Scenario: Conflicting shape fails validation
- **WHEN** a manifest declares a `workflow` step with `targetKind='dag'` alongside a `pluginId`
- **THEN** manifest validation rejects the step with a structured error identifying the conflicting shape

#### Scenario: Unknown targetKind fails validation
- **WHEN** a manifest declares a `workflow` step with `targetKind` set to an unsupported value
- **THEN** manifest validation rejects the step with a structured error identifying the unknown target kind

### Requirement: Plugin run exposes `awaiting_sub_workflow` step status

The workflow plugin run persistence SHALL support an `awaiting_sub_workflow` step status, exposed on per-step state in the run record, to represent a step parked while a DAG child executes. The overall plugin run status MUST be derivable from step status such that a run containing any `awaiting_sub_workflow` step is not reported as terminal.

#### Scenario: Parked step reflected on plugin run status
- **WHEN** a plugin run's `workflow` step is parked with `awaiting_sub_workflow`
- **THEN** the run's persisted step state shows that status
- **THEN** the run's overall status is not reported as `completed`, `failed`, or `cancelled` while the step remains parked

#### Scenario: Plugin run read exposes parked step status
- **WHEN** a caller reads a plugin run containing a parked `workflow` step
- **THEN** the response DTO exposes the `awaiting_sub_workflow` status on that step
