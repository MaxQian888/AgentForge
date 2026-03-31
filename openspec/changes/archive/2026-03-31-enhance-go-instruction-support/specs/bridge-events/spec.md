# Bridge Events Specification

## ADDED Requirements

### Requirement: TS bridge events are transported to Go over the internal bridge websocket
The system SHALL accept bridge-originated agent events over the internal websocket used by the TypeScript runtime. The Go bridge handler MUST decode JSON event payloads, ignore malformed payloads without crashing the connection loop, and forward valid events to the configured bridge event processor.

#### Scenario: Forward valid bridge event to processor
- **WHEN** the bridge websocket receives a valid serialized agent event payload
- **THEN** the bridge handler decodes the payload into a bridge event
- **THEN** the handler forwards that event to the configured Go processor

#### Scenario: Ignore malformed bridge payload
- **WHEN** the bridge websocket receives invalid JSON
- **THEN** the handler logs the invalid payload condition
- **THEN** the websocket loop continues processing later events

### Requirement: TS bridge websocket transport provides ready, buffering, reconnect, and heartbeat behavior
The TypeScript bridge event streamer SHALL emit a ready status signal on websocket connect, buffer outbound events while disconnected, flush buffered events when the socket reopens, and emit application-level heartbeat events while connected.

#### Scenario: Emit ready status on connect
- **WHEN** the TypeScript bridge websocket opens
- **THEN** the event streamer emits a `status_change` event indicating the bridge is ready

#### Scenario: Buffer events while disconnected
- **WHEN** the bridge attempts to send agent events while the websocket is not open
- **THEN** the event streamer stores those events in its bounded ring buffer

#### Scenario: Flush buffered events after reconnect
- **WHEN** the websocket becomes open again
- **THEN** the event streamer flushes buffered events before sending newly queued events

#### Scenario: Emit heartbeat event while connected
- **WHEN** the websocket remains open
- **THEN** the event streamer emits periodic heartbeat events containing bridge health and MCP server status data

### Requirement: Go currently projects runtime output, cost, and terminal status events back into orchestration state
The Go bridge event processor SHALL currently recognize runtime output events, cost update events, and terminal status change events for active runs.

#### Scenario: Process runtime output event
- **WHEN** Go receives a bridge `output` event with non-empty content for an active run
- **THEN** the processor broadcasts agent output to clients
- **THEN** the processor updates task progress and bridge activity state

#### Scenario: Process runtime cost update event
- **WHEN** Go receives a bridge `cost_update` event for an active run
- **THEN** the processor updates run token usage, run cost, and task spent budget state

#### Scenario: Process terminal status change event
- **WHEN** Go receives a bridge `status_change` event whose new status maps to a terminal agent run state
- **THEN** the processor updates the run lifecycle status and performs terminal cleanup behavior

### Requirement: Bridge-side advanced event types are schema-preserved but not yet fully projected in Go
The TypeScript bridge serializer SHALL preserve newer agent event payload shapes such as reasoning, todo updates, progress, rate limits, partial messages, and permission requests. The Go bridge processor does not yet guarantee orchestration-side handling for those advanced event types through this capability.

#### Scenario: Preserve advanced event payload shape during serialization
- **WHEN** the TypeScript bridge serializes a reasoning, todo update, progress, rate limit, partial message, or permission request event
- **THEN** the serialized websocket payload preserves the event type and payload shape for transport
