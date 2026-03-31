# bridge-health-probe Specification

## Purpose
Define how the Go backend probes TypeScript Bridge readiness, propagates degraded state to agent-facing APIs, and exposes bridge health details to authenticated clients.
## Requirements
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

### Requirement: Bridge 测试矩阵文档
TESTING.md SHALL 新增 Bridge 测试矩阵章节，以表格形式展示：Bridge 类型 × 测试维度（单元测试、集成测试、E2E 测试），标注每种组合的覆盖状态和运行命令。

#### Scenario: 查看当前 Bridge 测试覆盖情况
- **WHEN** 开发者需要了解哪些 Bridge 已有测试覆盖
- **THEN** 测试矩阵表格清晰展示各 Bridge 的测试状态和运行命令

### Requirement: 覆盖率提升指南
TESTING.md SHALL 新增覆盖率提升指南，包含：当前各模块覆盖率数据、未覆盖场景识别方法、测试编写建议（优先级排序）。

#### Scenario: 为低覆盖模块添加测试
- **WHEN** 开发者需要提高某个模块的测试覆盖率
- **THEN** 指南提供优先级排序和测试编写模板

