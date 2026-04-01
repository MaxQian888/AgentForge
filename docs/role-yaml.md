# Role YAML Support

AgentForge now treats Go as the source of truth for role loading, advanced role normalization, preview, sandbox probing, and execution-profile projection.

## Canonical Layout

New or updated roles should be stored at:

```text
roles/<role-id>/role.yaml
```

Examples in this repository include:

- `roles/coding-agent/role.yaml`
- `roles/frontend-developer/role.yaml`
- `roles/code-reviewer/role.yaml`

Role skills are now expected to resolve first against repo-local skill fixtures under:

```text
skills/<skill-id>/SKILL.md
```

Examples in this repository now include:

- `skills/react/SKILL.md`
- `skills/typescript/SKILL.md`
- `skills/css-animation/SKILL.md`
- `skills/testing/SKILL.md`

The official repo-owned built-in skill marketplace surface is now declared separately in:

```text
skills/builtin-bundle.yaml
```

That bundle does not replace `skills/**/SKILL.md` as the canonical package layout. It only declares which repo-local skills are currently promoted into the built-in marketplace surface, along with the market-facing metadata and verification path used to keep that surface truthful.

## Legacy Compatibility

The loader still reads legacy flat files such as `roles/frontend-developer.yaml` during migration.
When both a canonical directory role and a legacy flat file resolve to the same role id, the canonical file wins.

## Supported Authoring Surface

The current product flow supports more than the original minimal role manifest. Operators can now author or preview these sections through the backend-normalized role contract:

- `metadata`: `id`, `name`, `version`, `description`, `author`, `tags`, `icon`
- `identity`: `role`, `goal`, `backstory`, `persona`, `goals`, `constraints`, `personality`, `language`, `response_style`
- `capabilities`: `packages`, `allowed_tools`, structured `tools`, `skills`, `max_turns`, `max_budget_usd`
- `capabilities` advanced authoring: `tools.mcp_servers` and `custom_settings`
- `knowledge`: legacy `repositories/documents/patterns` plus `shared`, `private`, and `memory`
- `security`: `profile`, `permission_mode`, path rules, `output_filters`, and structured permissions or resource-limit blocks
- `collaboration`: delegation rules and communication preferences
- `triggers`: bounded event/action/condition rows
- `extends` and `overrides`

The dashboard authoring workspace now has dedicated controls for:

- `capabilities.custom_settings` as structured key-value rows
- `capabilities.tools.mcp_servers` as named server rows
- detailed `knowledge.shared` and `knowledge.private` rows, including `description` and `sources`
- `knowledge.memory` toggles and numeric limits
- `overrides` through a controlled JSON editor

The dashboard role workspace also supports a repo-local skill catalog:

- it discovers canonical repo-owned skills from `skills/**/SKILL.md`
- it offers catalog-backed role skill selection while preserving manual path entry
- it marks unresolved manual skill paths in review context instead of silently treating them as resolved catalog entries
- it exposes direct skill dependencies and declared tool requirements from skill metadata so authoring surfaces can explain why a role-skill combination is or is not compatible
- it shares the same package truth that now powers built-in skill marketplace previews, including Markdown from `SKILL.md` and normalized YAML from supported `agents/*.yaml` files

The UI still does not attempt to turn every future role field into a bespoke visual builder. When a field is not yet fully modeled by dedicated controls, the authoring flow must preserve it rather than silently dropping it during create, update, preview, or sandbox round-trips.

## Normalized Execution Profile

Go parses and normalizes the full Role YAML, then projects the runtime-facing subset into Bridge `role_config`.
The normalized execution profile currently contains:

- `role_id`
- `name`
- `role`
- `goal`
- `backstory`
- `system_prompt`
- `allowed_tools`
- `tools`
- `knowledge_context`
- `loaded_skills`
- `available_skills`
- `skill_diagnostics`
- `output_filters`
- `max_budget_usd`
- `max_turns`
- `permission_mode`

