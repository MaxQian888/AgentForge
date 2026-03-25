## 1. Role Schema And Persistence

- [x] 1.1 Expand the Go role manifest models, parser, and canonical YAML round-trip logic to support the advanced PRD-backed role fields needed for authoring completeness.
- [x] 1.2 Update role inheritance and merge logic so advanced sections such as packages, tool host config, knowledge or memory blocks, collaboration metadata, triggers, and governance settings resolve deterministically.
- [x] 1.3 Extend role handlers and API response types so list, get, create, and update flows preserve and return the advanced structured fields instead of dropping them.

## 2. Role Preview And Sandbox Backend

- [x] 2.1 Add a non-persistent role preview service and API that resolves stored roles or unsaved drafts into normalized and effective manifest payloads plus execution profile output.
- [x] 2.2 Add authoritative validation and readiness diagnostics for preview or sandbox requests, including nested schema errors, inheritance issues, and runtime prerequisite checks.
- [x] 2.3 Implement a bounded role sandbox probe flow that reuses the bridge text-generation surface for sample prompt testing without creating tasks, worktrees, or persisted agent runs.

## 3. Frontend Role Contract And Draft Modeling

- [x] 3.1 Expand the frontend role types and serialization helpers to cover the advanced manifest sections returned by the backend and required by the updated specs.
- [x] 3.2 Refactor the role draft model and normalization utilities so advanced identity, capability, knowledge, security, collaboration, trigger, and override sections can be edited and validated consistently.

## 4. Authoring Workspace Experience

- [x] 4.1 Upgrade the role workspace UI to expose advanced authoring sections, inheritance visibility, and structured editors for the newly supported role fields.
- [x] 4.2 Add field-level authoring guidance and YAML-oriented draft or preview rendering so operators can understand how structured input maps back to canonical role assets.
- [x] 4.3 Integrate backend preview and sandbox actions into the role workspace so operators can inspect effective manifests, diagnostics, and bounded probe output from the current draft.

## 5. Samples And Documentation

- [x] 5.1 Update canonical sample role manifests under `roles/` to demonstrate the advanced schema paths that the product now supports.
- [x] 5.2 Refresh `docs/role-yaml.md` plus the role-related PRD and plugin-design guidance so the repo documents the new role authoring contract and sandbox workflow accurately.
- [x] 5.3 Add operator-facing guidance for how to use advanced role fields, YAML preview, and sandbox validation during role authoring.

## 6. Verification

- [x] 6.1 Add focused Go tests for advanced role parsing, inheritance merge, canonical save and reload, preview responses, and sandbox diagnostics.
- [x] 6.2 Add focused frontend tests for advanced role draft editing, YAML preview, validation messaging, and preview or sandbox interaction flows.
- [x] 6.3 Run scoped verification for the role parser or API, role workspace, and sandbox happy paths before marking the change ready for apply.
