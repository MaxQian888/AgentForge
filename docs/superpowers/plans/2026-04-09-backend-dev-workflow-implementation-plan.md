# Backend Dev Workflow Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add backend-only local dev commands and let the full-stack workflow reuse a separately started backend stack without duplicate ownership.

**Architecture:** Generalize the existing `scripts/dev-all.js` control plane into profile-aware workflow helpers so `all` and `backend` modes share startup, status, stop, logs, and health semantics. Keep ownership isolation by giving each workflow profile its own runtime state file while reusing the same repo-local log directory and service health probes.

**Tech Stack:** Node.js scripts, Jest node-environment tests, repo-local runtime state under `.codex/`, existing Docker/Go/Bun/Next.js startup commands.

---

## Chunk 1: Lock the workflow contract with tests

**Files:**
- Modify: `scripts/dev-all.test.ts`

- [ ] Add a failing test for the backend-only service matrix and `dev-backend-state.json`.
- [ ] Run the focused Jest command and confirm the new assertions fail for the current implementation.
- [ ] Add a failing test for the root `dev:backend*` command family in `package.json`.
- [ ] Add a failing test proving `dev:all` reuses healthy backend services and only needs to start the frontend when the backend stack is already up.
- [ ] Run the focused Jest command again and confirm the expected failures are now about missing backend workflow support.

## Chunk 2: Implement profile-aware workflow support

**Files:**
- Modify: `scripts/dev-all.js`
- Modify: `package.json`

- [ ] Add profile metadata for `all` and `backend`, including state file names, human-readable labels, and the included services.
- [ ] Refactor path/service-definition/start/status/stop/log helpers so they can run for either profile without changing existing `dev:all` behavior.
- [ ] Add backend-only wrappers/CLI handling and export the backend helpers needed by tests.
- [ ] Expose `dev:backend`, `dev:backend:status`, `dev:backend:stop`, and `dev:backend:logs` in `package.json`.
- [ ] Run the focused Jest command and confirm the new tests pass.

## Chunk 3: Sync docs and verify

**Files:**
- Modify: `README.md`
- Modify: `README_zh.md`

- [ ] Update the local development workflow docs to describe the backend-only command family and its service scope.
- [ ] Document that `pnpm dev:all` can reuse a healthy backend stack started via `pnpm dev:backend`.
- [ ] Run the focused Jest command for `scripts/dev-all.test.ts`.
- [ ] Run a targeted lint command for the touched files.
- [ ] Review the final diff to ensure only the intended workflow/docs surfaces changed.
