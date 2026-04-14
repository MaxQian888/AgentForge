# Backend Dev Workflow Design

## Goal

Add a backend-only local development command family for AgentForge and make the existing full-stack workflow safely reuse a separately started backend stack.

## Scope

- Add root commands for backend-only startup, status, stop, and logs
- Keep the backend-only scope to PostgreSQL, Redis, Go Orchestrator, TS Bridge, and IM Bridge
- Preserve `pnpm dev:all` as the full-stack entrypoint, with frontend added on top when backend services are already healthy
- Keep runtime state and stop ownership isolated between the full-stack and backend-only command families

## Current Repo Truth

- `scripts/dev-all.js` already owns cross-platform startup, health probing, runtime logs, and state persistence for the full local stack
- `scripts/dev-workflow.js` already provides shared helpers for runtime state, stop planning, command availability, and Docker Desktop readiness
- `scripts/dev-all.test.ts` already verifies service matrices, package script exposure, reuse behavior, conflict handling, and stop semantics
- `README.md` and `README_zh.md` document `pnpm dev:all` plus a manual backend-only path, but there is no root backend-only command family yet

## Design

1. Keep a single Node control plane in `scripts/dev-all.js`, but add workflow profiles so the same implementation can serve both `all` and `backend` modes.
2. Expose the backend command family from `package.json` as:
   - `pnpm dev:backend`
   - `pnpm dev:backend:status`
   - `pnpm dev:backend:stop`
   - `pnpm dev:backend:logs`
3. Give each workflow profile its own state file:
   - full stack: `.codex/dev-all-state.json`
   - backend-only: `.codex/dev-backend-state.json`
4. Reuse the existing `.codex/runtime-logs/` directory so log discovery stays consistent across both profiles.
5. Treat healthy services outside the current workflow state file as reusable instead of managed, so `dev:all` can layer the frontend on top of an already running backend stack without taking ownership of those backend processes.

## Validation

- Jest proves the backend profile exposes the expected service matrix and state file path.
- Jest proves the new root `dev:backend*` scripts exist.
- Jest proves `dev:all` reuses healthy backend services and only needs to start the frontend when the backend stack is already running.
- README and README_zh describe the new backend-only command family and its relationship to `dev:all`.
