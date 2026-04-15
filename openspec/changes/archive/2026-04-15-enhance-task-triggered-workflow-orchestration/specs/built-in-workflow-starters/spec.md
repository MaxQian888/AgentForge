## ADDED Requirements

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
