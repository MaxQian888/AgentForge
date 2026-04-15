## ADDED Requirements

### Requirement: Task transition actions SHALL execute canonical task lifecycle workflows
The system SHALL treat canonical task transition actions submitted through `/api/v1/im/action` as executable backend task lifecycle requests. When the Bridge submits a supported task transition action such as `transition-task`, the backend MUST execute the corresponding task status transition through the canonical task workflow surface and MUST return a truthful action result containing the updated task state or the explicit blocked or failed reason.

#### Scenario: Transition-task action updates the real task state
- **WHEN** the Bridge submits `transition-task` for a valid task together with a supported target status and preserved reply target
- **THEN** the backend transitions that task through the canonical task status workflow instead of returning a placeholder acknowledgement
- **AND** the action result reports the updated task identity and resulting workflow status

#### Scenario: Invalid task transition returns an explicit non-success outcome
- **WHEN** the Bridge submits `transition-task` for a missing task, an unsupported target status, or an invalid status transition
- **THEN** the backend returns an explicit blocked or failed action outcome
- **AND** the result does not claim that the task state changed when no canonical transition was applied

#### Scenario: Transition-task result preserves reply-target lineage for later follow-up
- **WHEN** an IM-originated task transition action succeeds or is blocked
- **THEN** the returned IM action result preserves the canonical reply target and task identity associated with that originating conversation
- **AND** later task or workflow follow-up delivery can reuse that lineage without inventing a new destination