Bridge code does not read YAML files directly and should only consume this normalized profile.
`loaded_skills` contains the fully resolved repo-local auto-load skill bundles that Go has already prepared for prompt injection, while `available_skills` keeps non-auto-load skills visible as runtime inventory without preloading their full instructions.
`skill_diagnostics` explains blocking versus warning-only skill projection issues. A missing or invalid auto-load skill blocks execution-facing projection; an unresolved non-auto-load skill remains warning-level inventory context unless another runtime contract requires it.
`skill_diagnostics` now also covers role-skill compatibility mismatches. Auto-load skills and their dependency closure can block execution when their declared tool requirements are not covered by the effective role capability set; non-auto-load tool mismatches remain warning-only inventory context.
Fields such as `collaboration`, `memory`, and `triggers` still remain in the normalized Go role model, but they are not forwarded into the Bridge execution contract until there is a runtime consumer for them.
The same stored-only rule currently applies to `overrides`: it stays part of the canonical role definition and preview context, but it is not emitted into today's Bridge execution profile.

## Preview And Sandbox

The role authoring workflow now has two non-persistent backend surfaces:

- `POST /api/v1/roles/preview`
  - accepts either `roleId` or an unsaved `draft`
  - returns `normalizedManifest`, `effectiveManifest`, and `executionProfile`
- `POST /api/v1/roles/sandbox`
  - accepts either `roleId` or an unsaved `draft`, plus a bounded `input`
  - returns the same preview payload, runtime readiness diagnostics, selected runtime tuple, and an optional lightweight probe result
- `GET /api/v1/roles/skills`
  - returns repo-local skill catalog entries for role authoring, including canonical path, display metadata, direct dependency paths, and declared tool requirements when available

These flows do **not** create `agent_runs`, worktrees, or update `roles/<role-id>/role.yaml`. They exist to help operators validate advanced role definitions before saving or launch.
When the authoring flow edits an existing role, it should include the current `roleId` alongside the draft so preview and sandbox can preserve advanced stored sections even if only part of the role was edited in the current UI step.
Unresolved manual skill paths remain a valid authoring state as long as the role manifest itself is valid, but runtime behavior now depends on load policy:

- unresolved `auto_load` skills become blocking readiness diagnostics for preview, sandbox, and spawn
- unresolved non-auto-load skills remain warning-level inventory gaps
- resolved skills whose declared tool requirements are not covered by the effective role capability set follow the same severity split: blocking for auto-load closure, warning-only for on-demand inventory

## Role-Skill Compatibility Normalization

Compatibility checks do not compare raw role YAML tool strings to `SKILL.md` tool hints directly.
Go first normalizes the effective role into a capability set used only for compatibility evaluation:

- legacy CLI-oriented built-ins such as `Read`, `Edit`, `Write`, `Glob`, `Grep`, and `Bash`
- structured tool host entries such as `tools.external` and `tools.mcp_servers`
- compatible package and framework hints already present in the role manifest

That normalized set is then compared with skill-declared tool hints such as `code_editor`, `terminal`, and `browser_preview`.
This keeps sample roles like `roles/coding-agent/role.yaml` and `roles/frontend-developer/role.yaml` compatible with current repo-local skills without rewriting their canonical runtime `allowed_tools` into a new vocabulary just for authoring diagnostics.

## Agent Spawn Binding

The current backend runtime accepts an optional `roleId` when creating an agent run:

```json
{
  "taskId": "...",
  "memberId": "...",
  "runtime": "codex",
  "provider": "openai",
  "model": "gpt-5-codex",
  "roleId": "frontend-developer"
}
```

Go resolves that `roleId` through the unified YAML-backed role store before execution starts, projects it into the normalized `role_config`, and forwards the projected settings to the Bridge request. Unknown role ids are rejected before runtime startup. The resulting `agent_runs` record also retains the selected `role_id` for inspection and API responses.

The Bridge-bound `role_config` is now expected to carry the runtime-facing advanced fields that the current Bridge path actually consumes:

