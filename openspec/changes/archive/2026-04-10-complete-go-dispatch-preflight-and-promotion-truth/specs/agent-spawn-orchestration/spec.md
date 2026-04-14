## MODIFIED Requirements

### Requirement: Manual spawn and queued promotion reuse dispatch control-plane guardrails
The system SHALL route task-scoped manual spawn requests and queue promotions through the same canonical dispatch preflight used by assignment-triggered dispatch. Manual spawn and promotion MUST reuse task or member context resolution, task or sprint or project budget admission checks, active-run conflict checks, transient system guardrail classification, and structured non-start outcomes, and they MUST emit matching history and queue metadata for every decisive verdict.

#### Scenario: Manual spawn returns a structured queued outcome
- **WHEN** an operator requests manual spawn for a task and AgentPool admission has no immediate slot available
- **THEN** the synchronous spawn result returns `queued`
- **THEN** the result includes the queue reference, priority, resolved runtime tuple, and machine-readable dispatch context used for that admission decision
- **THEN** the system MUST NOT create a real agent run until that queued request is later admitted

#### Scenario: Manual spawn is blocked by canonical dispatch guardrails before runtime startup
- **WHEN** an operator requests manual spawn for a task but canonical dispatch preflight fails because of budget, task or member validity, active-run conflict, or other control-plane guardrails
- **THEN** the synchronous spawn result returns `blocked`
- **THEN** the result carries the same machine-readable guardrail classification used by assignment-triggered dispatch
- **THEN** the resulting dispatch history entry preserves the same canonical metadata instead of degrading to a free-form failure string
- **THEN** the system MUST NOT create a new agent run for that request

#### Scenario: Queue promotion requeues recoverable guardrail failures with a matching history verdict
- **WHEN** a queued dispatch reaches promotion revalidation and the latest canonical preflight fails because of a recoverable budget or transient infrastructure guardrail
- **THEN** the queue entry remains queued with refreshed latest guardrail metadata and recovery disposition
- **THEN** the system records a new dispatch history verdict for that promotion recheck instead of silently mutating queue state only
- **THEN** consumers can distinguish a still-queued recoverable recheck from the original admission event

#### Scenario: Queue promotion records terminal invalidation truthfully
- **WHEN** a queued dispatch reaches promotion revalidation and the latest canonical preflight fails because task or member context is irrecoverably invalid
- **THEN** the queue entry transitions to a terminal failed state with machine-readable guardrail metadata
- **THEN** the system records a terminal dispatch history verdict for that promotion recheck
- **THEN** the control plane MUST NOT retry that entry as though it were still recoverable

#### Scenario: Successful promotion persists queue linkage and started verdict together
- **WHEN** a queued dispatch passes promotion revalidation and starts runtime execution successfully
- **THEN** the system records the resulting start through the canonical dispatch history path with linkage back to the originating queue entry
- **THEN** the queue lifecycle is completed with the promoted run linkage before promotion payloads are emitted to consumers
- **THEN** operator and realtime consumers can relate the started run back to the original queued admission without reconstructing it from free-form text
