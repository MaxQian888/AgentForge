## ADDED Requirements

### Requirement: The standalone marketplace workspace renders truthful browse, detail, and status states
The system SHALL provide a standalone `/marketplace` operator workspace for plugin, skill, and role marketplace items. The workspace MUST render list, detail, filter, empty, loading, unavailable, blocked, and actionable states from real marketplace and consumption contracts instead of silently swallowing errors or inferring installability from incomplete local state.

#### Scenario: Operator browses marketplace items with truthful states
- **WHEN** the operator opens `/marketplace`
- **THEN** the workspace loads marketplace items, featured items, and typed install or consumption state from the configured services
- **THEN** each selected item shows its type, version availability, verification state, installability, and downstream consumer status without requiring the operator to infer them from generic badges

#### Scenario: Marketplace service is unavailable
- **WHEN** the configured marketplace service is unreachable, misconfigured, or returns an invalid response
- **THEN** the workspace shows an explicit unavailable state with a stable reason
- **THEN** the workspace MUST NOT fall back to an empty result set that implies the marketplace has no items

#### Scenario: Item detail distinguishes installable, installed, and used states
- **WHEN** the operator opens the detail view for an item
- **THEN** the workspace shows whether the item is installable, already installed, or already discoverable in its downstream consumer surface
- **THEN** the workspace exposes the next supported action such as install, manage, or open the downstream consumer surface

### Requirement: Authors and administrators can complete the item lifecycle from the workspace
The standalone marketplace workspace SHALL expose the existing marketplace service lifecycle for item publishing, version management, reviews, and moderation. Authorized authors MUST be able to publish and manage item versions from the workspace, and authorized administrators MUST be able to verify or feature items without leaving the operator UI.

#### Scenario: Author publishes an item and uploads a version
- **WHEN** an authenticated author creates a marketplace item and then uploads a version artifact for that item
- **THEN** the workspace persists the item metadata and new version through the marketplace service contracts
- **THEN** the detail view refreshes to show the uploaded version, its latest or yanked state, and any validation failures from the upload flow

#### Scenario: Administrator moderates an item
- **WHEN** an authenticated administrator opens an item that is eligible for verification or featuring
- **THEN** the workspace shows the supported moderation actions for that user's role
- **THEN** the resulting verified or featured state becomes visible in both the item detail view and marketplace list surfaces

#### Scenario: Unauthorized operator cannot use author or admin-only actions
- **WHEN** an operator lacks the permissions required to manage item versions or moderation state
- **THEN** the workspace suppresses or disables those actions with an explicit permission reason
- **THEN** read-only browsing and review visibility continue to work

### Requirement: The workspace supports explicit install and side-load flows
The standalone marketplace workspace SHALL support explicit install confirmation and side-load flows for supported item types. The workspace MUST distinguish publish-time side-loading from install-time side-loading, reuse the repository's existing source and provenance model where applicable, and keep blocked or unsupported flows operator-visible.

#### Scenario: Operator installs an item from the marketplace workspace
- **WHEN** the operator confirms installation for a supported marketplace item and version
- **THEN** the workspace invokes the type-appropriate install contract instead of a direct browser download shortcut
- **THEN** the item only moves into an installed state after the downstream consumer surface reports a successful handoff

#### Scenario: Operator side-loads a local marketplace artifact
- **WHEN** the operator chooses a supported local file or path side-load action from the marketplace workspace
- **THEN** the workspace uses the supported local source flow for that item type and records provenance for the imported asset
- **THEN** the workspace shows whether the imported asset became a draft marketplace item, a local install candidate, or a completed consumer-side installation

#### Scenario: Unsupported side-load or install flow stays blocked
- **WHEN** the selected item type or host environment does not support the requested side-load or install flow
- **THEN** the workspace shows a stable blocked reason and the supported next step
- **THEN** it MUST NOT report a misleading successful installation or hide the unavailable action entirely
