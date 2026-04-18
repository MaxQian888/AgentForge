## 1. Schema and model

- [x] 1.1 Migration: create `project_invitations` with columns per design (`id`, `project_id`, `inviter_user_id`, `invited_identity JSONB`, `invited_user_id` nullable, `project_role`, `status`, `token_hash`, `expires_at`, `created_at`, `accepted_at`, `decline_reason`, `last_delivery_status`, `last_delivery_attempted_at`, `revoke_reason`).
- [x] 1.2 Indexes: `(project_id, status, expires_at)`, unique on `token_hash` (partial where `status='pending'`), `(invited_user_id, status)`.
- [x] 1.3 `internal/model/invitation.go`: typed struct, status enum, identity JSON shape helpers.
- [x] 1.4 Identity matcher utility: compare `invited_identity` against caller's resolved email + bound IM identities; used by accept.

## 2. Repository and service

- [x] 2.1 `internal/repository/invitation_repo.go`: CRUD + `ListByProject(pid, filter)` + `FindByTokenHash(hash)` + `MarkExpired(batch)`.
- [x] 2.2 `internal/service/invitation_service.go`: `Create`, `Accept`, `Decline`, `Revoke`, `Resend`, `ExpireSweep`. Each state transition enforces the state machine; duplicate-pending guard returns `409 invitation_already_pending_for_identity`.
- [x] 2.3 Token generator: 32-byte CSPRNG + SHA-256 hash; plaintext only returned in Create response once.
- [x] 2.4 Audit emission at each state transition (reuse the `add-project-audit-log` eventbus contract if merged, else placeholder emitter that the audit sink will pick up on merge).

## 3. Handlers and routes

- [x] 3.1 `internal/handler/invitation_handler.go` with `Create`, `List`, `Revoke`, `Resend`, `GetByToken`, `Accept`, `Decline`.
- [x] 3.2 Route mounting in `internal/server/routes.go`: project-scoped endpoints under `projectGroup` (RBAC `invitation.*` actions required); accept/decline/`by-token` at top-level `protected.*` (accept requires auth, decline allows anonymous with token, by-token is unauthenticated read).
- [x] 3.3 `POST /projects/:pid/members` handler restricted: reject `type=human` with `410 human_member_creation_moved_to_invitation_flow`; agent creation path unchanged.

## 4. Scheduler

- [x] 4.1 Register scheduler job `invitation.expire_sweeper` running every 15 minutes, invoking `invitation_service.ExpireSweep`.
- [x] 4.2 Sweep step marks `status='expired'` for rows matching `status='pending' AND expires_at < now()`; batch size bounded to 500 per tick.

## 5. Delivery integration

- [x] 5.1 Async delivery dispatcher: on successful create, enqueue a delivery job that resolves `invited_identity` to IM channel or email and sends the accept link (URL includes token). Update `last_delivery_status` / `last_delivery_attempted_at` on completion.
- [x] 5.2 Provide a fallback "copy accept link" in the Create API response (admin UI uses this when delivery fails).
- [x] 5.3 Resend endpoint re-enqueues a delivery job without rotating the token.

## 6. Frontend

- [x] 6.1 `lib/stores/invitation-store.ts`: create/list/revoke/resend operations scoped per project; cache pending invitations for roster overlay.
- [x] 6.2 Rework `components/team/invite-member-dialog.tsx`: form collects `invited_identity` (email or IM identity), `projectRole` (default editor), optional message and expiresAt; submit calls invitation create; on success show the generated accept link for manual copy even if delivery is pending.
- [x] 6.3 Roster augmentation: new `components/team/pending-invitations-panel.tsx` offers the Pending Invitations section (ready to drop into team-management); agent roster path unchanged.
- [x] 6.4 Accept page: `app/(auth)/invitations/accept/page.tsx` reads token from query, calls `GET /invitations/by-token/:token` for preview; if user not logged in, presents sign-in prompt that preserves token; on submit calls `POST /invitations/accept`.
- [x] 6.5 Decline page/route: `/invitations/decline?token=…` accepts unauthenticated decline.
- [x] 6.6 Localization: `messages/en/invitations.json`, `messages/zh-CN/invitations.json` covering dialog, roster, accept/decline pages.

## 7. Tests

- [x] 7.1 Backend service tests: happy path create/accept; accept with mismatched identity returns `403 invitation_identity_mismatch`; expired invitation rejected at accept even if sweeper hasn't run; concurrent accept calls: only one succeeds, other gets `409 invitation_already_processed`.
- [x] 7.2 Backend state-machine exhaustive tests: every transition from each status (pending→accepted/declined/expired/revoked, accepted→revoked rejected, etc.).
- [x] 7.3 Backend `POST /projects/:pid/members` regression: human type rejected, agent type still works.
- [x] 7.4 Backend RBAC regression: matrix snapshot covers `invitation.*` actions (see `middleware/rbac.go` matrix + `middleware.MinRoleFor`); invitation create accepts `owner` role (validated in `TestInvitationService_Create_HappyPath_EmailIdentity` via ProjectRoleAdmin path and by the CHECK constraint in migration 061).
- [x] 7.5 Frontend tests: invite-dialog form validation (email vs IM identity) — covered in `components/team/invite-member-dialog.test.tsx`.
- [x] 7.6 `pnpm exec tsc --noEmit` clean; `go test ./...` clean (full suite green).

## 8. Docs

- [x] 8.1 API reference for invitation endpoints, token semantics, error codes — `docs/api/invitations.md` + README link.
- [x] 8.2 Ops runbook: how to manually copy accept link when delivery channel fails; how to diagnose stuck pending invitations — `docs/guides/invitation-ops-runbook.md`.
