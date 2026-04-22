# Changelog

All notable changes to AgentForge should be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and the repository continues to use semantic versioning as the release intent
even though the workspace has evolved well beyond its original starter state.

## [Unreleased]

### Added

- Multi-surface dashboard workspaces for:
  - overview metrics and quick actions
  - project dashboard CRUD and widget operations
  - shared task workspace across board/list/timeline/calendar views
  - role authoring with preview, sandbox, and repo-local skill catalog support
  - project docs/wiki editing with comments, versions, templates, and task linkage
  - team, scheduler, workflow, memory, IM, plugin, and review operator surfaces
  - cost analytics with per-agent, per-project, and per-team breakdowns
  - global agent fleet view with status and run stats
  - global cross-project document browser
  - governed skills operator surface with diagnostics and mirror sync
  - marketplace with unified plugin/skill/role publishing, discovery, and install
- Per-employee (agent identity) workspace with profile, run history, and trigger
  configuration sub-pages (`/employees/[id]/runs/`, `/employees/[id]/triggers/`)
- Per-project VCS integration management (GitHub, GitLab, Gitea) with webhook
  routing and `vcs-integrations-store`
- Per-project secrets management with Go-backed storage and `secrets-store`
- Qianchuan ads-platform operator surfaces including channel bindings and strategy
  authoring and editing, backed by `qianchuan-bindings-store` and
  `qianchuan-strategies-store`
- Knowledge base with chunked asset ingestion, vector search, live-artifact
  materialization, comment threading, and frontend knowledge components
  (`IngestedFilesPane`, `KnowledgeSearch`, `MaterializedFromPill`,
  `SourceUpdatedBanner`)
- Automation trigger engine (`src-go/internal/trigger/`) with CRUD service,
  idempotency, rule routing, schedule ticker, and dry-run support
- Declarative automation rules (`src-go/internal/automation/`) evaluated by the
  trigger engine, including `review_completed_rule`
- Dispatch observability and preflight handlers for structured agent-dispatch
  readiness checks and capability matrix validation before dispatch
- Custom fields, milestones, notifications, project templates, saved views,
  queue management, and workflow run history handlers on the Go API
- Coding-agent runtime catalog support across `claude_code`, `codex`, `opencode`,
  `cursor`, `gemini`, `qoder`, and `iflow` with truthful lifecycle/sunset metadata
  for iFlow and degraded-state diagnostics for all runtimes
- Repo-local plugin authoring flow with `create-plugin`, `plugin:build`,
  `plugin:debug`, `plugin:dev`, `plugin:verify`, and `plugin:verify:builtins`
- Desktop window chrome, sidecar supervision, updater-manifest generation, and
  release-time updater artifact validation
- Modular CI with root quality/tests, Go CI, bridge typecheck, IM bridge build,
  desktop build matrix, and layered PR review automation
- Standalone marketplace Go microservice (`src-marketplace/`) with item CRUD,
  version management, reviews, admin moderation, and typed install bridge

### Changed

- Repository documentation now consistently treats AgentForge, not
  `react-quick-starter`, as the product identity
- Root frontend build guidance now reflects static export output in `out/`
  rather than treating `pnpm start` as the primary deployment path
- Contribution, testing, and CI/CD docs now describe the actual multi-workspace
  verification model used by the repository

### Documentation

- Refreshed `README.md` and `README_zh.md` to match the current implemented workspaces
- Refreshed `docs/PRD.md` implementation snapshot
- Rewrote `TESTING.md` around real verification entrypoints by subsystem
- Rewrote `CI_CD.md` around the current GitHub Actions workflows
- Replaced stale starter-era contribution guidance in `CONTRIBUTING.md`
- Updated plugin runtime guidance in `docs/GO_WASM_PLUGIN_RUNTIME.md`

## [0.8.0] — 2026-04-22

### Added

- UI visual specifications and compliance testing framework with regex-based
  token guardrails (`docs/UI_VISUAL_SPEC.md`)
