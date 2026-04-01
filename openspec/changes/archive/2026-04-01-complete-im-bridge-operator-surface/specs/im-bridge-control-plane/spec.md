## ADDED Requirements

### Requirement: Bridge status snapshot SHALL expose operator-facing runtime summary

`GET /api/v1/im/bridge/status` SHALL return an operator-oriented IM Bridge snapshot in addition to basic liveness. The snapshot MUST include overall health, per-provider transport and capability data, pending delivery counts, recent delivery summary, rolling aggregate counters, and last-known provider diagnostics metadata when available.

#### Scenario: Status snapshot includes backlog and recent delivery health
- **WHEN** one Feishu bridge has pending deliveries and recent fallback or failure activity
- **THEN** `GET /api/v1/im/bridge/status` includes that provider's pending count, last settled delivery timestamp, recent failure or fallback summary, and the aggregate pending/error counters for the operator console

#### Scenario: Status snapshot tolerates missing diagnostics metadata
- **WHEN** a registered provider has not reported optional diagnostics metadata
- **THEN** the status endpoint still returns the provider entry successfully
- **THEN** the diagnostics field is marked unavailable instead of failing the entire snapshot

### Requirement: Control-plane delivery settlement SHALL be operator-truthful

A delivery queued for a live IM Bridge SHALL be recorded as `pending` until the bridge reports a terminal settlement. The bridge settlement payload MUST carry terminal status, processed timestamp, and optional failure or downgrade reason so operator history, queue depth, and latency metrics reflect actual delivery outcomes instead of optimistic queue acceptance.

#### Scenario: Successful settlement updates a pending delivery
- **WHEN** the backend queues delivery `d1` and the bridge later settles `d1` with status `delivered`
- **THEN** `d1` is removed from the pending backlog, marked `delivered` in history, and assigned a processed timestamp and latency derived from queue time to settlement time

#### Scenario: Failed settlement remains visible in operator history
- **WHEN** the bridge settles delivery `d2` with status `failed` and failure reason `rate_limit`
- **THEN** the history record for `d2` is marked `failed`
- **THEN** the failure reason is persisted and included in subsequent operator snapshot and history responses

#### Scenario: Unsettled delivery stays pending
- **WHEN** the backend queues delivery `d3` and no terminal settlement has been reported yet
- **THEN** `d3` remains `pending` in the operator snapshot and history
- **THEN** `d3` is not counted as delivered for success-rate or latency metrics

### Requirement: Bridge registration and heartbeat SHALL support optional diagnostics refresh

Bridge registration and heartbeat flows SHALL allow an instance to refresh optional operator diagnostics metadata, including transport warnings, callback health, quota summaries, or last transport error snapshots. The backend MUST store the latest diagnostics per bridge instance and expose them through the operator status snapshot.

#### Scenario: Heartbeat refreshes diagnostics metadata
- **WHEN** a bridge heartbeat reports webhook health `healthy` and quota summary metadata
- **THEN** the backend stores the latest diagnostics for that bridge instance
- **THEN** the next operator status snapshot exposes those diagnostics on the matching provider card
