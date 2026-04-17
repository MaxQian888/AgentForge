## ADDED Requirements

### Requirement: Per-asset subscription filter registered on open

The system SHALL register, for each open asset session, the union of event topics returned by each live-block projector's `Subscribe(targetRef)` call. Subscriptions SHALL be keyed by `(client_id, asset_id)` and discarded when the client navigates away or disconnects.

#### Scenario: Subscription registered on open

- **WHEN** a client opens a wiki-page asset containing three live blocks (one `agent_run`, two `task_group`)
- **THEN** the server registers a subscription filter covering the agent_run topic for the first block's target id and the task topics for the two task_group filters, scoped to that client and asset

#### Scenario: Subscription released on navigate-away

- **WHEN** a client navigates away from the asset or disconnects
- **THEN** the server removes the subscription filter for `(client_id, asset_id)`

### Requirement: Live-artifacts-changed WebSocket event

The system SHALL broadcast a `knowledge.asset.live_artifacts_changed` event to clients whose subscription filter matches when any referenced entity emits a relevant event. The event payload SHALL carry `{asset_id, block_ids_affected: [string]}`.

#### Scenario: Entity event triggers push

- **WHEN** an `AgentRun` emits a status change event that matches a subscription filter for a viewer's open asset
- **THEN** the server sends `knowledge.asset.live_artifacts_changed` to that viewer with `asset_id` and the block id(s) that reference that run

#### Scenario: Push targets only affected clients

- **WHEN** an entity event occurs and is referenced by open assets for some but not all connected clients
- **THEN** only the clients whose subscription filter matches receive the push

### Requirement: Coalescing window for high-churn entities

The system SHALL coalesce multiple entity events matching the same client/asset pair within a 250 ms window into a single `live_artifacts_changed` payload with the union of affected block ids.

#### Scenario: Five events in 200ms become one push

- **WHEN** five events for different blocks in the same open asset arrive within 200 ms of each other for one client
- **THEN** the client receives one `live_artifacts_changed` payload whose `block_ids_affected` contains all five block ids

#### Scenario: Events straddling coalesce window produce two pushes

- **WHEN** events for the same asset arrive at t=0ms and t=260ms for one client
- **THEN** the client receives two separate `live_artifacts_changed` pushes

### Requirement: Per-block refresh-rate cap

The system SHALL cap the push rate per individual block to at most one `live_artifacts_changed` entry per second per client. Subsequent matching events within the window SHALL be dropped (the next scheduled push already informs the client that the block needs re-projection).

#### Scenario: Runaway entity emits 100 events in one second

- **WHEN** an entity emits 100 events in one second, all matching a subscription filter for a given block in a given client's open asset
- **THEN** the client receives at most one `live_artifacts_changed` push for that block in that second

### Requirement: Client re-projection triggered by push

The frontend SHALL respond to `live_artifacts_changed` by calling the projection endpoint with only the `block_ids_affected`. Unaffected blocks SHALL NOT be re-projected.

#### Scenario: Client refreshes only affected blocks

- **WHEN** the client receives `live_artifacts_changed` with `block_ids_affected=[b1, b2]`
- **THEN** the client calls the projection endpoint with only those two block refs and replaces their rendered content on success

#### Scenario: Client re-projects all blocks on reconnect

- **WHEN** the WebSocket reconnects after a disconnect
- **THEN** the client re-projects every live block currently visible in the open asset (full refresh) before reestablishing the subscription filter server-side