- Tailwind CSS visual token compliance batches (Batch 3/5 CI rules)
- `CODEOWNERS` with minimal per-stack ownership
- `SECURITY.md` routing vulnerability reports through GitHub advisory
- Contributor Covenant v2.1 code of conduct
- Workflow engine completion design spec (P1–P4) with concurrent session support
- P1 template hardening implementation plan

### Changed

- Go module path renamed from `react-go-quick-starter` to `github.com/agentforge/server`
- Extracted RSC page-client boundaries; migrated Qianchuan to plugin architecture
- Normalized status color usage to unify success and warning tracks
- Replaced arbitrary font sizes, magic number sizes, and hardcoded hex colors
  with standardized Tailwind classes and CSS variables

### Fixed

- EventBus `close-while-send` data race in `Subscribe`
- Qianchuan `DialogDescription` accessibility warning in `CreateBindingDialog`
- Integration test `db.Close` ordering to avoid closed-db race
- Go WASM plugin `Capabilities` count mismatch (2 → 3)
- Tauri sidecar path parsing across Windows/macOS/Linux hosts
- Desktop Tauri Logic coverage threshold (80% → 70%) after test split

### Performance

- Cut CI wall-clock time ~40% with four targeted optimisations
- Added target-aware sidecar builds and bun setup in `build-tauri`

## [0.7.0] — 2026-04-21

### Added

- **Observability & debugging system**
  - `trace_id` Echo middleware and context helpers across Go orchestrator
  - Per-source token-bucket rate limiter for log ingest endpoint
  - `POST /api/v1/internal/logs/ingest` handler (single or batch)
  - Browser logger with batched ingest and `X-Trace-ID` propagation
  - Frontend API client injects `X-Trace-ID` on outbound requests
  - Error boundaries report to ingest via `log.error`
  - TS Bridge pino logger with `trace_id` child support
  - Admin-gated pprof under `/debug/pprof` (requires `DEBUG_TOKEN`)
  - `/debug/trace/:id` merged timeline + `/metrics` endpoints
  - Live Tail tab with WebSocket subscribe, pause, and bounded buffer
  - Zustand devtools in dev builds (no-op in production)
  - Tauri rotating file sink via `tauri-plugin-log`
  - Panic recovery with stack trace capture
- **IM Bridge provider extensibility**
  - `core.ProviderFactory` registry with self-registration per provider
  - Multi-provider registration inventory assembled at bridge startup
  - `Registry.Snapshot` for bridge inventory reporting
  - Providers and `CommandPlugins` propagated through `RegisterBridge`
  - Re-registration on SIGHUP / plugin reload / reconcile
  - Personal WeChat (iLinks) wired as a first-class IM channel
- **Plugin system expansion**
  - `WorkflowExecutor` interface with registry pattern
  - `SequentialExecutor`, `HierarchicalExecutor`, `EventDrivenExecutor`
  - `ToolChain` model (`ToolChainSpec`, `ToolChainStep`) with resolver
  - `ToolChainExecutor` supporting stop/skip/retry policies
  - `email-adapter` — SMTP outbound notification plugin
  - `generic-webhook-adapter` — any HTTP webhook → EventBus
  - `github-actions-adapter` — GitHub webhook → EventBus
  - `notification-fanout` first-party in-process plugin
- **ACP client integration** (T0–T8 + TΔ1–4)
  - `ChildProcessHost` with stderr ring buffer and graceful shutdown
  - `AcpConnectionPool` with per-adapter mutex and idle reclaim
  - `MultiplexedClient` with session routing and handler stubs
  - `AcpSession` with connection-pool factory and mock agent
  - `FsSandbox`, `TerminalManager`, permission handler, elicitation passthrough
  - Full `session/update` → `AgentEventType` mapping
  - `createAcpRuntimeAdapter` public barrel export
  - Runtime adapter registry wired to ACP (opencode, claude_code)