- `tools`: Bridge-consumable plugin or MCP tool identifiers
- `knowledge_context`: extra injected role knowledge context for prompt assembly
- `loaded_skills`: resolved auto-load skill bundles with prompt-ready instructions
- `available_skills`: non-auto-load skill inventory kept available for diagnostics or future runtime controls
- `skill_diagnostics`: normalized skill projection diagnostics computed by Go
- `output_filters`: output filter identifiers such as `no_credentials` or `no_pii`

This remains the production execution seam for real agent runs. Preview and sandbox authoring flows are separate non-persistent helpers and do not replace the normal spawn contract.

## Runtime Selection And Propagation

Role binding now participates in a larger coding-agent runtime contract shared by project settings, agent launch, and Team launch flows.

The resolved execution tuple is always:

- `runtime`
- `provider`
- `model`

Current supported coding-agent runtimes are:

| Runtime | Provider Rules | Typical Use |
| --- | --- | --- |
| `claude_code` | only `anthropic` | Claude Code backed execution |
| `codex` | `openai` or legacy-compatible `codex` | Codex-backed execution |
| `opencode` | only `opencode` | OpenCode-backed execution |
| `cursor` | only `cursor` | Cursor Agent backed execution through the CLI-backed runtime profile family |
| `gemini` | `google` or `vertex` | Gemini CLI backed execution through the CLI-backed runtime profile family |
| `qoder` | only `qoder` | Qoder CLI backed execution through the CLI-backed runtime profile family |
| `iflow` | only `iflow` | iFlow CLI backed execution through the CLI-backed runtime profile family |

Go resolves this tuple from project defaults plus explicit launch overrides before it projects the role profile into Bridge `role_config`. That means role selection no longer silently falls back to a provider-only guess.

## Team Lifecycle Consistency

When a Team run starts, the resolved runtime/provider/model tuple is stored with the team config and reused for:

- planner spawn
- downstream coder spawn
- reviewer spawn
- retry flows

This keeps Claude Code, Codex, and OpenCode support consistent across the full Team lifecycle instead of only applying the selection to the first planner phase.
The same resolved tuple is now also preserved for the additional CLI-backed runtimes. However, Team or single-agent callers must still respect each runtime's advertised capability matrix instead of assuming full lifecycle parity.
Team-managed Bridge runs now also carry explicit `team_id` and `team_role` (`planner`, `coder`, or `reviewer`) in their execution request and preserved runtime identity, so status, snapshot, and resume flows do not need to reconstruct Team phase context from separate database lookups.

## Readiness Diagnostics

Runtime readiness is exposed through the coding-agent catalog returned by the backend. UI surfaces should use that catalog to show:

- missing API credentials
- missing runtime executables
- incompatible runtime/provider pairs
- unsupported bounded model selections
- blocked continuity for runtimes that do not support truthful resume

This aligns with the PRD and plugin-system direction that runtime capability discovery belongs to the execution infrastructure, not to hard-coded frontend option lists.

## Advanced Authoring Boundaries

Current authoring surfaces should make the following distinction explicit:

- runtime-facing fields such as `allowed_tools`, Bridge tool identifiers, `knowledge_context`, `output_filters`, budget, turns, and permission mode influence the execution profile directly
- stored-only advanced fields such as `knowledge.memory`, `collaboration`, `triggers`, and `overrides` remain in the canonical YAML manifest and preview context only

This boundary prevents operators from assuming that every advanced role field is already consumed by the runtime while still keeping those fields stable and editable in the source of truth.

## Inheritance

Roles may use `extends` to inherit from another role.
The loader resolves inheritance in Go before roles are exposed through APIs or projected to execution config.
Security-oriented values resolve to the stricter effective policy when parent and child disagree.
Advanced sections such as packages, tool host config, shared knowledge, collaboration metadata, and triggers also merge through documented deterministic rules instead of being silently dropped.

## Focused Verification

For the TSBridge role and Team context seam, the current focused verification path is:

```powershell
cd src-go
go test ./internal/bridge ./internal/role ./internal/service -count=1

cd ../src-bridge
bun test src/schemas.test.ts src/runtime/agent-runtime.test.ts src/handlers/claude-runtime.test.ts
```
