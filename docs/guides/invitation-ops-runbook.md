# Invitation Flow — Operations Runbook

Operational guide for the `project_invitations` lifecycle. See
[`docs/api/invitations.md`](../api/invitations.md) for the API surface
and [`openspec/specs/member-invitation-flow/spec.md`] for the authoritative
spec.

## Delivery failed — how to hand an accept link to the invitee

1. Open the target project's **Team → Pending Invitations** panel.
2. Find the pending row. If `lastDeliveryStatus` is anything other than
   `dispatched`, the notification never landed.
3. Use **Resend** to retry delivery (same token, fresh attempt).
4. If delivery still fails (e.g. no IM binding exists for the invitee
   yet), revoke the invitation and re-create it. The **Create** response
   contains the one-time `acceptUrl`; the admin UI displays it until you
   close the dialog — copy it and hand it to the invitee through a
   trusted side channel.

> Plaintext tokens are **not** retained server-side. If you miss the
> one-shot display, you must revoke + recreate — there is no "reveal
> token" endpoint by design.

## Diagnosing a stuck pending invitation

- **Checklist:**
  1. Is `expiresAt` in the past? The scheduler should have flipped it to
     `expired`; if not, verify the `invitation-expire-sweeper` scheduler
     job is enabled and running. `GET /api/v1/scheduler/jobs` shows
     status.
  2. Is `lastDeliveryStatus = manual_copy_required`? The invitee has no
     resolvable account/IM binding yet; hand them the accept link from
     a fresh Create and ask them to register first.
  3. Does the invitee report "identity mismatch" on accept? Check the
     row's `invitedIdentity`:
     - For `kind=email`: compare against the invitee's primary email
       case-insensitively. If the invitee logs in with a different
       email, revoke + re-invite with their actual email.
     - For `kind=im`: the invitee must have at least one `members` row
       bound to the same `(im_platform, im_user_id)`. Invite them via
       IM bridge first, then retry.

## Scheduler: invitation-expire-sweeper

| Setting | Value |
| --- | --- |
| Job key | `invitation-expire-sweeper` |
| Default schedule | `*/15 * * * *` (every 15 minutes) |
| Batch size | 500 rows per tick |
| Scope | system |

The sweeper is idempotent and safe to run manually via the scheduler
handler's **Run Now** affordance. Each run reports `expired: <count>`
in the run metrics.

## Audit trail

Every state transition emits a `project_audit_events` row with
`resource_type=invitation` and one of:

- `invitation.create`
- `invitation.accept`
- `invitation.decline`
- `invitation.revoke`
- `invitation.resend`

Query through the standard audit API:

```
GET /api/v1/projects/:pid/audit?resourceType=invitation&from=…
```

Payload snapshots include `invitationId`, `projectRole`, `status`, and
`identityKind`; sensitive values (tokens, raw email addresses) are
never persisted.

## Breakage modes and mitigations

| Symptom | Likely cause | Mitigation |
| --- | --- | --- |
| `409 invitation_already_pending_for_identity` on create | Prior pending invitation exists for same email / IM tuple. | Revoke the existing one, then re-create. |
| `403 invitation_identity_mismatch` on accept | Invitee logged in with a different identity than the one invited. | Verify the invite target; either invite the invitee's actual identity or ask them to log in with the matching one. |
| `404 invitation_not_found` on accept | Token has been revoked/declined/expired, or never existed. | Recreate the invitation and re-share the link. |
| `410 invitation_expired` on accept | Invitee acted after expiry. | Recreate with a longer `expiresAt` if appropriate (max 30 days). |
| Direct `POST /projects/:pid/members` returning `410 human_member_creation_moved_to_invitation_flow` | Caller is using the legacy path. | Switch to the invitation flow; only agents may be created directly. |

## Rollback

The invitation feature is additive. If a rollback is needed:

1. Frontend: disable the invitation entry points and restore the
   legacy "create human member" affordance.
2. Backend: revert the Go handler change that rejects `type=human` on
   `POST /projects/:pid/members`.
3. Database: the migration is reversible — `061_create_project_invitations.down.sql`
   drops the table and restores the prior audit `resource_type` CHECK.
   Existing pending invitations are deleted with the table; notify
   admins before rolling back.
