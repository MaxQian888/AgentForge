## MODIFIED Requirements

### Requirement: Built-in workflow starters declare executable trigger profiles truthfully
Official built-in workflow starters SHALL declare structured trigger profiles that map to currently supported execution entrypoints such as manual start or supported task-driven activation. A built-in starter MUST NOT be presented as executable for a trigger profile unless the current workflow runtime and control-plane seam can actually start it through that profile with the required task or trigger context. If a starter is manual-only or a specific task-driven profile is unsupported, the platform MUST return a structured availability or dependency error instead of implying the starter can already run that way.

#### Scenario: Starter declares a supported manual trigger profile
- **WHEN** an official built-in workflow starter declares a manual trigger profile supported by the current workflow runtime
- **THEN** the platform treats that trigger profile as executable for activation and start flows

#### Scenario: Supported task-driven profile starts through the canonical task workflow control-plane
- **WHEN** an official built-in workflow starter declares a task-driven trigger profile that the current task workflow control-plane supports
- **AND** a matching task transition supplies the required task context for that profile
- **THEN** the platform treats that trigger profile as executable through the canonical task workflow control-plane
- **THEN** the resulting workflow run uses the normal workflow runtime persistence and dependency checks rather than a task-only shortcut path

#### Scenario: Unsupported trigger profile remains non-executable
- **WHEN** an official built-in workflow starter declares a trigger profile that the current workflow runtime or control plane cannot execute yet
- **THEN** the platform reports that trigger profile as unavailable or non-executable instead of implying the starter can already run that way

## ADDED Requirements

### Requirement: Task-driven starter activation preserves duplicate and dependency truth
When a built-in workflow starter is started through a task-driven trigger profile, the workflow runtime and task workflow control-plane SHALL preserve duplicate-run and dependency truth before the first step begins. The platform MUST fail or block the task-driven activation before step execution when the required task context is missing, the starter dependency state is stale, or an equivalent active run already exists for the same task and trigger profile.

#### Scenario: Missing task context prevents task-driven starter execution
- **WHEN** a task-driven workflow starter activation request reaches the runtime without the task identity required by the declared trigger profile
- **THEN** workflow start fails before any step begins executing
- **THEN** the returned error identifies the missing task context instead of creating a partial run

#### Scenario: Equivalent active run blocks duplicate task-driven activation
- **WHEN** the control-plane receives a second task-driven activation request for the same task, starter, and trigger profile while an earlier run is still active
- **THEN** the platform returns a structured non-started verdict for the duplicate request
- **THEN** the runtime MUST NOT create a second active workflow run for that same task-driven scope
