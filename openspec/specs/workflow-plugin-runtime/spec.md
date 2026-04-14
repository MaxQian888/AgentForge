# workflow-plugin-runtime Specification

## Purpose
Define how workflow plugins declare executable orchestration contracts, run through the Go orchestrator, persist execution state, and fail explicitly when unsupported workflow modes are requested.
## Requirements
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

### Requirement: Repository ships a built-in sequential workflow starter
The system SHALL provide at least one official built-in WorkflowPlugin starter as part of the repository's built-in plugin bundle. The starter MUST use the same manifest contract, sequential workflow process mode, and role resolution rules required of custom workflow plugins, and it MUST reference role identifiers that can be resolved from the current role registry.

#### Scenario: Built-in workflow starter is discoverable and installable
- **WHEN** the repository ships an official built-in workflow starter with a valid manifest and bundle entry
- **THEN** the control plane exposes it through built-in discovery and allows it to be installed through the explicit built-in install flow

#### Scenario: Built-in workflow starter fails validation on unknown role ids
- **WHEN** the built-in workflow starter references a role id that does not exist in the current role registry
- **THEN** the platform rejects that workflow starter before enablement with the same unknown-role validation error used for custom workflow plugins

### Requirement: Built-in workflow starter executes through the standard workflow runtime
An installed official built-in workflow starter SHALL execute through the same workflow runtime, persistence model, and run-history surfaces used by custom WorkflowPlugin definitions. The platform MUST NOT special-case built-in workflow starters by bypassing step persistence or workflow run visibility.

#### Scenario: Installed built-in workflow starter creates a normal workflow run
- **WHEN** an operator starts an installed and enabled built-in workflow starter
- **THEN** the platform creates a normal workflow run record, persists per-step status, and exposes the run through the existing workflow run query surfaces

#### Scenario: Built-in workflow starter remains sequential-only until broader modes are implemented
- **WHEN** the repository-maintained built-in starter declares process sequential
- **THEN** the platform executes it through the existing sequential runner and does not imply hierarchical or event-driven support beyond the current workflow runtime contract

### Requirement: Workflow role dependency health remains authoritative after registration
The system SHALL continue to evaluate workflow role references after a workflow plugin has been installed or enabled. Workflow plugin detail, activation, and execution paths MUST detect when a previously valid role binding has drifted out of the authoritative role registry and MUST return a stable dependency error instead of treating the workflow as healthy until step execution fails implicitly.

#### Scenario: Activating a workflow plugin with a stale role binding is refused
- **WHEN** an installed workflow plugin is enabled or activated after one of its referenced roles has been removed or become unresolved
- **THEN** the platform refuses the activation or enablement path with an explicit dependency error for the stale role binding
- **THEN** the same stale dependency state is visible through the workflow plugin's detail diagnostics

#### Scenario: Starting a workflow with a stale role binding fails before step execution
- **WHEN** an operator or internal service starts an installed workflow plugin whose current step role no longer resolves from the role registry
- **THEN** the workflow start fails before any step begins executing
- **THEN** the returned error identifies the stale role dependency instead of emitting a partial run with late step failure

### Requirement: Built-in workflow starters declare executable trigger profiles truthfully
Official built-in workflow starters SHALL declare structured trigger profiles that map to currently supported execution entrypoints such as manual start or supported task-driven activation. A built-in starter MUST NOT be presented as executable for a trigger profile unless the current workflow runtime and control-plane seam can actually start it through that profile.

#### Scenario: Starter declares a supported manual trigger profile
- **WHEN** an official built-in workflow starter declares a manual trigger profile supported by the current workflow runtime
- **THEN** the platform treats that trigger profile as executable for activation and start flows

#### Scenario: Unsupported trigger profile remains non-executable
- **WHEN** an official built-in workflow starter declares a trigger profile that the current workflow runtime or control plane cannot execute yet
- **THEN** the platform reports that trigger profile as unavailable or non-executable instead of implying the starter can already run that way

### Requirement: Built-in workflow starter library keeps dependency diagnostics after installation
The workflow plugin runtime SHALL preserve starter-library dependency diagnostics for installed official built-in workflow starters, including current role binding status and any declared service dependency gaps needed for execution. Activation and execution paths MUST fail before the first step begins when a built-in starter's current dependency state is no longer satisfied.

#### Scenario: Installed built-in starter shows dependency-ready state
- **WHEN** an installed official built-in workflow starter still resolves all declared roles and required services
- **THEN** activation and execution proceed through the normal workflow runtime path with dependency-ready diagnostics

#### Scenario: Installed built-in starter fails before execution on missing dependency
- **WHEN** an installed official built-in workflow starter no longer satisfies one of its declared role or service dependencies
- **THEN** the runtime refuses activation or execution before any workflow step begins
- **THEN** the returned error and diagnostics identify the missing dependency explicitly

