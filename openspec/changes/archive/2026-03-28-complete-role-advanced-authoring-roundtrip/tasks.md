## 1. Draft And Contract Alignment

- [x] 1.1 Extend frontend role draft/state helpers to retain the full normalized manifest and preserve untouched advanced sections during create/edit/template/inheritance flows.
- [x] 1.2 Update Go role API and preview/sandbox contract handling so advanced authoring sections round-trip consistently across get/list/create/update/preview/sandbox.
- [x] 1.3 Add focused backend and store-level tests that prove advanced fields survive partial edits without silent loss.

## 2. Advanced Role Authoring Workspace

- [x] 2.1 Add advanced authoring controls for supported structured sections such as custom settings, richer tool-host metadata, and detailed knowledge or memory rows.
- [x] 2.2 Add a controlled advanced editor for open-ended sections such as `overrides`, with clear validation and author guidance.
- [x] 2.3 Update role workspace validation so malformed advanced subsection input blocks save and maps errors back to the relevant authoring group.

## 3. Preview, Sandbox, And Review Context

- [x] 3.1 Extend the role review rail and YAML-oriented views to show advanced field provenance, preserved sections, and execution-profile versus canonical-YAML boundaries.
- [x] 3.2 Update preview and sandbox result handling so advanced validation issues and inherited/effective advanced values are visible from the current authoring flow.
- [x] 3.3 Add focused UI tests for advanced authoring review, preview, sandbox, and responsive access paths.

## 4. Documentation And Fixtures

- [x] 4.1 Update `docs/role-yaml.md` and `docs/role-authoring-guide.md` to describe the supported advanced authoring surface and editing boundaries.
- [x] 4.2 Refresh canonical role fixtures under `roles/` and any related test fixtures so advanced-field examples reflect the supported contract.
- [x] 4.3 Run focused verification for role parser/store tests, role workspace tests, and any preview/sandbox coverage added by this change.
