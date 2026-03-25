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

## Legacy Compatibility

The loader still reads legacy flat files such as `roles/frontend-developer.yaml` during migration.
When both a canonical directory role and a legacy flat file resolve to the same role id, the canonical file wins.

## Supported Authoring Surface

The current product flow supports more than the original minimal role manifest. Operators can now author or preview these sections through the backend-normalized role contract:

- `metadata`: `id`, `name`, `version`, `description`, `author`, `tags`, `icon`
- `identity`: `role`, `goal`, `backstory`, `persona`, `goals`, `constraints`, `personality`, `language`, `response_style`
- `capabilities`: `packages`, `allowed_tools`, structured `tools`, `skills`, `max_turns`, `max_budget_usd`
- `knowledge`: legacy `repositories/documents/patterns` plus `shared`, `private`, and `memory`
- `security`: `profile`, `permission_mode`, path rules, `output_filters`, and structured permissions or resource-limit blocks
- `collaboration`: delegation rules and communication preferences
- `triggers`: bounded event/action/condition rows
- `extends` and `overrides`

The authoring UI does not have to expose every nested field with a dedicated control at all times, but the Go role store and APIs now preserve these sections instead of dropping them during round-trip save or preview.

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
- `output_filters`
- `max_budget_usd`
- `max_turns`
- `permission_mode`

Bridge code does not read YAML files directly and should only consume this normalized profile.
Fields such as `collaboration`, `memory`, and `triggers` still remain in the normalized Go role model, but they are not forwarded into the Bridge execution contract until there is a runtime consumer for them.

## Preview And Sandbox

The role authoring workflow now has two non-persistent backend surfaces:

- `POST /api/v1/roles/preview`
  - accepts either `roleId` or an unsaved `draft`
  - returns `normalizedManifest`, `effectiveManifest`, and `executionProfile`
- `POST /api/v1/roles/sandbox`
  - accepts either `roleId` or an unsaved `draft`, plus a bounded `input`
  - returns the same preview payload, runtime readiness diagnostics, selected runtime tuple, and an optional lightweight probe result

These flows do **not** create `agent_runs`, worktrees, or update `roles/<role-id>/role.yaml`. They exist to help operators validate advanced role definitions before saving or launch.

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

Go resolves this tuple from project defaults plus explicit launch overrides before it projects the role profile into Bridge `role_config`. That means role selection no longer silently falls back to a provider-only guess.

## Team Lifecycle Consistency

When a Team run starts, the resolved runtime/provider/model tuple is stored with the team config and reused for:

- planner spawn
- downstream coder spawn
- reviewer spawn
- retry flows

This keeps Claude Code, Codex, and OpenCode support consistent across the full Team lifecycle instead of only applying the selection to the first planner phase.
Team-managed Bridge runs now also carry explicit `team_id` and `team_role` (`planner`, `coder`, or `reviewer`) in their execution request and preserved runtime identity, so status, snapshot, and resume flows do not need to reconstruct Team phase context from separate database lookups.

## Readiness Diagnostics

Runtime readiness is exposed through the coding-agent catalog returned by the backend. UI surfaces should use that catalog to show:

- missing API credentials
- missing runtime executables
- incompatible runtime/provider pairs

This aligns with the PRD and plugin-system direction that runtime capability discovery belongs to the execution infrastructure, not to hard-coded frontend option lists.

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
