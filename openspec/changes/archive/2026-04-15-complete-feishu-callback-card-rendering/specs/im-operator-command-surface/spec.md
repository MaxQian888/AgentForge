## ADDED Requirements

### Requirement: Feishu help discoverability SHALL reflect callback-backed affordance readiness
When `/help` targets Feishu, the Bridge SHALL tailor the output to the runtime's real callback readiness. If callback-backed quick actions are available, the help response MUST expose them through the Feishu card surface. If callback handling is unavailable, the help response MUST fall back to readable command guidance that tells the operator which plain commands to send manually.

#### Scenario: Callback-ready Feishu help shows executable quick actions
- **WHEN** an IM user sends `/help` on Feishu and the active runtime exposes a usable callback intake through long connection or webhook configuration
- **THEN** the Bridge renders callback-backed quick actions for the supported help shortcuts
- **AND** those actions align with the canonical operator catalog rather than inventing platform-only commands

#### Scenario: Callback-missing Feishu help falls back to readable manual guidance
- **WHEN** an IM user sends `/help` on Feishu but the active runtime does not have usable callback readiness for card actions
- **THEN** the Bridge omits callback-backed quick actions from the help response
- **AND** the response includes readable manual command guidance for the same supported flows
