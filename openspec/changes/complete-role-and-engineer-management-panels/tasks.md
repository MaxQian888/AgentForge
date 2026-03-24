## 1. Align Role And Member Data Contracts

- [x] 1.1 Extend the member API contract in `src-go/internal/model/member.go`, `src-go/internal/handler/member_handler.go`, and `src-go/internal/repository/member_repo.go` so agent profile data and skills can round-trip through list/create/update flows instead of being silently dropped.
- [x] 1.2 Update `lib/stores/member-store.ts` and `lib/dashboard/summary.ts` to normalize typed agent profile data, member-type-specific summaries, and readiness cues needed by the Team UI.
- [x] 1.3 Add shared draft/summary helpers for role editing and agent member profile editing so UI components can validate, prefill, preview, and serialize supported fields consistently.

## 2. Build The Role Management Workspace

- [x] 2.1 Replace the current minimal `components/roles/role-form-dialog.tsx` flow with a workspace-style editor in `app/(dashboard)/roles/page.tsx` and `components/roles/*` that covers metadata, version, identity, prompt, capabilities, knowledge, security, and inheritance sections.
- [x] 2.2 Add template-copy and inheritance-start flows that reuse existing roles from `useRoleStore` as creation sources, including visible source markers in the draft experience.
- [x] 2.3 Implement a live execution-summary rail for role drafts that surfaces prompt intent, tool constraints, budget or turn limits, permission mode, and safety/path cues before save.
- [x] 2.4 Expand the role library cards/list summaries so operators can compare version, tags, inheritance state, and governance signals without opening the editor.

## 3. Upgrade Team Management For Engineer Profiles

- [ ] 3.1 Refactor `components/team/team-management.tsx` into member-type-aware create/edit flows so human members stay simple while agent members gain structured profile sections for skills, bound role, activation state, and supported agent settings.
- [ ] 3.2 Add role-binding and agent-profile editing controls that consume the existing roles catalog and persist agent profile changes through the updated member store/API contract.
- [ ] 3.3 Surface agent readiness and role linkage in the roster UI so agent members show bound-role state, key skills, and attention-needed cues when their configuration is incomplete.
- [ ] 3.4 Ensure project/team member updates preserve edited skills and agent profile data end-to-end instead of regressing to the current shallow update behavior.

## 4. Verification And Regression Coverage

- [ ] 4.1 Add or update focused frontend tests for role workspace editing, template/inheritance flows, execution-summary rendering, and role library governance cues.
- [ ] 4.2 Add or update focused team-management tests for agent member creation/editing, readiness summaries, and member API/store round-trip behavior.
- [ ] 4.3 Run the relevant lint, typecheck, and scoped test commands for the touched frontend/backend surfaces and document any remaining follow-up items before apply completion.