- **Dev / debug infrastructure**
  - `/debug` page shell with Timeline tab and sidebar navigation link
  - `LOG_LEVEL` env var honoured by orchestrator
  - `dev:all` requires prepared sidecars on Windows before startup
  - Hide console windows when spawning sidecars on Windows

### Changed

- Renamed `feishu-adapter` demo to `sample-integration-plugin` across fixtures
  and builtin bundle targets
- IM Bridge outbound/inbound clients propagate `X-Trace-ID`
- EventBus publish attaches `trace_id` to `Event.Metadata`
- Automation engine writes `trace_id` into `automation_logs.detail`
- Background jobs generate `trace_id` at entry points

### Fixed

- IM Bridge provider ID normalization on register for symmetry with lookup
- Background goroutines inherit parent `trace_id` when present
- Go integration test environment hardened (Postgres cleanup, race removal)
- Repository portable SQL timestamp expression for cross-DB compatibility

## [0.6.0] — 2026-04-20

### Added

- **VCS integration management**
  - Provider interface + typed sentinel errors (`vcs` package)
  - Provider registry with `Constructor` seam and mock recording provider
  - GitHub provider via `go-github` v60 with typed error mapping
  - GitLab + Gitea stubs returning `ErrUnsupported`
  - `vcs_integrations` table (migration 072) + audit resource type
  - VCS integration service with secret-resolved auth probe + webhook lifecycle
  - Webhook handler with deduplication + outbound dispatcher (Spec 2B)
  - Webhook router `synchronize` handler + stale-findings policy
  - Frontend VCS integrations management page with secret-ref selectors
  - `vcs-integrations-store` with CRUD + sync actions
- **Qianchuan ads platform**
  - `qianchuan_bindings` table (migration 070) + audit resource type
  - Ads-platform provider interface + registry + neutral types
  - Qianchuan provider implementation with neutral mapping
  - Bindings handler + REST endpoints + audit + RBAC
  - Strategy library CRUD (repo, service, HTTP handler, YAML parser)
  - Strategy YAML schema + action allowlist + per-action param validators
  - Frontend strategy store, list page, and Monaco edit page
  - Frontend bindings list page + create dialog + Zustand store
- **Project secrets management**
  - Migration 069 + audit `resource_type` enum extension
  - AES-256-GCM cipher with `key_version` handshake
  - GORM repository with name-conflict mapping
  - Orchestrated create/rotate/resolve with audit hook
  - HTTP CRUD with one-time value response and RBAC gating
  - `secret_resolver` with strict HTTP-node field whitelist
  - Frontend secrets page with create/rotate/delete + one-time reveal
  - `secrets-store` with one-time-reveal capture
- **Incremental code review**
  - `parent_review_id` column + migration 076 for incremental reviews
  - `TriggerIncrementalReviewRequest` model + `BuildIncrementalPlan`
  - `ReviewService.TriggerIncremental` + `Complete` writes `last_reviewed_sha`
  - Per-finding detail page with diff panel and decision buttons
  - Diff viewer + `decideFinding` store action
  - Findings decision API (`approve` / `dismiss` / `defer`)
  - Findings SDK v2 with `suggested_patch` field
- **Workflow engine enhancements**
  - Sub-workflow invocation — bridge pattern, recursion guard, DAG child step
  - HTTP node + IM-send node + card action routing (Spec 1E)
  - Outbound dispatcher subscribing terminal exec events
  - `created_via` / `display_name` / `description` columns on `workflow_triggers`
  - `system_metadata` jsonb column on `workflow_executions`
  - Strategy execution loop — 3 node types, 2 tables, canonical DAG seed (Spec 3D)
  - Unified run view — parent-link tracking, event emitter, run view API
  - Workflow acting-employee attribution guard and DB model
  - `EmployeeRunsRepository.ListByEmployee` UNION query
