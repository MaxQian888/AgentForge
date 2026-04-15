# task-triggered-workflow-automation Specification

## Purpose
Define how project task transition triggers start canonical workflow runs, normalize trigger actions, and expose truthful orchestration outcomes for task-driven workflow automation.
## Requirements
### Requirement: Project task workflow triggers normalize canonical action contracts
The system SHALL normalize project task workflow trigger actions into canonical control-plane actions before execution. At minimum, the canonical actions MUST include `dispatch_agent`, `start_workflow`, `notify`, and `auto_transition`. The backend MUST continue accepting supported legacy aliases for backward compatibility, but it MUST normalize them before execution and before emitting any outward trigger outcome. Unknown actions or invalid trigger config MUST return an explicit structured non-success outcome instead of silently no-oping.

#### Scenario: Legacy dispatch alias is normalized before execution
- **WHEN** a stored project workflow trigger still uses a legacy dispatch alias such as `auto_assign` or `auto_assign_agent`
- **THEN** the backend normalizes that trigger to the canonical `dispatch_agent` action before evaluating execution
- **THEN** any emitted trigger outcome exposes `dispatch_agent` as the normalized action instead of the legacy alias

#### Scenario: Invalid workflow-start trigger is rejected explicitly
- **WHEN** a matching trigger declares action `start_workflow` but omits the required workflow starter or profile reference
- **THEN** the backend returns a structured non-success outcome for that trigger
- **THEN** the system MUST NOT create a workflow run for the invalid trigger

### Requirement: Task transitions can start canonical workflow runs
The system SHALL allow project task workflow triggers to start workflow runs through the canonical workflow plugin runtime when the matched trigger references a workflow starter and trigger profile that the current control plane marks executable. A successful task-triggered start MUST carry task identity into the workflow run trigger payload and MUST reuse the same run persistence, dependency validation, and step execution contract as a manual workflow start.

#### Scenario: Matching task transition starts a built-in task delivery workflow
- **WHEN** a project workflow trigger matches a task transition and references the executable task-driven profile of `task-delivery-flow`
- **THEN** the backend starts a canonical workflow run for `task-delivery-flow`
- **THEN** the created workflow run trigger payload includes the originating task identity required by the starter

#### Scenario: Duplicate active run is not started twice
- **WHEN** the same task matches the same task-driven workflow starter profile while an earlier run for that task and profile is still `pending`, `running`, or `paused`
- **THEN** the backend returns a structured non-started outcome for the later trigger
- **THEN** the system MUST NOT create a second active workflow run for that same task and starter profile

### Requirement: Task-triggered workflow outcomes remain visible to automation consumers
The system SHALL emit a structured trigger outcome for every matched project task workflow rule. The outward outcome MUST include the normalized action, matched transition, result status, and the machine-readable metadata needed to explain whether the trigger started work, completed synchronously, was blocked, was skipped, or failed. When the action starts a workflow run, the outward outcome MUST include the starter identity and workflow run reference.

#### Scenario: Successful workflow start is broadcast with lineage metadata
- **WHEN** a matched project task workflow trigger starts a workflow run successfully
- **THEN** the emitted `workflow.trigger_fired` event includes the normalized action, the trigger outcome, the workflow starter identity, and the started workflow run reference
- **THEN** downstream consumers do not need to infer the started workflow from free-form log text alone

#### Scenario: Unsupported starter profile remains explicit in trigger outcome
- **WHEN** a matched project task workflow trigger references a starter profile that is not executable for task-driven activation
- **THEN** the emitted trigger outcome marks the trigger as blocked or failed with a machine-readable reason
- **THEN** the system MUST NOT emit a fake started outcome for that trigger
