## ADDED Requirements

### Requirement: Desktop notification delivery outcomes are exposed additively to the frontend
The desktop event bridge SHALL emit normalized notification outcome events for native desktop delivery attempts. Outcome events MUST distinguish at least delivered, suppressed, and failed results, MUST include the related business notification identifier plus summary metadata when available, and MUST remain additive to the existing backend notification API and websocket business events instead of replacing them.

#### Scenario: Native delivery emits a delivered outcome event
- **WHEN** the desktop bridge successfully shows a native notification for a business notification
- **THEN** the desktop event bridge emits a normalized delivered outcome event that frontend subscribers can consume for diagnostics or UI feedback

#### Scenario: Suppressed delivery still emits an observable outcome
- **WHEN** the desktop bridge suppresses a native notification because the documented foreground policy applies
- **THEN** the desktop event bridge emits a normalized suppressed outcome event instead of leaving the frontend unable to tell whether delivery was skipped or broken

#### Scenario: Failed native delivery does not replace business notification truth
- **WHEN** native desktop delivery fails for a business notification
- **THEN** the desktop event bridge emits a normalized failed outcome event
- **AND** the same notification continues to exist through the standard backend notification API and in-app notification store