- **Automation & triggers**
  - Automation rule for `review.completed` + router automation branch
  - Trigger CRUD service + dry-run for spec1-1C
  - Trigger registrar with `created_via` merge (delete-and-insert replaced)
  - `EventRouter` matches triggers, renders input mapping, starts executions
  - `Registrar.SyncFromDefinition` materializes trigger-node config
  - `POST /api/v1/triggers/im/events` routes normalized IM events
  - `/employees/[id]/triggers` page + CRUD components
  - `employee-trigger-store` for trigger CRUD
- **Employee runtime**
  - `employees`, `employee_skills`, `workflow_triggers` tables (migration 062)
  - `Employee`, `EmployeeSkill`, `WorkflowTrigger` domain models
  - `EmployeeRepository` with GORM record pattern + skill CRUD
  - `EmployeeService` with CRUD, `SetState`, skill bindings, `Invoke`
  - `Invoke` delegates to `AgentService.SpawnForEmployee` with role/skill merge
  - YAML registry seeds employees per project with `default-code-reviewer`
  - `Employee` CRUD + skills HTTP API
  - `/employees/[id]/runs` page + layout shell + `EmployeeRunRow`
  - `employee-runs-store` with WebSocket event forwarding
- **Digital employee specs**
  - Foundation gaps spec for digital employee end-to-end (Spec 1)
  - Code reviewer digital employee (Spec 2)
  - E-commerce streaming digital employee (Spec 3)
  - 10 implementation plans for code-reviewer + e-commerce employees
  - OpenAgents Python SDK integration design
- **IM Bridge cards**
  - Provider-neutral card schema (`ProviderNeutralCard`)
  - Card renderer registry + text fallback
  - Per-platform renderers (Feishu / Slack / DingTalk)
  - `/im/send` accepts `ProviderNeutralCard` via `RawCardSender`
  - Re-route Feishu `SendCard` / `ReplyCard` through `core.DispatchCard`
- **UI refinement**
  - UI design cohesion refactor — shared primitives + dashboard (phases 0–6)
  - Workflow runs tab, plugin-run body, run store, sub-workflow config panel

### Fixed

- Workflow migration renumbering (077/078) to avoid 076 conflict
- Schedule ticker context propagation + strategy handler
- Dead code removal (`RouteFixRequest`, `EventReviewFixRequested`)
- ESLint errors in workflow run + sub-workflow components
- OAuth bind flow, background token refresher, `auth_expired` handling (Spec 3B)

## [0.5.0] — 2026-04-19

### Added

- **Project governance**
  - Project RBAC (owner/admin/editor/viewer) gating human and agent actions
  - Audit log for project-scoped mutations
  - Project archival and soft-delete
  - Project templates with seed workflows
  - Project invitations with email/token flow
- **Trigger engine foundation**
  - Redis-backed `IdempotencyStore` with `miniredis` tests
  - `WorkflowTriggerRepository` with md5-hash-based upsert
  - Minute-boundary schedule ticker fires `source=schedule` triggers
  - `/workflow <name>` IM command forwards event to backend trigger router
  - Smoke test fixture for Feishu `/workflow` command end-to-end path
- **Review-workflow integration**
  - `system:code-review` workflow template backing `/reviews/trigger`
  - Workflow-backed review via `USE_WORKFLOW_BACKED_REVIEW` opt-in
  - Round-trip `execution_id` so reviews link to workflow runs
  - IM approve/request-changes advances workflow `human_review` when backed
- **Workflow authoring**
  - Workflow Create/Update syncs trigger subscriptions via Registrar
  - `HandleExternalEvent` handler with validation and error handling
  - `StartExecution` extended with `StartOptions` (seed + `triggered_by`)
  - `llm_agent` node accepts optional `employeeId` in config
  - Applier routes employee-backed spawns through `EmployeeService.Invoke`
