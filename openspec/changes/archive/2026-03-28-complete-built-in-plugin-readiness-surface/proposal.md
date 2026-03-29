## Why

AgentForge has already defined and shipped an official built-in plugin bundle, but the current operator experience still stops at discovery. Built-in entries expose only coarse availability text, so operators can see that a plugin exists yet still have no structured path to understand whether it is ready, what prerequisite is missing, which configuration is required, or what step should be taken next before activation can succeed.

## What Changes

- Add a built-in plugin readiness contract that turns bundle metadata into machine-readable prerequisite, configuration, and next-step guidance instead of relying on a single free-form availability message.
- Extend built-in discovery and catalog responses so the Go control plane evaluates and returns current readiness state, blocking reasons, and setup guidance for each official built-in plugin.
- Extend the plugin management panel so built-in entries and installed built-ins expose readiness badges, setup guidance, docs links, and truthful action gating for install and activation paths.
- Add focused verification for readiness metadata drift and deterministic prerequisite/configuration preflight checks so official built-ins do not regress back into visible-but-not-actionable entries.
- Keep built-in asset inventory, remote marketplace behavior, and plugin runtime architecture unchanged; this change only closes the readiness and setup surface for the built-ins that already ship with the repo.

## Capabilities

### New Capabilities
- `built-in-plugin-readiness`: define the operator-facing readiness contract for official built-in plugins, including prerequisite checks, configuration requirements, blocking reasons, and next-step guidance.

### Modified Capabilities
- `built-in-plugin-bundle`: change bundle requirements so official built-ins declare structured readiness metadata instead of only coarse availability strings.
- `plugin-catalog-feed`: change catalog and discovery requirements so official built-in entries return evaluated readiness state and setup guidance from the control plane.
- `plugin-management-panel`: change panel requirements so operators can inspect built-in readiness, understand blocked activation or install states, and follow setup guidance without leaving the console.
- `plugin-development-scripts`: change verification requirements so repo-owned checks validate built-in readiness metadata and bounded prerequisite preflight behavior.

## Impact

- Affected metadata and repo assets: `plugins/builtin-bundle.yaml` and any readiness-related docs references or verification profile declarations for official built-ins.
- Affected backend and DTO seams: `src-go/internal/model/plugin.go`, `src-go/internal/service/plugin_service.go`, `src-go/internal/handler/plugin_handler.go`, and the built-in discovery or catalog response shapes consumed by the frontend.
- Affected frontend seams: `lib/stores/plugin-store.ts`, `app/(dashboard)/plugins/page.tsx`, related plugin detail components, and tests that currently assume built-ins only expose static availability messaging.
- Affected verification flows: `scripts/verify-built-in-plugin-bundle.*`, built-in readiness fixtures, and any targeted checks that confirm prerequisite or configuration guidance remains truthful.
