## 1. Backend member bulk-governance contract

- [x] 1.1 Add project-scoped bulk member update request/response models plus route/handler wiring for availability governance under the existing member API surface.
- [x] 1.2 Implement repository/service support for batch status transitions with canonical status normalization and per-member result reporting.
- [x] 1.3 Add focused Go tests covering successful bulk updates, mixed-result responses, and project-scope safety.

## 2. Frontend member state plumbing

- [x] 2.1 Extend `lib/stores/member-store.ts` with bulk-governance actions, pending/result state, and roster refresh behavior after batch updates.
- [x] 2.2 Add or update shared team-management helpers so attention categories and quick lifecycle actions derive from the same readiness/status truth as the existing roster.

## 3. Team roster management workspace

- [x] 3.1 Add attention summary controls to `components/team/team-management.tsx` so operators can focus setup-required, inactive, and suspended members without rebuilding filters manually.
- [x] 3.2 Implement roster multi-select and bulk availability actions for `active` / `inactive` / `suspended`, including inline result feedback.
- [x] 3.3 Add row-level quick lifecycle controls and in-flight protection while preserving the existing deep edit flow for complex member changes.
- [x] 3.4 Reset selection, bulk-action state, and attention-specific UI state when the active project scope changes in `components/team/team-page-client.tsx`.

## 4. Focused verification

- [x] 4.1 Expand the team workspace Jest coverage for attention filtering, bulk governance, quick lifecycle actions, and project-switch cleanup.
- [x] 4.2 Run targeted frontend and Go verification for the touched team-management/member seams and fix any regressions needed for the new flow.