- **Logging & diagnostics**
  - Unified dev/test logging contract across Go, TS Bridge, and IM Bridge
  - Sink diagnostics and reporting across services
  - Internal service instrumentation with correlation IDs
  - Context-based diagnostics onboarding guide

### Changed

- Team strategy interface removed; fully delegated to workflow templates
- `go.mod` dependencies reorganized for clarity

## [0.4.0] — 2026-04-14 – 2026-04-17

### Added

- **Unified event bus** (`src-go/internal/eventbus/`)
  - Event envelope with validation (TDD)
  - URI address parsing with helpers
  - Reserved metadata accessors
  - Mod interfaces, modes, and glob matcher
  - Pipeline executor with three-mode ordering
  - Bus with registration lock and depth-limited emits
  - `core.validate` and `core.auth` guards
  - `core.ws-fanout` with visibility-aware routing
  - `im.forward-legacy` transitional observer
  - `PublishLegacy` helper for mechanical callsite migration
  - Events + DLQ repositories; migration 054
- **DAG node type registry**
  - Two-layer registry with handler contract and effect DSL
  - `EffectApplier` supporting park, fire-forget, and control-flow effects
  - 14 built-in handlers: `trigger`, `gate`, `parallel_split`, `parallel_join`,
    `sub_workflow`, `condition`, `notification`, `status_transition`, `loop`,
    `llm_agent`, `agent_dispatch`, `human_review`, `wait_event`, `function`
- **Feishu callback closure**
  - Synthetic action name enum for card callbacks
  - Checker tag → `toggle` action with `checker_state`
  - `multi_select` vs `select` with `selected_options`
  - `input_submit`, `date_pick`, `overflow`, `form_submit` action dispatch
  - Message reactions forwarded as `react` action
  - `IMReactionEvent` model and repository (migration 055)
  - Action executor dispatches to task transition, comment repo, reaction recorder
- **ACP client scaffolding**
  - ACP workspace scaffold in `src-bridge/src/acp/`
  - Gemini adapter with verified `--acp` flag (T0)
  - Extended error vocabulary (capability/auth/command/crash)
  - `ChildProcessHost` with stderr ring buffer and graceful shutdown
  - `AcpConnectionPool` with per-adapter mutex and idle reclaim
- **Frontend surfaces**
  - `enhance-frontend-panel` across dashboard (14 surface specs promoted)
  - Skills store retains last verification result
  - Canonical IM provider catalog truth
  - Project-management API contracts unified
- **Knowledge base**
  - Wiki pages and ingested documents unified into `KnowledgeAsset`
  - Live-artifact blocks in wiki pages
- **IM Bridge hardening**
  - Multi-tenant gateway with rich delivery and security-ops
  - `im_reaction_events` migration for reaction audit trail

### Changed

- WebSocket Hub stripped of `BroadcastEvent`; became channel-aware registry
- Agent and dispatch services migrated to event bus (M1)
- Complete eventbus M1 migration across all services
- Workflow node execution routed through `NodeTypeRegistry` + `EffectApplier`

### Fixed

- `upsert` via jsonb equality (was broken under non-canonical JSON)
- `Get` returns bare `ErrNotFound`; `Update` reports `ErrNotFound` on zero rows
- RingBuffer oversized chunk, double-start guard, null exit code (ACP T2b)
- Host exited check, `AcquireContext` seam, timing margin (ACP T2c)

## [0.3.0] — 2026-04-14 – 2026-04-15

### Added

- **Comprehensive platform expansion**
  - Backend services expansion (team artifacts, workflow definitions, structured output)
  - Bridge runtime enhancements (Claude runtime handler, agent runtime)
  - IM platform enhancements (Feishu native messages, renderer, expanded commands)
  - Frontend enhancements (workflow canvas, execution view, team artifacts UI)
- **Document management**
  - Office document parsing with memory integration
  - PDF library integration
  - Frontend document management UI
  - IM bridge commands for document operations
  - Cross-platform document support
