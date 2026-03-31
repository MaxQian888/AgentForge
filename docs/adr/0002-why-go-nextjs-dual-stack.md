# ADR-0002: Why Go Plus Next.js / 为什么使用 Go + Next.js 双栈

- Status: Accepted
- Date: 2026-03-31
- Owners: AgentForge maintainers

## Context

AgentForge needs a responsive operator UI, strong backend control-plane logic,
database-backed state, WebSocket fan-out, scheduler loops, and orchestration of
AI runtimes. A single-stack implementation would either weaken the backend
systems programming side or the dashboard/UI iteration speed.

## Decision

Keep a dual-stack architecture:

- Next.js 16 + React 19 for the dashboard and auth UI
- Go for the orchestrator, persistence, scheduling, realtime hub, plugin control plane, and worktree-aware execution control

## Consequences

- backend and frontend can evolve independently while sharing HTTP/WebSocket contracts
- the repository needs explicit DTO and runtime-catalog coordination across the language boundary
- build, test, and docs must stay honest about which surface owns which behavior
