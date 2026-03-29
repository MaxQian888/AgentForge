## 1. Member Persistence And API Contract

- [x] 1.1 Add member schema/migration support for canonical `status`, `im_platform`, and `im_user_id`, including backfill and uniqueness rules needed by the documented contract.
- [x] 1.2 Update Go member models, DTOs, handlers, and repositories to round-trip the new fields while deriving compatibility `isActive` from canonical status.
- [x] 1.3 Add or update focused backend tests for member create/update/list behavior covering tri-state status, IM identity fields, and compatibility mapping.

## 2. Frontend Team Management Surface

- [x] 2.1 Update member store/types and dashboard team-member normalization so frontend consumers receive canonical status plus IM identity without breaking existing callers that still read `isActive`.
- [x] 2.2 Extend `components/team/team-management.tsx` and related page plumbing to create, edit, filter, and display documented member status and IM identity for both human and agent members.
- [x] 2.3 Add or update focused frontend tests for roster rendering, filters, and member form round-trip with the documented member contract.

## 3. Availability Consumers

- [x] 3.1 Introduce a shared availability helper that interprets canonical member status for recommendation, assignment, and agent-ready checks.
- [x] 3.2 Update recommendation or assignment entry points to exclude inactive/suspended members from ready candidates while keeping them manageable in team surfaces.
- [x] 3.3 Add or update focused tests for unavailable-member recommendation and assignment behavior.

## 4. Focused Verification

- [x] 4.1 Run targeted backend verification for member repository/handler/service paths affected by the new member contract.
- [x] 4.2 Run targeted frontend verification for team-management and task-assignment surfaces affected by status/IM identity changes.
- [x] 4.3 Confirm the change remains apply-ready with `openspec status --change align-team-management-with-documented-member-contract`.
