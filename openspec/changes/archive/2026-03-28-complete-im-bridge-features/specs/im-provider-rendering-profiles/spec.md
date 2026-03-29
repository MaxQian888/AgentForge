## MODIFIED Requirements

### Requirement: Active IM provider SHALL publish a rendering profile

Each runnable IM provider SHALL define supported text formatting modes, structured rendering preferences, message length limits, mutable-update constraints, and card capability declarations. Optional provider-owned builders SHALL turn typed delivery into final provider payloads.

The DingTalk provider SHALL declare `card: true` and `cardUpdate: false` in its rendering profile, indicating support for sending ActionCard messages but no support for in-place card updates. The profile SHALL include an ActionCard builder that maps typed envelope actions to DingTalk ActionCard button payloads.

#### Scenario: DingTalk provider publishes rendering profile with card support
- **WHEN** DingTalk provider initializes
- **THEN** its rendering profile declares `{text: true, markdown: false, card: true, cardUpdate: false, callback: "stream"}` and includes an ActionCard payload builder

#### Scenario: Feishu provider declares full card lifecycle
- **WHEN** Feishu provider initializes
- **THEN** its rendering profile declares `{text: true, markdown: true, card: true, cardUpdate: true, callback: "longConnection"}` with delayed-update constraints

#### Scenario: QQ provider declares text-only profile
- **WHEN** QQ provider initializes
- **THEN** its rendering profile declares `{text: true, markdown: false, card: false, cardUpdate: false, callback: "websocket"}` with reply-segment awareness
