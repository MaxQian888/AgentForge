# Role YAML Support

AgentForge now treats Go as the source of truth for role loading and execution-profile projection.

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
- `max_budget_usd`
- `max_turns`
- `permission_mode`

Bridge code does not read YAML files directly and should only consume this normalized profile.

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

This is intentionally a minimal backend binding seam. It enables PRD-aligned role-to-agent runtime behavior without requiring the full frontend role-selection UI to exist yet.

## Inheritance

Roles may use `extends` to inherit from another role.
The loader resolves inheritance in Go before roles are exposed through APIs or projected to execution config.
Security-oriented values resolve to the stricter effective policy when parent and child disagree.
