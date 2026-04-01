## ADDED Requirements

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
