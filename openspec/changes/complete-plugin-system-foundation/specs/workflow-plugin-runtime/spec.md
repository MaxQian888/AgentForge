## ADDED Requirements

### Requirement: Workflow plugin manifests define executable orchestration contracts
The system SHALL accept `WorkflowPlugin` manifests that declare a process mode, participating role references, step definitions, trigger metadata, and execution limits. A workflow manifest MUST use stable step identifiers and MUST validate all referenced role ids, input references, and step transitions before the workflow can be enabled.

#### Scenario: Valid sequential workflow is registered
- **WHEN** a workflow manifest declares `process: sequential`, valid role references, unique step ids, and valid step transitions
- **THEN** the platform registers the workflow plugin and records it as an executable sequential workflow definition

#### Scenario: Workflow with unknown role reference is rejected
- **WHEN** a workflow manifest references a role id that does not exist in the unified role registry
- **THEN** the platform rejects the workflow before enablement and returns a validation error describing the unknown role reference

### Requirement: Sequential workflow plugins can be executed through the Go orchestrator
The system SHALL execute `WorkflowPlugin` definitions that declare `process: sequential` through the Go orchestrator by resolving each step's role binding, materializing step input from trigger data or prior step outputs, and invoking the corresponding agent, review, or task service seams in declared order.

#### Scenario: Manual sequential workflow completes in declared order
- **WHEN** an operator or internal service starts a valid enabled sequential workflow
- **THEN** the platform executes each declared step in order, persists each step outcome, and marks the workflow run completed only after the final step finishes successfully

#### Scenario: Disabled workflow cannot start
- **WHEN** a workflow plugin record is marked `disabled`
- **THEN** the platform refuses workflow execution requests for that plugin until it is re-enabled

### Requirement: Workflow execution state is persisted and observable
The system SHALL persist workflow run state, per-step status, step outputs, retry counters, and terminal outcome so operators and downstream services can inspect in-progress and completed workflow runs.

#### Scenario: Failed step retry is recorded
- **WHEN** a sequential workflow step fails and the manifest allows a retry
- **THEN** the platform records the failed attempt, increments the retry counter, and persists the later retry outcome on the same workflow run

#### Scenario: Workflow completion emits a terminal run state
- **WHEN** a workflow run reaches a completed, failed, or cancelled outcome
- **THEN** the platform stores the terminal workflow state and makes that outcome queryable through the workflow run record

### Requirement: Unsupported workflow modes fail explicitly
The system SHALL reject activation or execution of workflow manifests that declare a process mode or action type without a supported runtime implementation, and MUST return a structured unsupported-mode error instead of silently treating the workflow as executable.

#### Scenario: Hierarchical workflow remains non-executable until a runner exists
- **WHEN** a workflow plugin declares `process: hierarchical` before the hierarchical runner is implemented
- **THEN** the platform records the plugin definition but rejects execution with an explicit unsupported-mode error

