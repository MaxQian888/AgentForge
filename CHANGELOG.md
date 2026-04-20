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
