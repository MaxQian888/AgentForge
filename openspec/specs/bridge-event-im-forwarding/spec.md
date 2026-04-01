# bridge-event-im-forwarding Specification

## Purpose
Define how TS Bridge events are forwarded through the Go backend to IM platforms with ordered, preference-aware delivery.
## Requirements
### Requirement: Budget alerts forwarded to IM platforms
TS Bridge SHALL forward `budget_alert` events to Go backend, which then forwards them to IM platforms based on user preferences and budget thresholds, and alert frequency.

#### Scenario: Budget alert with default thresholds
- **WHEN** Bridge emits `budget_alert` with default thresholds (80%, 90%, 100%)
- **then** go backend checks user preferences, finds default thresholds acceptable
    - Go backend forwards to all IM platforms
    - if enabled, send alert to users subscribed to task
    - otherwise, log and alert without sending
 notification
  - If disabled, send "Budget alerts disabled for this task" message

#### Scenario: Budget alert with frequency limit
- **WHEN** user sets frequency limit to 3 alerts per day
- **then** Bridge emits `budget_alert` with frequency limit of 3 alerts per day
    - if user preferences don't limit, alert, - **then** IM Bridge sends "Budget alerts temporarily disabled for this task" message
    - otherwise, send critical alert with links to adjust budget
- **then** if frequency still 3/day, IM Bridge sends critical alert to user
 in DM immediately
      - IM Bridge replies: "⚠️ Budget alert: 3 alerts per day threshold reached. Consider pausing agent or adjusting budget."
"

### Requirement: Agent status changes forwarded to IM for operational visibility
TS Bridge SHALL forward `agent_status_change` events to Go backend, which then forwards them to IM platforms for real-time status updates for#### Scenario: Agent started successfully
- **WHEN** Bridge emits `agent_status_change` event with `{ old_status: "starting", new_status: "running" }` for task "task-123", runtime: "claude_code" }`
- **then** bridge calls `POST http://localhost:7778/ws/bridge` to`forward toGo backend WebSocket
 - **then** go backend determines task watchers ( from subscriptions
    - if watchers exist, forward to their IM clients
      - IM Bridge sends notification: "🚀 Agent started\n - **then** IM Bridge updates message with agent status card
 - **if no watchers, just update task status in database
- **then** IM Bridge logs the change for monitoring

 - **then** IM Bridge replies with confirmation

#### Scenario: Agent paused successfully
- **WHEN** Bridge emits `agent_status_change` event with `{ old_status: "running", new_status: "paused" }` for task "task-123", runtime: "claude_code" }
`
- **then** bridge calls `POST http://localhost:7778/ws/bridge` to forward to Go backend
    - Go backend updates task status in database to `paused`
    - Go backend determines task watchers from subscriptions
      - If watchers exist, forward to their IM clients
        - IM Bridge sends notification: "⏸ Agent paused"
    - **then** IM Bridge logs status change in task timeline
    - **if no watchers, just log "No watchers" (won't spam)
 - **then** IM Bridge replies with "Agent paused (no watchers)"

#### Scenario: Agent failed
- **WHEN** Bridge emits `agent_status_change` event with `{ old_status: "running", new_status: "failed" }` for task "task-123", runtime: "claude_code" }`
- **then** bridge calls `POST http://localhost:7778/ws/bridge` to forward to Go backend
    - Go backend updates task status to `failed` in database
    - Go backend marks task as degraded (not subscribed to IM)
    - IM Bridge sends notification: "❌ Agent failed"
      - Error details available
    - **if watchers exist**, suggest creating follow-up task from findings

      - IM Bridge replies with "Agent failed" and action buttons

#### Scenario: Agent completed successfully
- **WHEN** Bridge emits `agent_status_change` event with `{ old_status: "running", new_status: "completed" }` for task "task-123", runtime: "claude_code" }`
- **then** bridge calls `POST http://localhost:7778/ws/bridge` to forward to Go backend
    - Go backend updates task status in database to `completed`
    - Go backend removes task from IM subscriber list
    - **if watchers exist**, forward to their IM clients
      - IM Bridge sends notification: "✅ Agent completed"
    - **then** IM Bridge logs completion in task timeline
    - **if watchers exist**, forward to agent-spawned subtasks
      - IM Bridge replies with "Agent completed successfully"

### Requirement: Event ordering guarantees delivery
Go backend and IM notification system SHALL preserve event ordering when forwarding events from Bridge to IM platforms.

#### Scenario: Permission request delivered before agent request event
- **WHEN** bridge emits `permission_request` then agent status change events
- **THEN** Go backend forwards permission request to IM platforms
- **Bircuit:**
 events should be processed in order:
  1. Deduplicate similar events from different providers
  2. Avoid event loss by removing duplicate fields
  3. Use consistent event type mapping across both system

#### Scenario: Permission request and status change events
- **WHEN** Bridge emits `permission_request`, then `status_change` event to `paused`
 state
    - **then** bridge emits `permission_request` event again with updated status `paused`
 (permission request now pending)
    - **then** IM Bridge receives both events in order: permission request first, status change event second
 then bridge's final state

#### Scenario: Agent status change events processed quickly
- **WHEN** Bridge emits multiple rapid `agent_status_change` events in succession
    - **then** go backend processes and event in sequence and preserves ordering
    - **then** go backend forwards to IM platforms in batch
    - **then** IM clients receive and display the updates in real-time

#### Scenario: High-priority budget alert skips agent status updates
- **WHEN** user configures budget alerts as high priority for agent status updates
- **then** bridge sends budget alert before of agent status updates
    - **then** go backend processes agent status update and budget alert before forwarding to IM
    - **then** IM Bridge receives the budget alert first, then forwards the agent status update

### Requirement: Event forwarding respects user notification preferences
The user's preferences for IM platform (via Go backend) SHALL determine which Bridge events are forwarded to IM platforms and how they are formatted.

#### Scenario: User enables detailed budget alerts
- **WHEN** user preferences show detailed budget alerts
    - **and** go backend has user preferences for a `budget_alerts.detailed` flag set to true
    - Bridge forwards detailed budget alert to IM
  - **then** IM Bridge displays detailed alert with breakdown by budget percentage, current usage, and trend analysis

#### Scenario: User disables specific event types
- **WHEN** user preferences have `budget_alerts.enabled` set to false for some event types
  - **then** go backend filters out budget alerts from forwarding logic
  - **then** those event types are not forwarded to IM

#### Scenario: User customizes notification format per event type
- **WHEN** user preferences have custom notification format for permission requests
  - **then** go backend uses user's custom format for permission request notifications
  - **then** IM Bridge receives and displays custom-formatted permission request

### Requirement: Go backend provides event forwarding configuration
The Go backend SHALL provide configuration for which Bridge event types to forward to IM platforms, default timeout, and format mappings per event type.

 and whether to include detailed diagnostics.

#### Scenario: Default configuration forwards all event types
- **WHEN** Go backend starts with default configuration
- **THEN** all defined event types (permission_request, budget_alert, agent_status_change, tool_installed) are forwarded to IM

- **THEN** events are formatted according to their respective templates before forwarding

#### Scenario: Custom event type configuration
- **WHEN** admin configures custom event types to forward (e.g., `mcp_server_added`, `plugin_crashed`)
  - **THEN** Go backend adds these event types to forwarding configuration
  - **THEN** Bridge events of these new types are forwarded to IM with appropriate formatting
