## ADDED Requirements

### Requirement: Feishu native card transport SHALL use the documented interactive message envelope
The Feishu provider SHALL send and reply richer cards through the documented interactive message envelope: IM message APIs MUST use `msg_type: "interactive"` and MUST serialize the card body into the `content` JSON string expected by Feishu. Callback acknowledgements and delayed-update calls MUST use the documented response body shape for the relevant path instead of reusing one provider payload form across send, reply, callback, and deferred update flows.

#### Scenario: Send or reply path serializes an interactive card body
- **WHEN** the Bridge sends or replies with a Feishu native card for `/help`, command guidance, or any other richer operator response
- **THEN** the outbound Feishu request uses `msg_type: "interactive"`
- **AND** the card body is serialized into the documented `content` JSON string instead of being sent as an unwrapped object or plain text payload

#### Scenario: Callback acknowledgement distinguishes raw and template card responses
- **WHEN** a Feishu `card.action.trigger` callback is acknowledged with a card-changing response
- **THEN** the synchronous callback response uses the documented raw-card or template-card response shape for that callback path
- **AND** the Bridge does not pretend that an IM send/reply payload can be reused unchanged as the callback response body

### Requirement: Feishu callback acknowledgement SHALL stay truthful to callback readiness and response timing
If the current Feishu runtime cannot accept a `card.action.trigger` callback through either its active long-connection intake or a configured webhook intake, the Bridge MUST treat callback-backed affordances as unavailable instead of advertising them as ready. When the runtime is callback-ready, synchronous callback handling MUST acknowledge the interaction within the documented response window while preserving the token and reply-target context needed for any later delayed update.

#### Scenario: Callback-ready runtime preserves synchronous acknowledgement context
- **WHEN** the Feishu runtime receives a `card.action.trigger` callback and the action resolves within the immediate-response path
- **THEN** the Bridge acknowledges the callback within the provider response window
- **AND** the preserved reply target still retains the callback token and message identity required for any later delayed update or fallback

#### Scenario: Callback-missing runtime does not expose callback-backed affordances
- **WHEN** the active Feishu runtime has no usable callback intake for card interactions, whether through long connection or webhook callback
- **THEN** the Bridge does not advertise callback-backed quick actions as if they were executable
- **AND** Feishu-facing help or command guidance falls back to plain executable commands with explicit callback-unavailable messaging
