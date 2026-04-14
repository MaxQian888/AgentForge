## MODIFIED Requirements

### Requirement: Assignment and dispatch outcomes are visible to synchronous and realtime consumers
The agent detail page SHALL display dispatch context including the dispatch outcome (started, queued, blocked, skipped), the authoritative runtime tuple used for that dispatch, queue or promotion context when present, and the machine-readable preflight or guardrail summary needed to explain why runtime startup did or did not happen. This context SHALL be shown as a dedicated section on the agent detail page so operators can understand how the agent was dispatched without navigating away.

#### Scenario: Agent detail shows dispatch outcome and resolved runtime tuple
- **WHEN** operator views an agent's detail page
- **THEN** a Dispatch Context section displays the dispatch outcome (started/queued/blocked/skipped)
- **AND** shows the resolved runtime, provider, and model used for that dispatch when those values were part of the dispatch decision
- **AND** shows the preflight summary including budget status, queue context, and pool state at dispatch time when available

#### Scenario: Agent detail shows rich dispatch history for the task
- **WHEN** operator views an agent's detail page for a task with multiple dispatch attempts
- **THEN** the Dispatch Context section shows the dispatch history for that task
- **THEN** each attempt includes outcome, timestamp, trigger source, and the machine-readable runtime or guardrail metadata needed to distinguish queued, blocked, skipped, and started attempts
- **THEN** queued or promotion-related attempts surface their queue reference or equivalent admission context instead of collapsing to a free-form reason string

#### Scenario: Agent without dispatch context shows minimal info
- **WHEN** operator views an agent that was spawned manually without dispatch
- **THEN** the Dispatch Context section shows "Manual spawn" with the spawn timestamp
- **AND** no preflight or guardrail data is displayed

### Requirement: Assignment-triggered dispatch outcomes remain canonical across consumer contracts
The system SHALL expose assignment-triggered dispatch through one canonical outcome contract for synchronous API callers, realtime consumers, and IM bridge clients. The contract MUST preserve explicit `started`, `queued`, `blocked`, and `skipped` branches, together with queue references, runtime tuple metadata, budget guardrail metadata, and machine-readable guardrail details required to explain non-started outcomes.

#### Scenario: Queued assignment preserves queue metadata across surfaces
- **WHEN** assigning a task to an eligible agent member results in a queued dispatch outcome
- **THEN** the synchronous assignment response includes the authoritative queue reference, priority level, resolved runtime tuple, and dispatch metadata for that request
- **THEN** the corresponding realtime and IM consumer contracts expose the same `queued` branch and queue metadata instead of collapsing it to generic success or idle text
- **THEN** consumers can identify that assignment succeeded even though runtime startup has not begun yet

#### Scenario: Human assignment remains an explicit skipped outcome
- **WHEN** a task assignment targets a human member and therefore does not attempt agent startup
- **THEN** the synchronous response returns a `skipped` dispatch outcome
- **THEN** realtime and IM consumer contracts preserve that `skipped` branch explicitly
- **THEN** consumers MUST NOT infer a human assignment indirectly from the absence of run or queue data

#### Scenario: Blocked assignment preserves machine-readable dispatch metadata
- **WHEN** task assignment succeeds but dispatch preflight blocks runtime startup
- **THEN** the synchronous response and realtime dispatch payload include the blocked outcome together with machine-readable guardrail metadata
- **THEN** the metadata distinguishes budget, pool, worktree, and target-validation blocks without relying on free-form reason strings alone
- **THEN** IM consumer output identifies that assignment completed while startup did not begin and preserves the same machine-readable guardrail fields

#### Scenario: Budget-blocked dispatch carries the governing scope in metadata
- **WHEN** a dispatch is blocked because a budget limit was exceeded
- **THEN** the dispatch outcome includes a `guardrailScope` field identifying which scope (task, sprint, or project) caused the block
- **THEN** the dispatch outcome includes a `guardrailType` field with value `budget`
- **THEN** the canonical consumer contracts can render the budget-blocked state with the specific scope without parsing the reason string
