# im-runtime-status-commands Specification

## Purpose
Enable IM commands to query Bridge runtime pool status, health, and available runtimes for operational visibility and monitoring from chat platforms.

## ADDED Requirements

### Requirement: Show Bridge pool status in agent list
The IM Bridge `/agent list` command SHALL display Bridge runtime pool status alongside Go agent pool status, showing active runtimes, capacity, and availability.

#### Scenario: Agent list shows combined status
- **WHEN** user sends `/agent list` via IM platform
- **THEN** IM Bridge calls both `GET /api/v1/agents` (Go) and `GET /api/v1/bridge/pool` (Bridge proxy)
- **THEN** IM Bridge displays combined view:
  - Go agent pool: active agents, max capacity
  - Bridge runtime pool: active runtimes, available slots

#### Scenario: Agent list with Bridge unavailable
- **WHEN** user sends `/agent list` and Bridge is unavailable
- **THEN** IM Bridge displays Go agent pool status
- **THEN** IM Bridge displays "Bridge status: unavailable" indicator
- **THEN** IM Bridge logs Bridge unavailability for monitoring

#### Scenario: Agent list with only Bridge status
- **WHEN** user sends `/agent list` and no Go agents are active
- **THEN** IM Bridge displays "No active agents" for Go pool
- **THEN** IM Bridge displays Bridge runtime pool status

### Requirement: Query available Bridge runtimes
The IM Bridge SHALL provide `/agent runtimes` command to display all available Bridge runtimes with their capabilities, providers, and default models.

#### Scenario: List available runtimes
- **WHEN** user sends `/agent runtimes` via IM platform
- **THEN** IM Bridge calls `GET /api/v1/bridge/runtimes`
- **THEN** Go backend proxies to `GET http://localhost:7778/bridge/runtimes`
- **THEN** IM Bridge displays runtime list:
  - claude_code: Claude Code CLI, Anthropic, claude-sonnet-4-5, available
  - codex: Codex CLI, OpenAI, gpt-4o, available
  - opencode: OpenCode Runtime, Anthropic, claude-opus-4-5, available

#### Scenario: Runtimes with diagnostics
- **WHEN** user sends `/agent runtimes` and one runtime has diagnostic warnings
- **THEN** IM Bridge displays runtime list with diagnostic indicators
- **THEN** IM Bridge highlights runtime with issues

#### Scenario: Runtimes command with Bridge unavailable
- **WHEN** user sends `/agent runtimes` and Bridge is unavailable
- **THEN** IM Bridge displays error "Unable to fetch runtimes: Bridge unavailable"
- **THEN** IM Bridge suggests checking Bridge health with `/agent health`

### Requirement: Check Bridge health status
The IM Bridge SHALL provide `/agent health` command to check Bridge health, including service status, MCP server connections, and recent errors.

#### Scenario: Bridge is healthy
- **WHEN** user sends `/agent health` via IM platform
- **THEN** IM Bridge calls `GET /api/v1/bridge/health`
- **THEN** Go backend proxies to `GET http://localhost:7778/bridge/health`
- **THEN** IM Bridge displays:
  - Status: healthy
  - Uptime: 2d 14h 32m
  - MCP servers: 3 connected, 0 disconnected
  - Active runtimes: 5
  - Memory usage: 1.2GB

#### Scenario: Bridge is degraded
- **WHEN** user sends `/agent health` and Bridge reports degraded status
- **THEN** IM Bridge displays:
  - Status: degraded
  - Issues:
    - MCP server "github" disconnected
    - 2 recent errors in last hour
  - Recommendation: Check Bridge logs

#### Scenario: Bridge is unavailable
- **WHEN** user sends `/agent health` and Bridge is unreachable
- **THEN** IM Bridge displays error "Bridge unavailable - connection refused"
- **THEN** IM Bridge suggests:
  - Check if Bridge process is running
  - Check Bridge port configuration
  - Review Bridge startup logs

#### Scenario: Health check with detailed diagnostics
- **WHEN** user sends `/agent health --verbose` via IM platform
- **THEN** IM Bridge displays full health report including:
  - Service status
  - MCP server details with connection status
  - Recent errors with timestamps
  - Performance metrics

### Requirement: Go backend exposes Bridge pool endpoint
The Go backend SHALL expose `GET /api/v1/bridge/pool` endpoint that proxies to TS Bridge `/bridge/pool` for authentication.

#### Scenario: Fetch pool status
- **WHEN** authenticated client calls `GET /api/v1/bridge/pool`
- **THEN** Go backend validates authentication
- **THEN** Go backend calls `GET http://localhost:7778/bridge/pool`
- **THEN** Go backend returns pool status:
  ```json
  {
    "active_runtimes": 5,
    "max_capacity": 10,
    "available_slots": 5,
    "runtimes": [
      {"task_id": "task-1", "runtime": "claude_code", "status": "running", "age_seconds": 3600}
    ]
  }
  ```

#### Scenario: Pool endpoint when Bridge unavailable
- **WHEN** authenticated client calls `GET /api/v1/bridge/pool` and Bridge is unavailable
- **THEN** Go backend returns 503 with `{ "error": "Bridge unavailable" }`

### Requirement: Go backend exposes Bridge health endpoint
The Go backend SHALL expose `GET /api/v1/bridge/health` endpoint that proxies to TS Bridge `/bridge/health` with authentication.

#### Scenario: Fetch health status
- **WHEN** authenticated client calls `GET /api/v1/bridge/health`
- **THEN** Go backend validates authentication
- **THEN** Go backend calls `GET http://localhost:7778/bridge/health`
- **THEN** Go backend returns health status with MCP server details

#### Scenario: Health endpoint with detailed diagnostics
- **WHEN** authenticated client calls `GET /api/v1/bridge/health` and Bridge reports degraded status
- **THEN** Go backend returns health status with diagnostic information
- **THEN** Response includes MCP server connection status and recent errors

### Requirement: Go backend exposes Bridge runtimes endpoint
The Go backend SHALL expose `GET /api/v1/bridge/runtimes` endpoint that proxies to TS Bridge `/bridge/runtimes` with authentication and caching.

#### Scenario: Fetch runtime catalog
- **WHEN** authenticated client calls `GET /api/v1/bridge/runtimes`
- **THEN** Go backend validates authentication
- **THEN** Go backend checks cache (60s TTL)
- **THEN** Go backend calls `GET http://localhost:7778/bridge/runtimes` if cache miss
- **THEN** Go backend returns runtime catalog with capabilities

#### Scenario: Runtimes endpoint uses cache
- **WHEN** authenticated client calls `GET /api/v1/bridge/runtimes` within 60s of previous call
- **THEN** Go backend returns cached runtime catalog
- **THEN** No call to Bridge is made

#### Scenario: Cache expires after 60 seconds
- **WHEN** authenticated client calls `GET /api/v1/bridge/runtimes` after 60s cache TTL
- **THEN** Go backend calls Bridge to refresh cache
- **THEN** Go backend returns fresh runtime catalog
