## ADDED Requirements

### Requirement: Assignment-triggered dispatch outcomes remain canonical across consumer contracts
The system SHALL expose assignment-triggered dispatch through one canonical outcome contract for synchronous API callers, realtime consumers, and IM bridge clients. The contract MUST preserve explicit `started`, `queued`, `blocked`, and `skipped` branches, together with any queue references and machine-readable guardrail details required to explain non-started outcomes.

#### Scenario: Queued assignment preserves queue metadata across surfaces
- **WHEN** assigning a task to an eligible agent member results in a queued dispatch outcome
- **THEN** the synchronous assignment response includes the authoritative queue reference and dispatch metadata for that request
- **THEN** the corresponding realtime and IM consumer contracts expose the same `queued` branch instead of collapsing it to generic success or idle text
- **THEN** consumers can identify that assignment succeeded even though runtime startup has not begun yet

#### Scenario: Human assignment remains an explicit skipped outcome
- **WHEN** a task assignment targets a human member and therefore does not attempt agent startup
- **THEN** the synchronous response returns a `skipped` dispatch outcome
- **THEN** IM and other consumer contracts preserve that `skipped` branch explicitly
- **THEN** consumers MUST NOT infer a human assignment indirectly from the absence of run or queue data

#### Scenario: Blocked assignment preserves a machine-readable dispatch reason
- **WHEN** task assignment succeeds but dispatch preflight blocks runtime startup
- **THEN** the synchronous response and realtime dispatch payload include the blocked outcome together with a machine-readable reason classification
- **THEN** IM consumer output identifies that assignment completed while startup did not begin
- **THEN** consumers can distinguish budget, pool, worktree, and target-validation blocks without relying on free-form reason strings alone
