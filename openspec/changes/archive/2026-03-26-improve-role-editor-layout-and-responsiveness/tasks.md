## 1. Workspace Shell Refactor

- [x] 1.1 Split the current `components/roles/role-workspace.tsx` into clearer catalog, editor-shell, and context-rail responsibilities without changing the existing role draft contract.
- [x] 1.2 Introduce a role authoring section model that groups setup, identity, capabilities, knowledge, governance, and review flows in the order defined by the role authoring guide.
- [x] 1.3 Keep create, edit, template, inheritance, preview, sandbox, and save state wired through the refactored workspace shell without regressing the existing happy path.

## 2. Responsive Role Authoring Layout

- [x] 2.1 Implement a desktop layout that keeps the role catalog, main editor, and context surfaces simultaneously visible.
- [x] 2.2 Implement a medium-width layout that preserves role-library access and validation context without horizontal overflow or unclear panel ordering.
- [x] 2.3 Implement a narrow-width layout that preserves the create-edit-preview flow, including reachable save, preview, sandbox, summary, and YAML surfaces without losing draft state.

## 3. Guided Authoring Experience

- [x] 3.1 Add visible setup-stage affordances for template selection, inheritance selection, and current role-mode context so operators can start authoring from the recommended flow.
- [x] 3.2 Add section-aware guidance content that aligns editor wording and ordering with `docs/role-authoring-guide.md` and current PRD terminology.
- [x] 3.3 Reorganize the summary, YAML preview, and preview or sandbox actions so they stay discoverable from the same authoring context across breakpoints.

## 4. Docs And Product Copy Alignment

- [x] 4.1 Refresh role-editor copy in the roles workspace so section labels, helper text, and validation cues match the documented authoring model.
- [x] 4.2 Update `docs/role-authoring-guide.md` where needed so the documented flow matches the final role workspace layout and review surfaces.

## 5. Verification

- [x] 5.1 Add or update focused frontend tests for role catalog-to-editor flow, section navigation, and responsive layout entry points.
- [x] 5.2 Add or update focused frontend tests that verify summary, YAML preview, and preview or sandbox actions remain reachable in responsive modes.
- [x] 5.3 Run scoped verification for the affected role workspace tests and lint the touched role authoring files before marking the change ready.
