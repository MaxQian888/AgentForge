## ADDED Requirements

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