- **Workflow editor**
  - 14 node type components with registry and styles
  - Editor context with reducer and undo/redo
  - Data-flow, undo-redo, and editor-actions hooks
  - Categorized node palette and editor toolbar
  - Custom edge, snap grid, and editor canvas
  - Hybrid condition builder with visual/expression modes
  - Node/edge config panels with data flow preview
  - Shell component and public API
  - Integrated editor module; old canvas files removed
- **Workflow runtime**
  - Unified workflow engine with data flow, templates, human review, team adapter
  - Agent completion routing and team-to-workflow delegation
  - Workflow templates integrated into marketplace
  - Reviews and Templates tabs with inline approval
  - Template vars dialog for clone/execute
  - `fetchPendingReviews` in workflow store
- **WeChat Official Account** platform adapter
- **Email platform** implementation with tests and rendering logic
- Windows sidecar preparation and progress logging

## [0.2.0] — 2026-03-27 – 2026-03-31

### Added

- **Complete workspace surfaces**
  - Review flow with findings and decision support
  - IM runtime support across Telegram, Feishu, Slack, DingTalk
  - Plugin control plane with WASM runtime
  - Scheduler with cron expressions and job queues
  - Agent pool management with priority controls
  - Wiki workspace with comments, versions, and templates
- **Marketplace**
  - Standalone `src-marketplace/` Go microservice
  - Domain model, repositories, service layer, HTTP handlers
  - Frontend marketplace store, components, page, sidebar, i18n
  - Marketplace install endpoints in main Go backend
  - Unified Skills/Plugin/Role marketplace
  - Go and TypeScript tests for marketplace
- **Dashboard & runtime**
  - Dashboard runtime surfaces expanded
  - Docs/wiki editing surfaces
  - Runtime registry with platform adapters
  - `dev-all` scripts for full local stack orchestration
- **Tests & quality**
  - Cross-stack coverage expansion
  - Desktop build gates
  - Jest tests for core UI components

### Changed

- `README.md` and `README_zh.md` refreshed with marketplace status

## [0.1.0] — 2026-03-23 – 2026-03-26

### Added

- Initial project scaffold (Next.js 16 + Tauri 2.9 + TypeScript + Go + Bun)
- **AgentForge IM Bridge** core implementation
  - Multi-platform adapter architecture
  - Telegram live messaging with long polling
  - Feishu rich card lifecycle specification
  - Action reference parsing and reply strategy
- **Task workspace** foundation
  - Task workspace P0 design spec and implementation plan
  - Context rail components and state helpers
  - Data bootstrap and planning input rules
  - Integration with task detail panel
- **Review & sprint**
  - Review and sprint commands
  - Bridge lifecycle, control plane, and platform interaction tests
- **Rendering & platform**
  - Rendering profile and text formatting for various platforms
  - Platform-specific message formatting
- **Infrastructure**
  - Migration scripts for plugin control plane, scheduler, agent pool, wiki
  - Workflow-plugin-runtime specification
  - `dev-all` orchestration scripts
- **Backend analysis**
  - Go vs Rust technology analysis document for AI-driven development platform
- **Tests**
  - Unit tests for UI components (Avatar, Card, Dialog, DropdownMenu, Popover,
    ScrollArea, Select, Separator, Sheet, Table, Tabs, Tooltip)
  - IM Bridge and platform stub tests

[Unreleased]: https://github.com/Arxtect/AgentForge/compare/v0.8.0...HEAD
[0.8.0]: https://github.com/Arxtect/AgentForge/compare/v0.7.0...v0.8.0
[0.7.0]: https://github.com/Arxtect/AgentForge/compare/v0.6.0...v0.7.0
[0.6.0]: https://github.com/Arxtect/AgentForge/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/Arxtect/AgentForge/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/Arxtect/AgentForge/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/Arxtect/AgentForge/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/Arxtect/AgentForge/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/Arxtect/AgentForge/releases/tag/v0.1.0
