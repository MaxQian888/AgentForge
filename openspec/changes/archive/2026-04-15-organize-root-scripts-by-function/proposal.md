## Why

The root `scripts/` directory now mixes backend build helpers, development orchestration, plugin authoring flows, internal skill governance, updater tooling, i18n audits, and smoke helpers in one flat namespace. Because those scripts are referenced from `package.json`, plugin sub-packages, GitHub workflows, tests, docs, and script-to-script imports, even a simple cleanup is easy to do incompletely unless the repository defines one canonical functional layout and migrates every caller with it.

## What Changes

- Reorganize the root `scripts/` directory into function-oriented subdirectories instead of keeping unrelated automation in one flat folder.
- Define the canonical destination for each current script family, including shared helpers, fixtures, and shell wrappers that are owned by those families.
- Update every supported caller to the new script paths, including root package commands, plugin package commands, GitHub workflows, repository docs, tests, fixtures, and script-to-script imports.
- Preserve the current supported command behaviors while changing the physical layout, so maintainers do not have to rediscover how to build, verify, or run existing workflows after the reorganization.
- Add or refresh verification where needed so stale legacy paths or partially migrated callers are caught deterministically.

## Capabilities

### New Capabilities
- `root-script-organization`: Organize root repository automation scripts by functional domain and keep all supported callers synchronized with the canonical locations.

### Modified Capabilities

## Impact

- Affected code: root `scripts/**`, root `package.json`, plugin-local `package.json` files under `plugins/**`, related Jest tests, shell wrappers, and any script-owned fixtures or helper modules.
- Affected automation: GitHub workflows and release/build entrypoints that call `node scripts/...` directly.
- Affected documentation: README files, CI/deployment docs, plugin development docs, and any design or runbook content that documents canonical script paths.
- Dependencies/systems: No new runtime dependency is required, but the change depends on preserving repo-supported command semantics across Node, pnpm, and GitHub Actions environments.
