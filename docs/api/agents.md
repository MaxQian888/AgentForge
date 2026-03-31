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
- may return a queued dispatch outcome instead of a direct run if the
  dispatcher is enabled
- refuses to start when the bridge is degraded

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
| `GET` | `/api/v1/bridge/runtimes` | Return runtime catalog and diagnostics |
| `POST` | `/api/v1/ai/generate` | Run bridge-backed generation |
| `POST` | `/api/v1/ai/classify-intent` | Run bridge-backed intent classification |

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
- incompatible provider/runtime combinations

The runtime tuple is:

- `runtime`
- `provider`
- `model`

## Typical Failure Modes

- `400 Bad Request`: malformed JSON or invalid UUID
- `404 Not Found`: task, member, or run missing
- `409 Conflict`: pool full, run already active, worktree unavailable
- `502 Bad Gateway`: bridge/runtime start failure
- `503 Service Unavailable`: bridge unavailable (`{"error":"bridge_unavailable"}`)
