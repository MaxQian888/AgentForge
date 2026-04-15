# Agent API / Agent 运行时 API

This document describes the agent-run control plane plus the adjacent bridge
health/runtime endpoints.

## Overview

Primary surfaces:

- agent lifecycle under `/api/v1/agents`
- pool and runtime health under `/api/v1/agents/pool` and `/api/v1/bridge/*`
- lightweight bridge AI utilities under `/api/v1/ai/*`

## Spawn Request

`POST /api/v1/agents/spawn` accepts:

```json
{
  "taskId": "task-uuid",
  "memberId": "optional-member-uuid",
  "runtime": "claude_code",
  "provider": "anthropic",
  "model": "claude-sonnet-4-5",
  "roleId": "coding-agent",
  "maxBudgetUsd": 5
}
```

Behavior:

- validates `taskId`
- optionally validates `memberId`
- validates the effective role binding before queue admission or runtime startup
  - explicit `roleId` is checked directly
  - if `roleId` is omitted, dispatch-driven flows may derive it from the target agent member's saved `agentConfig.roleId`
- may return a queued dispatch outcome instead of a direct run if the
  dispatcher is enabled
- performs Bridge health retries before runtime execution when the Go service
  path owns the spawn
- may queue early when the Bridge runtime pool is already at capacity
- refuses to start when the bridge is degraded
- refuses to queue or start when the effective role binding is stale

## Endpoint Summary

| Method | Path | Purpose |
| --- | --- | --- |
| `POST` | `/api/v1/agents/spawn` | Start or queue an agent run |
| `GET` | `/api/v1/agents` | List active/recent agent summaries |
| `GET` | `/api/v1/agents/:id` | Get one agent summary |
| `POST` | `/api/v1/agents/:id/pause` | Pause a run |
| `POST` | `/api/v1/agents/:id/resume` | Resume a paused run |
| `POST` | `/api/v1/agents/:id/kill` | Cancel a run |
| `GET` | `/api/v1/agents/:id/logs` | Read normalized agent logs |
| `GET` | `/api/v1/agents/pool` | Return pool capacity and queue summary |
| `GET` | `/api/v1/bridge/health` | Return TS bridge health |
| `GET` | `/api/v1/bridge/pool` | Return TS bridge runtime pool summary |
| `GET` | `/api/v1/bridge/runtimes` | Return runtime catalog and diagnostics |
| `GET` | `/api/v1/bridge/tools` | List installed Bridge tools/plugins |
| `POST` | `/api/v1/bridge/tools/install` | Install a Bridge tool/plugin from an allowed manifest |
| `POST` | `/api/v1/bridge/tools/uninstall` | Uninstall a Bridge tool/plugin |
| `POST` | `/api/v1/bridge/tools/:id/restart` | Restart a Bridge tool/plugin |
| `POST` | `/api/v1/ai/generate` | Run bridge-backed generation |
| `POST` | `/api/v1/ai/classify-intent` | Run bridge-backed intent classification |
| `GET` | `/api/v1/roles/:id/references` | Inspect blocking and advisory downstream role consumers |

## `AgentRunSummaryDTO`

Key fields:

- `id`, `taskId`, `taskTitle`
- `memberId`, `roleId`, `roleName`
- `status`
- `runtime`, `provider`, `model`
- `inputTokens`, `outputTokens`, `cacheReadTokens`
- `costUsd`, `budgetUsd`, `turnCount`
- `worktreePath`, `branchName`, `sessionId`
- `lastActivityAt`, `startedAt`, `completedAt`
- `canResume`, `memoryStatus`
- `teamId`, `teamRole`

## Pool Stats

`GET /api/v1/agents/pool` returns:

- `active`
- `max`
- `available`
- `pausedResumable`
- `queued`
- `warm`
- `degraded`
- optional `queue[]`

`GET /api/v1/bridge/pool` returns Bridge-side runtime capacity fields such as:

- `active`
- `max`
- `warmTotal`
- `warmAvailable`
- `degraded`

## Logs

`GET /api/v1/agents/:id/logs` returns normalized log entries:

```json
{
  "timestamp": "2026-03-31T12:34:56Z",
  "content": "tool result or agent output",
  "type": "output | tool_call | tool_result | error | status"
}
```

## Bridge Runtime Catalog

`GET /api/v1/bridge/runtimes` feeds the dashboard settings/runtime-selection UI.
Each runtime entry can include diagnostics such as:

- missing credentials
- missing executables
- sunset-window or runtime-sunset notices
- incompatible provider/runtime combinations

The runtime tuple is:

- `runtime`
- `provider`
- `model`

The catalog can also include:

- `interactionCapabilities`
- `launchContract`
- `lifecycle`

For CLI-backed runtimes, these fields let the frontend distinguish documented
headless launch semantics, supported approval modes, additional-directory
support, and deprecation or migration guidance before launch.

## Bridge Tool Management

`GET /api/v1/bridge/tools` returns:

```json
{
  "tools": [
    {
      "plugin_id": "web-search",
      "name": "search",
      "description": "Search repositories and docs"
    }
  ]
}
```

`POST /api/v1/bridge/tools/install` request body:

```json
{
  "manifest_url": "https://registry.example.com/web-search.yaml"
}
```

`POST /api/v1/bridge/tools/uninstall` request body:

```json
{
  "plugin_id": "web-search"
}
```

`POST /api/v1/bridge/tools/:id/restart` does not require a body when the plugin
id is present in the route.

Notes:

- install manifest URLs must pass the backend allowlist
- install/uninstall/restart proxy directly to the TS Bridge lifecycle surface
- IM `/tools *` commands and `/agent spawn` tool summaries both read from this
  same API

## Typical Failure Modes

- `400 Bad Request`: malformed JSON or invalid UUID
- `404 Not Found`: task, member, or run missing
- `409 Conflict`: pool full, run already active, worktree unavailable
- `422 Unprocessable Entity`: stale or invalid member-bound role configuration when the current flow validates structured agent profile input
- `502 Bad Gateway`: bridge/runtime start failure
- `503 Service Unavailable`: bridge unavailable (`{"error":"bridge_unavailable"}`)

For dispatch-driven launch flows, a stale effective role binding now shows up as a blocked dispatch outcome with:

- `guardrailType=target`
- `guardrailScope=role`
- a reason explaining that the bound role no longer resolves from the authoritative role registry
