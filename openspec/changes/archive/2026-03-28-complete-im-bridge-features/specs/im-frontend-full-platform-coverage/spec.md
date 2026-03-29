## ADDED Requirements

### Requirement: Frontend IMPlatform type SHALL include all 8 supported platforms

The `IMPlatform` union type in `lib/stores/im-store.ts` SHALL include `"feishu" | "dingtalk" | "slack" | "telegram" | "discord" | "wecom" | "qq" | "qqbot"`. All frontend components that render or filter by platform SHALL handle the full set.

#### Scenario: WeCom channel creation
- **WHEN** operator selects "WeCom" in the platform dropdown of IMChannelConfig
- **THEN** the form renders WeCom-specific configuration fields (corpId, agentId, callbackToken) and saves a channel with platform `"wecom"`

#### Scenario: QQ channel creation
- **WHEN** operator selects "QQ" in the platform dropdown
- **THEN** the form renders QQ-specific fields (onebot endpoint URL, access token) and saves a channel with platform `"qq"`

#### Scenario: QQ Bot channel creation
- **WHEN** operator selects "QQ Bot" in the platform dropdown
- **THEN** the form renders QQ Bot fields (app ID, app secret, webhook callback URL) and saves a channel with platform `"qqbot"`

### Requirement: Platform list in channel config SHALL be driven by a shared constant

A shared `PLATFORM_DEFINITIONS` constant SHALL define the label, icon, and platform-specific config fields for all 8 platforms. `IMChannelConfig` and any other component needing platform metadata SHALL reference this constant instead of maintaining independent lists.

#### Scenario: New platform added to definitions
- **WHEN** a new platform entry is added to `PLATFORM_DEFINITIONS`
- **THEN** `IMChannelConfig` dropdown, `IMBridgeHealth` provider list, and `IMMessageHistory` platform column all render the new platform without code changes in those components

### Requirement: IMBridgeHealth SHALL display per-provider capability summary

The bridge health panel SHALL show each registered provider's declared capabilities (card support, update support, callback mode) alongside connection status.

#### Scenario: Bridge with DingTalk provider registered
- **WHEN** bridge status returns a DingTalk provider with capabilities `{card: true, cardUpdate: false, callback: "stream"}`
- **THEN** the health panel displays capability badges for card support and stream callback mode

### Requirement: IMChannelConfig SHALL render platform-specific configuration fields

When the operator selects a platform, the channel form SHALL dynamically render additional configuration fields defined in `PLATFORM_DEFINITIONS` for that platform. Fields not relevant to the selected platform SHALL be hidden.

#### Scenario: Switching platform from Feishu to WeCom
- **WHEN** operator changes platform selection from "feishu" to "wecom"
- **THEN** Feishu-specific fields disappear and WeCom fields (corpId, agentId, callbackToken) appear

### Requirement: Event subscription list SHALL be fetched from backend

The event type checklist in `IMChannelConfig` SHALL be loaded from `GET /im/event-types` instead of a hardcoded frontend array. This ensures new event types (sprint.started, review.requested, workflow.failed) are available without frontend code changes.

#### Scenario: Backend adds new event type
- **WHEN** backend returns `["task.created", "task.completed", "review.completed", "agent.started", "agent.completed", "budget.warning", "sprint.started", "sprint.completed", "review.requested", "workflow.failed"]`
- **THEN** the channel config form renders all 10 event types as checkboxes

### Requirement: Shared PlatformBadge component SHALL be extracted for reuse

A `PlatformBadge` component in `components/shared/` SHALL render a platform icon + label given an `IMPlatform` value. It SHALL be used by IMChannelConfig, IMBridgeHealth, IMMessageHistory, and Settings notification configuration.

#### Scenario: PlatformBadge renders WeCom
- **WHEN** `<PlatformBadge platform="wecom" />` is rendered
- **THEN** a badge with WeCom icon and "WeCom" label is displayed

### Requirement: Shared EventBadgeList component SHALL be extracted for reuse

An `EventBadgeList` component in `components/shared/` SHALL render a list of event type badges. It SHALL be used in IMChannelConfig (selected events display) and any notification configuration UI.

#### Scenario: EventBadgeList renders multiple events
- **WHEN** `<EventBadgeList events={["task.created", "review.completed"]} />` is rendered
- **THEN** two styled badges are displayed with human-readable labels
