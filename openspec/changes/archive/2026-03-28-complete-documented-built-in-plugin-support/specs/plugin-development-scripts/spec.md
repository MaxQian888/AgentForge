## ADDED Requirements

### Requirement: Repository verifies maintained built-in plugins by host family
The repository SHALL provide supported verification workflows for the official built-in plugin bundle grouped by host family instead of relying on one monolithic live smoke run. Maintained Go-hosted wasm plugins, MCP tool plugins, MCP review plugins, and workflow starters MUST each have a documented verification path that identifies the target plugin and the workflow stage being validated.

#### Scenario: MCP built-in plugin verification validates maintained entrypoints
- **WHEN** a developer or CI workflow verifies an official built-in ToolPlugin or ReviewPlugin
- **THEN** the repository validation checks the maintained manifest, declared entrypoint or command contract, and family-specific prerequisites without requiring edits to script internals

#### Scenario: Workflow starter verification pinpoints starter-specific failures
- **WHEN** a maintained built-in workflow starter drifts from its declared manifest, role references, or execution contract
- **THEN** the verification workflow exits non-zero and identifies the failing workflow starter and validation stage instead of reporting a generic plugin failure

### Requirement: Live smoke remains explicit for prerequisite-heavy built-ins
The repository SHALL distinguish deterministic default verification from optional live smoke for official built-ins that require network access, secrets, or third-party local services. Default verification MUST remain bounded and reproducible, while live smoke MUST be opt-in and clearly identify the missing prerequisite when it cannot run.

#### Scenario: Default verification skips secret-dependent live execution
- **WHEN** an official built-in plugin requires secrets or external network access for live runtime execution
- **THEN** the default verification path validates the maintained contract without silently attempting a flaky live run

#### Scenario: Opt-in live smoke reports missing prerequisite
- **WHEN** a developer explicitly triggers live smoke for a prerequisite-heavy built-in without satisfying its dependencies
- **THEN** the live smoke command fails with a message that identifies the missing prerequisite instead of hanging or reporting an unrelated runtime error
