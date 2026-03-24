# task-progress-alerting Specification

## Purpose
Define the baseline contract for task progress warning, stalled, and recovery alerts across realtime events, persisted in-product notifications, and optional IM fan-out without duplicating unchanged conditions.

## Requirements
### Requirement: Progress risk alerts reach in-product consumers
The system SHALL emit user-visible progress alerts when a task enters a warning or stalled condition so that people can react before work silently stops.

#### Scenario: Task enters a risk condition
- **WHEN** the system determines that a task has entered an at-risk or stalled progress state
- **THEN** the system broadcasts a project-scoped realtime alert event for that task
- **AND** the system creates at least one persisted in-product notification that identifies the task, the risk condition, and a follow-up destination

#### Scenario: Alert contains actionable context
- **WHEN** a progress alert is created for a task
- **THEN** the alert payload includes the task identifier, title, current workflow status, detected reason, and alert timestamp
- **AND** the receiving client can route the user back to the affected task or related work surface without a blind search

### Requirement: Progress alert delivery is deduplicated and escalates only on meaningful change
The system SHALL suppress duplicate progress alerts while the same underlying condition remains unchanged, while still allowing escalation when the condition materially worsens or recurs after recovery.

#### Scenario: Detector re-evaluates the same stalled condition
- **WHEN** the periodic detector sees that a task is still in the same stalled condition and no recovery or severity change has occurred
- **THEN** the system does not create duplicate notifications for that unchanged condition
- **AND** the system preserves enough internal state to know that the alert has already been delivered

#### Scenario: Condition worsens or returns after recovery
- **WHEN** a task's progress condition escalates in severity or re-enters a risk state after having recovered
- **THEN** the system is allowed to emit a new alert for that updated condition
- **AND** the new alert is linked to the latest signal state rather than replaying stale context

### Requirement: IM delivery is best-effort and recovery is communicated
The system SHALL support forwarding progress alerts to IM targets when routing information exists, and it SHALL communicate recovery without letting IM failures break the core alerting path.

#### Scenario: IM target is available for a progress alert
- **WHEN** the system creates a progress alert for a task and a routable IM target is available for the recipient
- **THEN** the system forwards the alert to the IM notification receiver in addition to the in-product channels
- **AND** a failure to deliver the IM message does not roll back the persisted notification or realtime alert event

#### Scenario: Previously alerted task recovers
- **WHEN** a task that previously emitted a progress alert returns to a healthy progress state
- **THEN** the system emits a recovery signal to realtime consumers
- **AND** the system may create a recovery notification only when there was a prior active progress alert to resolve
