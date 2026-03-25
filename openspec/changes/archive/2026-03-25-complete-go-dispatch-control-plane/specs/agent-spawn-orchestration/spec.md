## ADDED Requirements

### Requirement: Manual spawn and queued promotion reuse dispatch control-plane guardrails
The system SHALL route task-scoped manual spawn requests and queue promotions through the same dispatch control-plane preflight used by assignment-triggered dispatch. Manual spawn and promotion MUST reuse task/member context resolution, budget admission checks, worktree readiness, and structured non-started outcomes instead of bypassing the task-centered dispatch contract.

#### Scenario: Manual spawn returns a structured queued outcome
- **WHEN** an operator requests manual spawn for a task and AgentPool admission has no immediate slot available
- **THEN** the synchronous spawn result returns `queued`
- **THEN** the result includes the queue reference and resolved dispatch context used for that admission decision
- **THEN** the system MUST NOT create a real agent run until that queued request is later admitted

#### Scenario: Manual spawn is blocked by dispatch guardrails before runtime startup
- **WHEN** an operator requests manual spawn for a task but dispatch preflight fails because of budget, task/member validity, or other control-plane guardrails
- **THEN** the synchronous spawn result returns `blocked`
- **THEN** the result carries the same machine-readable guardrail classification used by assignment-triggered dispatch
- **THEN** the system MUST NOT create a new agent run for that request

#### Scenario: Queue promotion revalidates the canonical dispatch preflight
- **WHEN** a queued dispatch becomes eligible for promotion after capacity is released
- **THEN** the system re-runs the canonical dispatch preflight before creating runtime state
- **THEN** only a passing decision may create a new agent run and persist task runtime metadata
- **THEN** a failing recheck is surfaced through the queue lifecycle without leaving ambiguous runtime state behind
