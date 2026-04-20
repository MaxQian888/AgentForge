## 1. Schema and model

- [x] 1.1 Add migration `src-go/migrations/0NN_workflow_trigger_target_kind.up.sql` + `.down.sql` adding `target_kind TEXT NOT NULL DEFAULT 'dag'` with CHECK constraint restricting to the enum `dag | plugin`
- [x] 1.2 Extend `model.WorkflowTrigger` with `TargetKind string` (or typed alias) and update JSON tag / DTO mapping
- [x] 1.3 Extend `repository/workflow_trigger_repo.go` to read/write `target_kind`, include it in all list/query paths, and add filter helpers (`ListEnabledBySourceAndKind`) where needed
- [x] 1.4 Update `workflow_trigger_repo_test.go` and `workflow_trigger_repo_integration_test.go` to cover both target kinds

## 2. Dispatch abstraction

- [x] 2.1 Define `trigger.TargetEngine` interface and `trigger.TriggerRun` return type in `src-go/internal/trigger/router.go` (or a new `engines.go` sibling)
- [x] 2.2 Implement `trigger.DAGEngineAdapter` wrapping `*service.DAGWorkflowService.StartExecution`, preserving existing `StartOptions.Seed` and `TriggeredBy` behavior
- [x] 2.3 Add a minimal exported `StartTriggered(ctx, pluginID, seed, triggerID)` seam on the workflow plugin runtime service (no new execution semantics — only an entry-point for trigger-initiated starts)
- [x] 2.4 Implement `trigger.PluginEngineAdapter` wrapping that seam and returning a `TriggerRun{Engine:"plugin", RunID:<workflow_plugin_run.ID>}`
- [x] 2.5 Refactor `trigger.Router` to hold `map[string]TargetEngine`, look up the adapter by `trigger.TargetKind`, and surface unknown-kind errors as structured non-success outcomes
- [x] 2.6 Add/adjust `trigger/router_test.go` to cover: DAG target, plugin target, unknown target kind, idempotency collision across kinds, input-mapping parity

## 3. Registrar validation

- [x] 3.1 Extend `trigger/registrar.go` sync logic to accept `target_kind` on incoming trigger configs (accept both kinds; remove the single-engine assumption from the validation branch at line 88)
- [x] 3.2 When syncing, resolve the referenced workflow against the declared engine (DAG definitions repo for `dag`; workflow plugin registry for `plugin`)
- [x] 3.3 Persist unresolvable targets as `enabled=false` with a structured `disabled_reason`; surface that reason in the sync response
- [x] 3.4 Add `registrar_test.go` cases covering both target kinds and each disabled-reason path

## 4. Schedule and IM dispatch paths

- [x] 4.1 Update `trigger/schedule_ticker.go` so the cron tick dispatcher fans out events through the unified router without caring about target engine
- [x] 4.2 Update `handler/trigger_handler.go` so `/api/v1/triggers/im/events` returns outcome metadata including `target_kind` and engine run id
- [x] 4.3 Add integration test that boots both the DAG service and the plugin runtime, registers two triggers matching the same IM command (one DAG, one plugin), and asserts a single IM event fires both engines exactly once

## 5. API + frontend surface

- [x] 5.1 Extend workflow trigger create/update REST payloads on `workflow_handler.go` (and any service-side validation) to accept optional `targetKind`, defaulting to `dag`
- [x] 5.2 Ensure list/read DTOs expose `targetKind` and `disabledReason`
- [x] 5.3 Update `lib/stores/workflow-trigger-store.ts` types and CRUD calls to round-trip `targetKind` and surface `disabledReason`
- [x] 5.4 Update `components/workflow/workflow-triggers-section.tsx` to render `targetKind` (selector + read-only badge) and to surface a disabled-reason warning
- [x] 5.5 Add or extend `lib/stores/workflow-trigger-store.test.ts` to cover both target kinds

## 6. Spec + verification

- [x] 6.1 Ensure the new capability spec at `openspec/specs/workflow-trigger-dispatch/spec.md` is merged from the change deltas at archive time (no code change, only confirmation)
- [x] 6.2 Run `go test ./internal/trigger/... ./internal/handler/... ./internal/repository/...` targeted to touched packages and record any unrelated pre-existing failures explicitly
- [x] 6.3 Run `pnpm test` scoped to `workflow-trigger-store` and `workflow-triggers-section`
- [ ] 6.4 Smoke-verify manually in `pnpm dev:all`: create one DAG-target trigger and one plugin-target trigger, send an IM event via the fixture in `src-im-bridge/commands/workflow.go`, confirm both engines produce a run
