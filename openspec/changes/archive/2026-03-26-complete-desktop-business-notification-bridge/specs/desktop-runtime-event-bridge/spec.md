## MODIFIED Requirements

### Requirement: Desktop notification delivery outcomes are exposed additively to the frontend
The desktop event bridge SHALL emit normalized notification outcome events for native desktop delivery attempts. Outcome events MUST distinguish at least delivered, suppressed, and failed results, MUST include the related business notification identifier, notification type, title, and any available `href` or policy metadata needed for frontend coordination, and MUST remain additive to the existing backend notification API and websocket business events instead of replacing them.

#### Scenario: Native delivery emits a delivered outcome event
- **WHEN** the desktop bridge successfully shows a native notification for a business notification
- **THEN** the desktop event bridge emits a normalized delivered outcome event that includes the business notification identifier and summary metadata
- **AND** frontend subscribers can consume it for diagnostics or UI feedback without re-fetching a second notification source

#### Scenario: Foreground suppression emits an observable outcome
- **WHEN** the desktop bridge suppresses a native notification because the documented foreground policy applies
- **THEN** the desktop event bridge emits a normalized suppressed outcome event that includes the related business notification identifier and suppression context
- **AND** the frontend remains able to distinguish intentional suppression from broken delivery

#### Scenario: Failed native delivery does not replace business notification truth
- **WHEN** native desktop delivery fails for a business notification
- **THEN** the desktop event bridge emits a normalized failed outcome event with the related business notification identifier
- **AND** the same notification continues to exist through the standard backend notification API and in-app notification store
