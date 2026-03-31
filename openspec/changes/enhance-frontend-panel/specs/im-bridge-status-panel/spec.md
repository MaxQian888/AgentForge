## ADDED Requirements

### Requirement: IM bridge status displays connection health

The system SHALL display real-time connection status for each configured IM bridge (Feishu, WeCom, QQ, DingTalk).

#### Scenario: User views bridge status
- **WHEN** user navigates to IM bridge status page
- **THEN** system displays status card for each configured IM platform
- **AND** each card shows connection state (connected, disconnected, degraded) with visual indicator

#### Scenario: Bridge is connected
- **WHEN** IM bridge has active connection
- **THEN** status card displays green "Connected" badge
- **AND** shows last successful heartbeat timestamp

#### Scenario: Bridge connection is lost
- **WHEN** IM bridge loses connection to platform
- **THEN** status card displays red "Disconnected" badge
- **AND** shows error message and last known state

### Requirement: IM bridge status shows message queue metrics

The system SHALL display pending message queue size, processing rate, and error count for each bridge.

#### Scenario: User views queue metrics
- **WHEN** bridge status panel is displayed
- **THEN** system shows queue depth, messages per minute, and error count
- **AND** metrics update every 5 seconds

#### Scenario: Queue is backed up
- **WHEN** pending queue exceeds threshold (100 messages)
- **THEN** queue metric displays warning indicator
- **AND** shows estimated drain time

### Requirement: IM bridge status enables retry controls

The system SHALL allow users to manually retry failed messages or clear the retry queue.

#### Scenario: User retries failed message
- **WHEN** user clicks "Retry" on a failed message entry
- **THEN** system re-queues the message for delivery
- **AND** message status updates to "pending"

#### Scenario: User clears retry queue
- **WHEN** user clicks "Clear All" in retry queue section
- **THEN** system displays confirmation dialog
- **AND** confirming removes all pending retries

### Requirement: IM bridge status displays recent activity log

The system SHALL show a scrollable log of recent message deliveries, failures, and system events.

#### Scenario: User views activity log
- **WHEN** user scrolls to activity log section
- **THEN** system displays chronological list of recent events
- **AND** each entry shows timestamp, event type, message ID, and status

#### Scenario: User filters activity log
- **WHEN** user selects "Failures Only" filter
- **THEN** log displays only failed delivery attempts
- **AND** filter badge shows count of matching entries

### Requirement: IM bridge status shows platform-specific diagnostics

The system SHALL display platform-specific health indicators (API rate limits, webhook status, certificate validity).

#### Scenario: User views Feishu diagnostics
- **WHEN** user expands Feishu bridge diagnostics
- **THEN** system shows app verification status, API quota remaining, and webhook endpoint status
- **AND** indicates any configuration issues

#### Scenario: Rate limit is approaching
- **WHEN** platform API rate limit is at 90% usage
- **THEN** diagnostics display warning indicator
- **AND** shows time until quota reset

### Requirement: IM bridge status enables bridge configuration

The system SHALL provide quick access to bridge configuration settings from the status panel.

#### Scenario: User accesses bridge settings
- **WHEN** user clicks "Configure" button on a bridge card
- **THEN** system navigates to settings page with IM bridge section focused
- **AND** specific bridge configuration is pre-selected

### Requirement: IM bridge status supports connection testing

The system SHALL allow users to send test messages to verify bridge functionality.

#### Scenario: User sends test message
- **WHEN** user clicks "Send Test" and provides target recipient
- **THEN** system sends test message through the bridge
- **AND** displays success or failure result with timing

#### Scenario: Test message fails
- **WHEN** test message delivery fails
- **THEN** system displays error details and suggested remediation
- **AND** logs the failure for debugging

### Requirement: IM bridge status displays aggregate metrics

The system SHALL show summary metrics for all bridges (total messages, success rate, average latency).

#### Scenario: User views aggregate metrics
- **WHEN** IM bridge status page loads
- **THEN** system displays summary cards with total messages sent, overall success rate, and average delivery latency
- **AND** metrics reflect last 24 hours by default
