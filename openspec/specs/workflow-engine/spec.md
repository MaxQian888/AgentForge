# workflow-engine Specification

## Purpose
Define the current Go workflow execution contract for executable workflow plugins, supported process modes, retry behavior, approval pauses, and routed workflow step actions.
## Requirements
### Requirement: Workflow execution starts only from executable workflow plugins
The system SHALL start workflow runs from plugin records that are enabled or active and declare one of the currently executable workflow process modes.

#### Scenario: Reject disabled workflow plugin
- **WHEN** the caller starts a workflow whose plugin record is disabled
- **THEN** workflow start fails instead of executing the workflow

#### Scenario: Accept currently executable process modes
- **WHEN** the workflow plugin declares process mode `sequential`, `hierarchical`, `event-driven`, or `wave`
- **THEN** the workflow execution service accepts that process mode for execution

### Requirement: Sequential workflow execution persists ordered step state and prior outputs
The workflow execution service SHALL persist workflow runs and execute sequential workflows in declared step order, exposing earlier step outputs to later steps.

#### Scenario: Execute ordered sequential workflow
- **WHEN** a sequential workflow contains ordered steps such as `implement` followed by `review`
- **THEN** the workflow execution service executes the steps in that declared order

#### Scenario: Later step receives earlier step outputs
- **WHEN** a later workflow step executes after one or more earlier steps complete
- **THEN** the later step input includes the previous step outputs keyed by step ID

### Requirement: Workflow step execution supports current action router contracts
The current workflow step router SHALL execute the supported step actions `agent`, `review`, `task`, `workflow`, and `approval`, validating the required trigger payload for each action type.

#### Scenario: Agent action spawns agent run
- **WHEN** a workflow step uses action `agent` with a valid trigger payload containing task and member context
- **THEN** the step router spawns an agent run and returns the run metadata as step output

#### Scenario: Review action triggers review execution
- **WHEN** a workflow step uses action `review` with either task or PR context
- **THEN** the step router triggers a review and returns review metadata as step output

#### Scenario: Task action dispatches task execution
- **WHEN** a workflow step uses action `task` with valid trigger task context
- **THEN** the step router dispatches task execution and returns the dispatch outcome

#### Scenario: Workflow action starts child workflow
- **WHEN** a workflow step uses action `workflow` and provides a child plugin reference through trigger or step config
- **THEN** the step router starts the child workflow and returns child workflow metadata

#### Scenario: Approval action pauses workflow progression
- **WHEN** a workflow step uses action `approval`
- **THEN** the step router returns an `awaiting_approval` status payload for the workflow run

### Requirement: Workflow retries persist attempt history
The workflow execution service SHALL retry failed steps up to the configured workflow retry budget and persist step attempt history across retries.

#### Scenario: Retry transient step failure within configured retry budget
- **WHEN** a workflow step fails and the workflow plugin allows at least one retry
- **THEN** the step is retried
- **THEN** the persisted step state records both the failed and successful attempts

#### Scenario: Exhaust retry budget and fail workflow
- **WHEN** a workflow step continues failing after the configured retry budget is exhausted
- **THEN** the workflow run is marked failed
- **THEN** the failed step state and attempt history remain persisted on the run

### Requirement: Workflow process modes honor current pause and parallelism semantics
The workflow execution service SHALL implement the currently supported process-mode semantics for hierarchical, event-driven, and wave workflows.

#### Scenario: Hierarchical workflow no longer fails as unsupported
- **WHEN** the workflow plugin declares process mode `hierarchical`
- **THEN** the execution service does not reject the workflow solely because of that process mode

#### Scenario: Event-driven workflow executes only triggered steps
- **WHEN** an event-driven workflow starts with a trigger payload that resolves specific step IDs
- **THEN** only the matching steps execute and unmatched steps are marked skipped

#### Scenario: Wave workflow runs steps in parallel within the same wave
- **WHEN** a wave workflow contains multiple steps in the same execution wave
- **THEN** the workflow execution service runs those steps concurrently within that wave

#### Scenario: Awaiting approval pauses the workflow run
- **WHEN** a step output reports status `awaiting_approval`
- **THEN** the workflow execution service pauses the workflow run instead of continuing to later steps

### Requirement: Workflow runs remain queryable after execution
The workflow execution service SHALL persist workflow runs so callers can retrieve a single run or list recent runs by plugin ID after execution.

#### Scenario: Retrieve persisted workflow run by ID
- **WHEN** a completed or failed workflow run has been stored
- **THEN** the caller can fetch that run by its workflow run ID

