## ADDED Requirements

### Requirement: Go backend performs startup readiness probe against TS Bridge
Go backend SHALL check TS Bridge availability at startup by calling `GET /bridge/health` with retry (up to 10 attempts, 2s interval). If the Bridge is not reachable after retries, the backend SHALL start in degraded mode with agent-related endpoints returning HTTP 503.

#### Scenario: Bridge available at startup
- **WHEN** Go backend starts and Bridge responds to `/bridge/health` with HTTP 200
- **THEN** backend marks Bridge status as `ready` and enables all agent endpoints

#### Scenario: Bridge unavailable at startup
- **WHEN** Go backend starts and Bridge does not respond after 10 retry attempts
- **THEN** backend marks Bridge status as `degraded` and agent spawn/pause/resume endpoints return HTTP 503 with `{"error": "bridge_unavailable"}`

### Requirement: Go backend performs periodic health checks against TS Bridge
Go backend SHALL call `GET /bridge/health` every 30 seconds to monitor Bridge availability. Health status transitions SHALL be logged and exposed via API.

#### Scenario: Health check succeeds after degraded state
- **WHEN** Bridge was in `degraded` state and health check returns HTTP 200
- **THEN** backend transitions Bridge status to `ready` and re-enables agent endpoints

#### Scenario: Health check fails after ready state
- **WHEN** Bridge was in `ready` state and 3 consecutive health checks fail
- **THEN** backend transitions Bridge status to `degraded`, logs warning, and agent endpoints return 503

### Requirement: Bridge health status is exposed via Go API endpoint
Go backend SHALL expose `GET /api/v1/bridge/health` returning Bridge health status, last check timestamp, and basic Bridge pool summary.

#### Scenario: Frontend queries bridge health
- **WHEN** authenticated client calls `GET /api/v1/bridge/health`
- **THEN** response contains `{"status": "ready"|"degraded", "last_check": "<ISO timestamp>", "pool": {"active": N, "available": N, "warm": N}}`

#### Scenario: Unauthenticated request
- **WHEN** unauthenticated client calls `GET /api/v1/bridge/health`
- **THEN** response is HTTP 401
