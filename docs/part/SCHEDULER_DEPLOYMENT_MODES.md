# Scheduler Deployment Modes

AgentForge now keeps Go as the canonical scheduler authority in every deployment mode. The `scheduled_jobs` table remains the source of truth for job definitions, enablement, schedules, run summaries, and the websocket-visible lifecycle state. What changes across deployment modes is only how cron triggers are produced.

## Server mode

- Default backend behavior keeps `SCHEDULER_EXECUTION_MODE=in_process`.
- Go's in-process scheduler loop evaluates due jobs and executes handlers directly.
- Bridge-side `Bun.cron` registration is not required, and the bridge scheduler adapter will remove any stale OS registrations if it is started with a non-desktop mode.

This is the expected mode for ordinary web deployments and for local backend-only development.

## Docker or container mode

- Container deployments use the same `in_process` execution mode as server mode unless explicitly overridden.
- Scheduler truth, run history, and websocket events stay inside the Go runtime.
- `Bun.cron` is intentionally not part of the container deployment path, so there is no second scheduler authority inside the bridge image.

If a containerized bridge process does start with `BRIDGE_SCHEDULER_MODE=server` or `container`, it will reconcile by removing prior OS registrations instead of creating new ones.

## Desktop mode

- Tauri now launches the backend sidecar with `SCHEDULER_EXECUTION_MODE=os_registered`.
- Tauri launches the bridge sidecar with `BRIDGE_SCHEDULER_MODE=desktop`.
- The bridge scheduler adapter polls `GET /internal/scheduler/jobs`, selects enabled jobs whose `executionMode` is `os_registered`, and reconciles OS-level registrations through `Bun.cron`.
- Each registered Bun cron entry runs a generated worker script that calls `POST /internal/scheduler/jobs/:jobKey/trigger`, so the effective execution still flows through the canonical Go scheduler service and persists the same run history records.

Desktop shutdown stops the bridge-side reconcile loop, but it does not proactively delete OS-level registrations. That keeps persistent desktop scheduling aligned with Bun's OS-backed cron model instead of collapsing back to app-lifetime-only timers.

## Local Bun sidecar mode

For local sidecar testing outside Tauri, use the same split as desktop mode:

- backend: `SCHEDULER_EXECUTION_MODE=os_registered`
- bridge: `BRIDGE_SCHEDULER_MODE=local`

That configuration exercises the same Bun adapter codepath without requiring the packaged desktop shell.

## Operational notes

- Job config changes and enable/disable state changes are picked up by the bridge scheduler adapter during its reconcile polling loop.
- When a job changes schedule, the adapter removes the prior OS registration and installs a fresh one with the updated cron expression.
- When a job is disabled or switched back to `in_process`, the adapter removes the matching OS registration and generated worker file.
- Manual operator triggers and Bun-triggered runs converge on the same Go scheduler service, so UI state, run history, and websocket events remain deployment-neutral.
