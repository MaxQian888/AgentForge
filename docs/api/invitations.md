# Invitations / 成员邀请流

Human member onboarding follows a **pending contract → accept/decline**
model. Agents are still created directly through `POST /projects/:pid/members`.

## Lifecycle

```
                      ┌──────────────┐  admin revoke
            ┌────────►│   revoked    │◄───────────┐
            │         └──────────────┘            │
            │                                     │
┌───────────┴─┐   accept (auth + id match)   ┌────┴─────┐
│   pending   │ ─────────────────────────►   │ accepted │
└──────┬──┬───┘                              └──────────┘
       │  │  decline (token, optional auth)
       │  │                                  ┌──────────┐
       │  └────────────────────────────────►│ declined │
       │                                    └──────────┘
       │  expires_at < now() (scheduler or accept check)
       │                                    ┌──────────┐
       └───────────────────────────────────►│ expired  │
                                            └──────────┘
```

All non-pending statuses are terminal. The one-time token hash is cleared
on every terminal transition; token reuse after accept/decline/revoke
returns `404 invitation_not_found`.

## Endpoints

| Method | Path | Auth | Role |
| --- | --- | --- | --- |
| `POST` | `/api/v1/projects/:pid/invitations` | Bearer | `admin+` |
| `GET` | `/api/v1/projects/:pid/invitations?status=pending` | Bearer | `admin+` |
| `POST` | `/api/v1/projects/:pid/invitations/:id/revoke` | Bearer | `admin+` |
| `POST` | `/api/v1/projects/:pid/invitations/:id/resend` | Bearer | `admin+` |
| `GET` | `/api/v1/invitations/by-token/:token` | **none** | — |
| `POST` | `/api/v1/invitations/accept` | Bearer | any |
| `POST` | `/api/v1/invitations/decline` | optional | — |

### Create — `POST /projects/:pid/invitations`

Request body:

```json
{
  "invitedIdentity": { "kind": "email", "value": "alice@example.com" },
  "projectRole": "editor",
  "message": "Welcome to Shipyard",
  "expiresAt": "2026-04-26T00:00:00Z"
}
```

Or, for IM identities:

```json
{
  "invitedIdentity": {
    "kind": "im",
    "platform": "feishu",
    "userId": "ou_abc123",
    "displayName": "Alice"
  },
  "projectRole": "editor"
}
```

`expiresAt` is clamped to `[1h, 30d]` from now; if omitted, defaults to 7
days. `projectRole` accepts the canonical four roles: `owner | admin |
editor | viewer`. Owner invitations are explicitly allowed.

**201 response**:

```json
{
  "invitation": { ...InvitationDTO },
  "acceptToken": "<plaintext-32-byte-hex>",
  "acceptUrl": "https://app/invitations/accept?token=<plaintext>"
}
```

`acceptToken` and `acceptUrl` are returned **once**; the server persists
only the SHA-256 hash. Subsequent reads (`list`, `resend`) never expose
them — admins that lose the link must use `resend` (delivery only) or,
failing that, revoke + recreate.

### List — `GET /projects/:pid/invitations`

Query param: `status` (optional) — one of
`pending | accepted | declined | expired | revoked`.

Returns `InvitationDTO[]` sorted by `createdAt` desc.

### Revoke — `POST /projects/:pid/invitations/:id/revoke`

Body: `{ "reason": "optional explanation" }`. Only pending invitations can
be revoked; revoking a terminal row returns `409 invitation_already_processed`.

### Resend — `POST /projects/:pid/invitations/:id/resend`

Re-triggers delivery via the configured notification channel without
rotating the token. Updates `lastDeliveryStatus` /
`lastDeliveryAttemptedAt` on the invitation row. If the plaintext token
has been lost (server never retains it), the delivery layer falls back to
`manual_copy_required`.

### Public preview — `GET /invitations/by-token/:token`

Unauthenticated. Returns a narrow payload suitable for a pre-login
"you've been invited" page:

```json
{
  "projectName": "Shipyard",
  "projectRole": "editor",
  "inviterName": "Ops Admin",
  "inviterEmail": "ops@example.com",
  "message": "optional",
  "expiresAt": "2026-04-26T00:00:00Z",
  "status": "pending",
  "identityKind": "email",
  "identityHint": "a***e@example.com"
}
```

Email `identityHint` is masked; IM identities return the `displayName`
field when present.

### Accept — `POST /invitations/accept`

Requires authenticated caller. Body: `{ "token": "..." }`.

Server validates:

1. Token hash resolves to a pending, unexpired invitation.
2. Caller's identity matches `invitedIdentity`:
   - `kind=email`: case-insensitive compare against user's primary email.
   - `kind=im`: caller must have a `members` row anywhere in the system
     with `(im_platform, im_user_id)` matching the invitation.
3. No other caller has already accepted (atomic `UPDATE … WHERE
   status='pending' AND token_hash=? AND expires_at>now()`).

On success returns:

```json
{
  "invitation": { ...accepted },
  "member": { ...materialized member row }
}
```

Re-accepting with an existing membership is idempotent (returns the
existing member without creating a duplicate row).

### Decline — `POST /invitations/decline`

Auth optional. Body: `{ "token": "...", "reason": "optional" }`.
Transitions the invitation to `declined` and clears the token hash. Safe
to invoke from a pre-login "decline invitation" link.

## Error codes

| HTTP | Message ID | Meaning |
| --- | --- | --- |
| `400` | `InvitationInvalidIdentity` | `invitedIdentity` shape is invalid. |
| `400` | `InvitationInvalidRole` | `projectRole` is not in the canonical enum. |
| `400` | `InvalidInvitationToken` | Token path parameter is empty. |
| `403` | `InvitationIdentityMismatch` | Caller identity doesn't match `invitedIdentity`. |
| `404` | `InvitationNotFound` | Token hash does not match any row (revoked, declined, or never existed). |
| `409` | `InvitationAlreadyPendingForIdentity` | A pending invitation already exists for the same identity + project. |
| `409` | `InvitationAlreadyProcessed` | The invitation is already in a terminal status. |
| `410` | `InvitationExpired` | `expires_at` has passed. |
| `410` | `HumanMemberCreationMovedToInvitationFlow` | Direct human creation on `POST /members` is retired; use the invitation flow. |

## Security notes

- 32-byte CSPRNG token, persisted as SHA-256 hash. Plaintext leaves the
  server only through the 201 response body and optional delivery channel.
- Delivery failures **do not** roll back the invitation; admins can
  always fall back to "copy accept link" in the UI.
- RBAC: create / list / revoke / resend all require `admin+`. Accept /
  decline / by-token are not project-scoped — gating is via token +
  identity matching.
- Scheduler job `invitation-expire-sweeper` runs every 15 minutes to
  flip `pending` rows past `expires_at` into `expired`; accept-time check
  is a belt-and-suspenders guard against sweeper lag.