#### Scenario: List workflow runs by plugin
- **WHEN** multiple workflow runs exist for the same workflow plugin
- **THEN** the caller can list recent runs filtered by plugin ID

### Requirement: Legacy workflow step router accepts optional employee attribution

The current workflow step router SHALL accept an optional `employee_id` on the trigger payload of its `agent`, `review`, and `task` actions, in addition to the existing `member_id`. When `employee_id` is provided, the spawned agent run, review, or task dispatch MUST carry that employee identifier so downstream records attribute the work to that Digital Employee. When `employee_id` is absent, the step router falls back to the run-level `acting_employee_id` recorded on the workflow run, and when that too is absent, the spawned run attributes only through the existing `member_id` pathway.

#### Scenario: Agent action with step-level employee attribution
- **WHEN** a legacy workflow step of action `agent` declares an explicit `employee_id = E` on its trigger payload
- **THEN** the step router spawns an agent run whose `employee_id` equals E

#### Scenario: Agent action falls back to run-level acting employee
- **WHEN** a legacy workflow step of action `agent` omits `employee_id` and the workflow run declares `acting_employee_id = E`
- **THEN** the step router spawns an agent run whose `employee_id` equals E

#### Scenario: Review action with step-level employee attribution
- **WHEN** a legacy workflow step of action `review` declares an explicit `employee_id = E`
- **THEN** the triggered review carries E as the acting employee on its record

#### Scenario: Task action with step-level employee attribution
- **WHEN** a legacy workflow step of action `task` declares an explicit `employee_id = E`
- **THEN** the dispatched task attributes its spawned runs to E

#### Scenario: Absent employee attribution preserves existing behavior
- **WHEN** neither the step nor the workflow run declares any employee identifier
- **THEN** the spawned agent run's `employee_id` remains null and the step continues to execute through the existing `member_id` pathway unchanged

### Requirement: Legacy workflow action can target a DAG child workflow

The current workflow step router SHALL accept an optional `targetKind` discriminator on the trigger payload (or step config) of the `workflow` action, with supported values `plugin` (default, existing behavior) and `dag`. When `targetKind='dag'` is declared, the router MUST resolve the child through the target-engine registry and start a DAG workflow execution as the child run. Omitting `targetKind` MUST preserve the existing `pluginId` / `plugin_id` resolution path unchanged.

#### Scenario: Workflow action defaults to plugin child when discriminator omitted
- **WHEN** a legacy `workflow` step provides `trigger.pluginId` and omits `targetKind`
- **THEN** the step router starts a child workflow plugin run exactly as it does today
- **THEN** the step output envelope includes `child_run_id`, `child_plugin`, and `status`

#### Scenario: Workflow action starts a DAG child when targetKind='dag'
- **WHEN** a legacy `workflow` step provides `targetKind='dag'` and a valid `targetWorkflowId`
- **THEN** the step router starts a DAG workflow execution as the child run
- **THEN** the step output envelope includes `child_run_id`, `child_engine='dag'`, `child_workflow_id`, and `status`

#### Scenario: Workflow action rejects unknown targetKind
- **WHEN** a legacy `workflow` step provides `targetKind` with an unsupported value
- **THEN** the step router returns a structured error identifying the unknown target kind
- **THEN** no child run is started

### Requirement: Legacy workflow step parks when DAG child is long-running

When the child is a DAG workflow, the legacy `workflow` step SHALL park the parent plugin run's step in an `awaiting_sub_workflow` status until the DAG child reaches a terminal state. The parent plugin step MUST resume when the DAG child completes, fails, or is cancelled. When the child is a plugin, the existing synchronous behavior of the `workflow` action is preserved.

#### Scenario: DAG child in progress parks the parent plugin step
- **WHEN** the `workflow` step starts a DAG child that has not yet reached a terminal state
- **THEN** the parent plugin run's step transitions to `awaiting_sub_workflow` status
- **THEN** the plugin run's overall status reflects the parked step and does not falsely report completion

#### Scenario: DAG child completion resumes the parent plugin step
- **WHEN** the DAG child reaches the `completed` terminal state
- **THEN** the parent plugin step transitions to `completed`
- **THEN** subsequent plugin steps execute normally

#### Scenario: DAG child failure fails the parent plugin step
- **WHEN** the DAG child reaches the `failed` terminal state
- **THEN** the parent plugin step transitions to `failed` with a structured reason identifying the child run

#### Scenario: Parent plugin cancellation cascades to DAG child
- **WHEN** the parent plugin run is cancelled while its `workflow` step is parked with a DAG child in progress
- **THEN** the DAG child is cancelled or detached from the linkage
- **THEN** the parent plugin step reports a cancellation terminal state
