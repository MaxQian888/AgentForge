## 1. Scheduler foundations

- [ ] 1.1 Add scheduler domain models, persistence schema, and repositories for scheduled jobs plus append-only run history.
- [ ] 1.2 Implement a Go-side scheduler registry that materializes the built-in job catalog with stable job keys, validated schedules, enable state, overlap policy, and latest-run metadata.
- [ ] 1.3 Add scheduler service logic for due-time calculation, singleton run acquisition, run finalization, and manual-trigger orchestration.

## 2. Control plane APIs and realtime observability

- [ ] 2.1 Add authenticated scheduler API endpoints to list jobs, inspect run history, enable or disable a job, update supported schedule settings, and trigger manual runs.
- [ ] 2.2 Emit scheduler lifecycle and run-result WebSocket events so management clients can react to start, success, failure, and enable-state changes without polling everything.
- [ ] 2.3 Add frontend store and management UI for scheduler operations, including job list, last/next run, failure summaries, and manual rerun controls.

## 3. Built-in job integrations

- [ ] 3.1 Migrate the task progress detector from the ad hoc server ticker into the scheduler registry while preserving current warning, stalled, recovery, and deduplication behavior.
- [ ] 3.2 Promote worktree stale-state inspection and garbage collection into a registered scheduler job while keeping startup repair and manual cleanup semantics aligned.
- [ ] 3.3 Add built-in scheduler handlers for bridge health reconciliation and cost reconciliation so they run through the same registry, run-history, and event model as other system jobs.

## 4. Desktop and local scheduling with Bun

- [ ] 4.1 Add a Bun-based scheduler adapter in `src-bridge` or a closely related local runtime entrypoint that reconciles OS-level job registration using `Bun.cron` for desktop/local deployments.
- [ ] 4.2 Ensure Bun-triggered executions call back into the canonical Go scheduler execution path instead of bypassing Go-owned business logic or run-history recording.
- [ ] 4.3 Add reconcile and cleanup behavior so desktop/local schedule registrations are updated or removed when job configuration, enable state, or app mode changes.

## 5. Verification and rollout hardening

- [ ] 5.1 Add focused backend tests for scheduler registry validation, singleton overlap handling, run-history persistence, and built-in job execution summaries.
- [ ] 5.2 Add frontend and integration coverage for scheduler management APIs, realtime updates, and manual-trigger workflows.
- [ ] 5.3 Run targeted verification for Go scheduler flows and Bun adapter behavior, then document deployment-specific scheduler behavior for server, Docker, and desktop modes.
