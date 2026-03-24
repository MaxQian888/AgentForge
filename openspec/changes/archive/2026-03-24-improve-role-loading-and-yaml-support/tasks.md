## 1. Role schema and YAML loading foundation

- [x] 1.1 Expand `src-go/internal/model/role.go` and `src-go/internal/role/*` to parse and normalize the PRD-aligned Role YAML structure, including top-level identity fields and deterministic prompt synthesis for executable roles.
- [x] 1.2 Add canonical `roles/<role-id>/role.yaml` discovery plus legacy flat-file compatibility, and enforce authoritative precedence when both layouts define the same role.
- [x] 1.3 Implement role inheritance and override resolution, including stricter-effective merging for security and resource-governance settings.

## 2. Unified Go role registry and API behavior

- [x] 2.1 Refactor the Go role registry/store so list/get/save/update operations all go through one YAML-backed role loading path instead of handler-local file scans and hardcoded preset role slices.
- [x] 2.2 Update `src-go/internal/handler/role_handler.go` and related wiring to persist API-created or updated roles to the canonical directory layout and return normalized role data.
- [x] 2.3 Add focused Go tests for canonical loading, legacy compatibility, duplicate precedence, validation failures, inheritance resolution, and API read/write behavior.

## 3. Execution profile projection and Bridge contract alignment

- [x] 3.1 Add a Go-side execution-profile builder that derives runtime-facing role configuration from a fully resolved role manifest for current and future agent execution call sites.
- [x] 3.2 Align `src-go/internal/bridge/client.go`, `src-bridge/src/types.ts`, `src-bridge/src/schemas.ts`, and role injection helpers around the normalized `role_config` contract rather than raw YAML-shaped payloads.
- [x] 3.3 Add focused Go and Bridge tests covering accepted normalized role execution profiles and rejection of incomplete or raw nested Role YAML payloads.
- [x] 3.4 Wire the minimal `POST /api/v1/agents/spawn` `roleId` path so Go resolves the selected role, persists `agent_runs.role_id`, and forwards the normalized execution profile into Bridge execution.

## 4. Role assets, docs, and verification

- [x] 4.1 Migrate or replace built-in role fixtures and sample role assets so the repository includes PRD-aligned YAML examples in the canonical layout.
- [x] 4.2 Update role-related documentation to describe the canonical directory structure, YAML schema expectations, compatibility boundaries, and execution-profile projection behavior.
- [x] 4.3 Run the scoped role-loading, spawn binding, and Bridge verification commands, then capture any residual follow-up items needed before implementation sign-off.
