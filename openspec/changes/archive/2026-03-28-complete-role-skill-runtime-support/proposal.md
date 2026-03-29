## Why

`docs/PRD.md` and `docs/part/PLUGIN_SYSTEM_DESIGN.md` define Skill-Tree as the third role layer for progressively loaded professional knowledge, not just a stored list of paths. AgentForge now supports structured `capabilities.skills` authoring and repo-local catalog discovery, but the current execution profile still drops skills from the Go to Bridge runtime contract, so `auto_load` has no runtime effect and on-demand skills never become an execution-facing knowledge layer.

## What Changes

- Add a repo-local skill runtime projection seam that resolves `skills/**/SKILL.md` into deterministic runtime-ready skill bundles, including normalized path, metadata, instruction body, and dependency metadata needed for execution-time loading.
- Project role skills into the normalized execution profile so auto-load skills become prompt-ready runtime context and on-demand skills remain available as summarized inventory instead of disappearing after authoring.
- Extend preview and sandbox flows to show which skills will auto-load into execution, which remain on-demand inventory, and which unresolved skill references are blocking versus warning-only.
- Update the TypeScript Bridge role contract and prompt injector so Bridge applies projected skill context without reading role YAML or skill files directly.
- Expand sample skills, sample role fixtures, docs, and focused verification so Skill-Tree behavior in docs, authoring, and runtime stays aligned.

## Capabilities

### New Capabilities
- `role-skill-runtime`: Resolve repo-local `SKILL.md` assets into normalized, load-policy-aware skill bundles that can be projected into execution-facing role context.

### Modified Capabilities
- `role-plugin-support`: Role execution profiles and spawn validation now project role skills into runtime-facing context instead of treating them as stored-only metadata.
- `agent-sdk-bridge-runtime`: Bridge `role_config` and prompt composition now consume projected skill context and skill inventory from Go without local file access.
- `role-authoring-sandbox`: Preview and sandbox output now explain loaded versus on-demand skill projection and distinguish blocking auto-load failures from warning-only on-demand gaps.

## Impact

- Affected backend seams: `src-go/internal/role/*`, `src-go/internal/model/role.go`, `src-go/internal/service/agent_service.go`, `src-go/internal/bridge/*`, and role preview or sandbox handlers that expose execution profiles and readiness diagnostics.
- Affected Bridge seams: `src-bridge/src/{schemas,types}.ts`, `src-bridge/src/role/injector.ts`, execution handlers, and runtime tests covering normalized role execution profiles.
- Affected authoring seams: preview or sandbox result mapping, role review helpers, and any dashboard surfaces that explain runtime projection before save or launch.
- Affected assets and docs: repo-local `skills/**/SKILL.md`, sample roles under `roles/`, `docs/role-yaml.md`, `docs/role-authoring-guide.md`, and any role or skill planning notes that currently imply runtime skill behavior.
- Affected verification: focused Go and Bridge tests for skill resolution, execution profile projection, prompt injection, preview or sandbox diagnostics, and sample role-to-skill consistency.
