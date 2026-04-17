## ADDED Requirements

### Requirement: Backend SHALL expose an authoritative operator-facing IM provider catalog
The Go backend SHALL expose an operator-facing IM provider catalog at a canonical authenticated API surface so `/im`, settings, and other operator workspaces can discover the supported IM providers without relying on stale frontend constants. The catalog MUST include one entry per operator-visible built-in provider, MUST remain available even when no bridge instance for that provider is currently registered, and MUST expose enough metadata for operator surfaces to render truthful controls: normalized provider id, display label, interaction class, test-send support, channel-configuration support, and provider-specific configuration field schema.

#### Scenario: Catalog includes WeChat and Email with truthful affordances
- **WHEN** an operator requests the canonical IM provider catalog
- **THEN** the response includes entries for `wechat` and `email`
- **AND** the `wechat` entry is marked as an interactive chat provider
- **AND** the `email` entry is marked as delivery-only instead of pretending full callback or slash-command parity

#### Scenario: Catalog remains available without active bridge registration
- **WHEN** no active bridge instance is currently registered for WeChat or Email
- **THEN** the canonical IM provider catalog still returns those provider entries
- **AND** operator configuration surfaces can prepare channels and validation rules without waiting for a live bridge instance

### Requirement: IM API validation SHALL honor provider affordances per surface
Operator-facing IM APIs SHALL validate provider ids according to the authoritative provider catalog instead of using one flat platform allowlist across every surface. Interactive inbound surfaces such as message or action handling MUST accept only providers whose catalog affordance includes interactive chat input. Delivery surfaces such as channel configuration, outbound send, notify, or operator test-send MUST continue to accept delivery-capable providers even when they are delivery-only.

#### Scenario: Interactive surface accepts WeChat and rejects Email
- **WHEN** an inbound IM message or action request identifies `wechat` as the provider
- **THEN** the backend accepts the provider as a valid interactive IM source
- **AND** the same interactive validation rejects `email` as a fake chat-command provider

#### Scenario: Delivery surface accepts Email as a valid provider
- **WHEN** an operator saves an Email channel configuration or submits an Email test-send request
- **THEN** the backend accepts `email` as a valid delivery-capable provider
- **AND** the response or validation metadata keeps Email marked as delivery-only rather than upgrading it to interactive chat parity
