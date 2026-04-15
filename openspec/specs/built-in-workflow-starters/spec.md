# built-in-workflow-starters Specification

## Purpose
Define the official built-in WorkflowPlugin starter library so AgentForge ships maintained task-delivery and review-escalation workflow templates alongside its minimal quickstart starter.
## Requirements
### Requirement: Repository ships an official built-in workflow starter library
The repository SHALL ship an official built-in WorkflowPlugin starter library that includes `standard-dev-flow`, `task-delivery-flow`, and `review-escalation-flow`. Each starter MUST remain manifest-backed, listed in the official built-in plugin bundle, and validated against the authoritative role registry before it is treated as an executable official starter.

#### Scenario: Built-in workflow starter library is discoverable
- **WHEN** the platform exposes official built-in workflow starters for the current checkout
- **THEN** `standard-dev-flow`, `task-delivery-flow`, and `review-escalation-flow` are discoverable as distinct official starters with valid manifests and bundle entries

#### Scenario: Starter with stale role dependency is rejected
- **WHEN** one of the official built-in workflow starters references a role id that no longer resolves from the current role registry
- **THEN** that starter fails validation before enablement or execution
- **THEN** the returned error identifies the stale role dependency explicitly

### Requirement: Task delivery starter preserves planner to coding to review handoff
The `task-delivery-flow` starter SHALL execute as a sequential workflow that hands off from `planner-agent` to `coding-agent` to `code-reviewer` using the existing workflow runtime and persisted step-output model. Later steps MUST be able to consume prior step outputs through the same workflow output contract used by other sequential workflows.

#### Scenario: Planner output is available to implementation step
- **WHEN** `task-delivery-flow` completes its planner step successfully
- **THEN** the subsequent coding step receives the planner step output through the persisted workflow step-output contract

#### Scenario: Review step executes after coding step completion
- **WHEN** the coding step of `task-delivery-flow` completes successfully
- **THEN** the workflow runtime starts the review step after the coding step rather than in parallel
- **THEN** the run history records each step outcome in order

### Requirement: Review escalation starter expresses deep-review to approval handoff
The `review-escalation-flow` starter SHALL use currently supported workflow actions to trigger review execution, persist the resulting review metadata, and pause at an approval step when operator or human intervention is required. The starter MUST surface review identity and pause state through the normal workflow run record instead of special-casing built-in behavior.

#### Scenario: Review escalation starter persists review output
- **WHEN** `review-escalation-flow` completes its review step successfully
- **THEN** the workflow run stores the review metadata as step output available to later steps and diagnostics

#### Scenario: Approval step pauses the review escalation flow
- **WHEN** `review-escalation-flow` reaches its approval step
- **THEN** the workflow runtime records the run as awaiting approval and pauses progression instead of silently completing the workflow

### Requirement: Starter library exposes truthful task-driven availability for task orchestration
The official built-in workflow starter library SHALL distinguish starters that are executable from project task transitions from starters that remain manual-only. At minimum, `task-delivery-flow` MUST declare at least one executable task-driven trigger profile that carries task identity into the canonical workflow runtime. A starter that does not yet have a supported task-driven path, including `review-escalation-flow` when no such path is implemented, MUST remain explicitly manual-only or unavailable for task-triggered orchestration.

#### Scenario: Task delivery starter exposes an executable task-driven profile
- **WHEN** the platform exposes the official built-in starter library for the current checkout
- **THEN** `task-delivery-flow` is discoverable with at least one task-driven trigger profile that the current task workflow control-plane can execute
- **THEN** that profile identifies the task context required for planner to coding to review handoff

#### Scenario: Manual-only starter is not misrepresented as task-triggerable
- **WHEN** an official built-in starter does not currently support task-driven activation for the current runtime and control-plane seam
- **THEN** the platform marks that starter or profile as manual-only or unavailable for task-triggered orchestration
- **THEN** project task workflow configuration does not treat that starter profile as executable by default
