## MODIFIED Requirements

### Requirement: Frontend IMPlatform type SHALL include all supported operator providers
The `IMPlatform` type used by frontend IM operator surfaces SHALL include every provider id exposed by the authoritative operator-facing IM provider catalog. At minimum, the current repo truth MUST cover `"feishu" | "dingtalk" | "slack" | "telegram" | "discord" | "wecom" | "qq" | "qqbot" | "wechat" | "email"`. All frontend components that render, filter, or preselect IM platforms SHALL handle the full set instead of freezing on the earlier 8-platform snapshot.

#### Scenario: WeCom channel creation
- **WHEN** the operator selects `"wecom"` in the platform dropdown of `IMChannelConfig`
- **THEN** the form accepts the value as a valid `IMPlatform`
- **AND** the operator can save a channel with platform `"wecom"`

#### Scenario: QQ channel creation
- **WHEN** the operator selects `"qq"` in the platform dropdown
- **THEN** the form accepts the value as a valid `IMPlatform`
- **AND** the operator can save a channel with platform `"qq"`

#### Scenario: QQ Bot channel creation
- **WHEN** the operator selects `"qqbot"` in the platform dropdown
- **THEN** the form accepts the value as a valid `IMPlatform`
- **AND** the operator can save a channel with platform `"qqbot"`

#### Scenario: WeChat channel creation
- **WHEN** the operator selects `"wechat"` in the platform dropdown
- **THEN** the form accepts the value as a valid `IMPlatform`
- **AND** the operator can save a channel with platform `"wechat"`

#### Scenario: Email channel creation
- **WHEN** the operator selects `"email"` in the platform dropdown
- **THEN** the form accepts the value as a valid `IMPlatform`
- **AND** the operator can save a channel with platform `"email"` without treating it as an interactive chat provider

### Requirement: Platform list in channel config SHALL be driven by the authoritative provider catalog
The provider list and provider-specific configuration schema used by `IMChannelConfig` and related operator surfaces SHALL be derived from the authoritative backend IM provider catalog instead of a stale frontend-only schema registry. Frontend-local platform metadata MAY still provide visual concerns such as icon mapping, but provider availability, config fields, and operator affordances MUST come from the backend catalog.

#### Scenario: Catalog adds WeChat and Email to the operator truth
- **WHEN** the backend provider catalog returns `wechat` and `email` entries with configuration schema
- **THEN** `IMChannelConfig` renders those providers in the platform selector without requiring a second frontend-only provider schema list
- **AND** the same catalog entries are available to `/im`, provider badges, and settings summaries

#### Scenario: Catalog marks Email as delivery-only
- **WHEN** the backend provider catalog marks `email` as delivery-only while keeping channel configuration and test-send enabled
- **THEN** frontend operator surfaces render Email as a valid configurable provider
- **AND** they do not invent interactive slash-command or callback affordances for Email

### Requirement: IMChannelConfig SHALL render platform-specific configuration fields
When the operator selects a platform, the channel form SHALL dynamically render the additional configuration fields declared by the authoritative provider catalog for that platform. Fields not relevant to the selected platform SHALL be hidden, and provider-specific affordances such as delivery-only labeling MUST remain truthful in the rendered form state.

#### Scenario: Switching platform from Feishu to WeChat
- **WHEN** the operator changes platform selection from `"feishu"` to `"wechat"`
- **THEN** the form hides Feishu-specific assumptions
- **AND** it renders the WeChat-specific configuration fields required by the catalog, such as app id, app secret, and callback token

#### Scenario: Switching platform from Feishu to Email
- **WHEN** the operator changes platform selection from `"feishu"` to `"email"`
- **THEN** the form hides interactive chat assumptions
- **AND** it renders the Email-specific configuration fields required by the catalog, such as SMTP host, SMTP port, and from address
- **AND** the surrounding UI keeps Email labeled as delivery-only rather than a callback-driven chat provider
