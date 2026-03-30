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
- Coding-agent runtime catalog support across `claude_code`, `codex`, and `opencode`
- Repo-local plugin authoring flow with `create-plugin`, `plugin:build`,
  `plugin:debug`, `plugin:dev`, `plugin:verify`, and
  `plugin:verify:builtins`
- Desktop window chrome, sidecar supervision, updater-manifest generation, and
  release-time updater artifact validation
- Modular CI with root quality/tests, Go CI, bridge typecheck, IM bridge build,
  desktop build matrix, and layered PR review automation

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
