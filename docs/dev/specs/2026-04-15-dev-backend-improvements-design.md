# Dev Backend Improvements Design

**Date:** 2026-04-15
**Status:** Approved

## Goal

Comprehensively improve `pnpm dev:backend` to fix Windows process management, support no-Docker environments, add Go hot-reload via air, and improve overall developer experience.

## 1. Windows Process Management

### Problem
`process.kill()` cannot kill detached process trees on Windows. After `pnpm dev:backend:stop`, Go/Bun child processes remain, occupying ports.

### Solution
- Replace `process.kill(pid)` with `taskkill /T /F /PID` on Windows to kill entire process tree
- After killing, verify port is released; if not, find residual PID via `netstat` and kill again
- Add graceful shutdown attempt before force kill (SIGTERM equivalent timeout)

### Files
- `scripts/dev/dev-workflow.js` — new `killProcessTree()` function
- `scripts/dev/dev-all.js` — update `stopManagedServiceProcesses()`

## 2. No-Docker Degradation

### Problem
When Docker Desktop is unavailable, `pnpm dev:backend` fails completely even if PG/Redis are installed natively.

### Solution
- Before attempting docker-compose, probe whether PG (5432) and Redis (6379) are already listening
- If already running → reuse directly, skip docker entirely
- If not running → attempt docker-compose (existing logic)
- If docker unavailable → for Redis, mark degraded and continue (Go backend already handles this); for PG, emit clear error with install instructions
- New infra service kind: `probe-or-compose` — probe first, compose as fallback

### Files
- `scripts/dev/dev-all.js` — update `ensureInfrastructure()` with probe-first logic

## 3. Air Hot-Reload Integration

### Problem
Go backend requires manual restart after code changes.

### Solution
- Add `.air.toml` config to `src-go/`
- In service definitions, detect `air` availability: if found, use `air` instead of `go run ./cmd/server`
- Environment variables pass through to air subprocess
- Fallback to `go run` if air is not installed
- Add `pnpm dev:backend:watch` alias that sets `PREFER_AIR=1`

### Files
- `src-go/.air.toml` — air configuration
- `scripts/dev/dev-all.js` — conditional air detection in service definitions

## 4. Single-Service Restart

### Problem
No way to restart one service without stopping everything.

### Solution
- New command: `pnpm dev:backend restart <service-name>`
- Stop only the named service (kill process, release port)
- Re-launch with same config
- Validate health before reporting success

### Files
- `scripts/dev/dev-all.js` — new `runWorkflowRestart()` function, update `main()` CLI parser

## 5. Enhanced Output & Diagnostics

### Colored Status Output
- Green for ready/healthy, red for failed/unhealthy, yellow for degraded/reused
- Use ANSI escape codes with TTY detection fallback

### Error Diagnostics
- On startup failure, auto-read last 20 lines of stderr log and display inline
- Show which process holds a conflicting port (via netstat/lsof)

### Environment Pre-check
- Display Go version, Bun version at startup
- Warn if versions are below minimum (Go 1.22+, Bun 1.0+)

### Files
- `scripts/dev/dev-workflow.js` — color helpers, version checks
- `scripts/dev/dev-all.js` — update all `print*()` functions

## Implementation Order

1. Windows process management (foundational fix)
2. No-Docker degradation (unblocks more environments)
3. Air hot-reload (developer productivity)
4. Single-service restart (convenience)
5. Enhanced output & diagnostics (polish)
