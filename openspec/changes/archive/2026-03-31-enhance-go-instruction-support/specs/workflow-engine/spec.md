# Workflow Engine Specification

## ADDED Requirements

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
