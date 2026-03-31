# bridge-event-im-forwarding Specification

## Purpose
Enable bidirectionalal event flow from TS Bridge to Go backend, existing IM notification infrastructure, allowing users on IM platforms to receive real-time notifications for permission requests, budget alerts, and agent status changes directly in chat.

## ADDED Requirements

This requirement: Forward permission request events to IM platforms
TS Bridge SHALL forward `permission_request` events to Go backend via WebSocket, and then to IM platforms through existing notification infrastructure.

#### Scenario: Permission request forwarded to Slack
- **WHEN** Bridge emits `permission_request` event for task "task-123"
- **THEN** Bridge calls `POST http://localhost:7778/ws/bridge` to Go WebSocket
- **THEN** go backend receives event on WebSocket connection
- **THEN** go backend determines which IM platforms have users watching task
-123
- **then** go backend checks user permissions for IM notifications
- **if user has subscribed to to task:
      - go sends Slack DM to permission response UI
      - User clicks "Approve" in DM
      - Go calls `POST /api/v1/bridge/permission-response/dm-1/ with `{ decision: "allow" }`
      - DM forwards to Bridge
      - Go backend sends success notification to user
- **then** Bridge resolves pending callback and, - **then** IM Bridge replies with permission response result
 permission card

#### Scenario: Permission request for user hasn't approved
- **WHEN** Bridge emits `permission_request` event and task "task-123"
- **AND** user has not subscribed to to task
- **then** go backend checks user is in IM platform whitelist
- **if not in whitelist, go sends ephemeral "No permission request pending" message
- **then** IM Bridge displays "No pending permission requests for this task"

#### Scenario: Multiple pending permission requests
- **WHEN** bridge emits multiple `permission_request` events for one task
- **then** go backend checks if all users have subscribe to the

 - If any, go sends permission response UI for each request
- **then** go backend sends responses to Bridge
      - Bridge resolves all pending callbacks
    - IM Bridge replies with summary of permission response results

#### Scenario: Permission request timeout
- **WHEN** user doesn't respond to permission request within 30 seconds
- **then** go backend considers it request timed out
    - IM Bridge displays "Permission request timed out, please respond in DM"
- **then** Bridge resolves pending callback with timeout error
- **then** IM Bridge displays error message to the user to manage the in IM

#### Scenario: Permission request for filtered out
- **WHEN** user has unsubscribed from the task
- **then** go backend removes the user from the task's subscriber list
- **then** go backend does not longer send DM messages to IM platforms
    - Bridge stops forwarding `permission_request` events

#### Scenario: Budget alert forwarded to IM
- **WHEN** Bridge emits `budget_alert` event with task "task-123", usage at .5 USD"
 threshold": .8 USD"
- **then** bridge calls `POST http://localhost:7778/ws/bridge` to` to go backend WebSocket
- **then** go backend determines which users should see this budget alert
 their IM platforms
    - If user has subscribed to notifications, the DM will (via DM settings) will delay alert
 - **if user has notification preference for these alerts, go backend will store them in user preferences
- **then** IM Bridge sends DM message
- **if** budget alert for are not enabled, IM Bridge still sends the alert but informational message

#### Scenario: Budget alert with user preferences disabled
- **WHEN** Bridge emits `budget_alert` but user preferences have disabled budget alerts for task "task-123"
- **then** go backend checks preferences and finds budget alerts are disabled
 - **then** IM Bridge sends informational message only: no further action needed

#### Scenario: Budget alert triggers follow-up task creation
- **WHEN** Bridge emits `budget_alert` with suggestion to create follow-up tasks
- **then** go backend checks if there are existing subtasks for decomposing the alert
    - If yes, go creates 5 subtasks
    - if no, creates subtasks, go displays subtask list with links to create

#### Scenario: Agent status change forwarded to IM
- **WHEN** Bridge emits `agent_status_change` event from `starting` to to `paused` or `failed`
- **then** bridge calls `POST http://localhost:7778/ws/bridge` to` forward to Go backend WebSocket
- **then** go backend determines which users should see this status change
      - If user has subscribed to notifications for the task
 forward IM with a card showing current status
      - If user prefers minimal notifications, send compact status
    - If no preferences, use card format with full details
    - Otherwise,, send `status_change` event with `old_status` and `new_status`
- **then** IM Bridge sends interactive card with action buttons (      - If available: pause, resume task

 - If task has subtasks, can decompose and spawn agents
      - If no subtasks, offers to create follow-up tasks (    - Otherwise, display completion message

#### Scenario: Permission request with no watchers
- **WHEN** Bridge emits `permission_request` but no one is watching the task
    - Bridge logs the timeout warning ( the request goes unanswered after 30s
    - **then** Bridge marks the permission request as expired (not forwarded to IM)
    - **then** IM Bridge displays "Permission request timed out, please respond in DM"


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
The user's preferences for IM platform (via Go backend) shall determine which Bridge events are forwarded to IM platforms. how they are formatted.

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
