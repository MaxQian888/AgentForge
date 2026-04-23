---
title: "Spec 5: Email System"
date: 2026-04-23
status: draft
depends_on: [1]
---

# Email System

## Problem

AgentForge has no email capability. There is no email verification at registration, no email notifications, no email-based interactions (reply-to-update-task), and no SMTP configuration. Enterprise deployment requires email for verification, alerts, and communication.

## Current State

- User model has `email` field (VARCHAR, stored in plaintext)
- No email verification flow
- No SMTP integration
- Notification service supports `email` as a channel type but has no implementation
- No email templates

## Design

### Architecture

```
AgentForge Backend → Email Service → SMTP Relay (org-configurable)
                                     └── Default: built-in SMTP or API-based (Resend/SendGrid)
```

The email service abstracts the transport layer so organizations can configure their own SMTP or use a cloud email API.

### Data Model

```sql
-- Email configuration per org (or global for single-org)
CREATE TABLE email_configs (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id      UUID UNIQUE REFERENCES organizations(id),  -- nullable = platform default
  provider    VARCHAR(32) NOT NULL DEFAULT 'smtp',  -- smtp, resend, sendgrid, console (dev)
  config      JSONB NOT NULL DEFAULT '{}',           -- provider-specific config (encrypted)
  from_address VARCHAR(256) NOT NULL,
  from_name   VARCHAR(256),
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Email templates
CREATE TABLE email_templates (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id      UUID REFERENCES organizations(id),  -- nullable = system template
  name        VARCHAR(128) NOT NULL,              -- e.g. "invitation", "task_assigned"
  subject     TEXT NOT NULL,
  body_html   TEXT NOT NULL,
  body_text   TEXT,                               -- optional plain-text fallback
  variables   JSONB DEFAULT '[]',                 -- e.g. ["userName", "taskTitle", "projectName"]
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(org_id, name)
);

-- Email delivery log
CREATE TABLE email_deliveries (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id      UUID REFERENCES organizations(id),
  user_id     UUID REFERENCES users(id),
  template    VARCHAR(128),
  recipients  JSONB NOT NULL,                     -- array of {email, name}
  subject     TEXT NOT NULL,
  status      VARCHAR(16) NOT NULL DEFAULT 'pending', -- pending, sent, delivered, failed, bounced
  provider_id VARCHAR(256),                       -- external message ID for tracking
  sent_at     TIMESTAMPTZ,
  error       TEXT,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### Transport Providers

```go
type EmailProvider interface {
    Send(ctx context.Context, msg EmailMessage) (providerID string, err error)
}

type EmailMessage struct {
    To          []Address
    Cc          []Address
    Bcc         []Address
    Subject     string
    BodyHTML    string
    BodyText    string
    Headers     map[string]string
    ReplyTo     *Address
}
```

**Built-in providers:**

| Provider | Config | Use Case |
|----------|--------|----------|
| `console` | None | Dev — logs to stdout |
| `smtp` | `{host, port, username, password, tls}` | Self-hosted or org SMTP |
| `resend` | `{apiKey}` | Cloud API (recommended default) |
| `sendgrid` | `{apiKey}` | Cloud API alternative |

### Email Verification Flow

1. User registers → `users.email_verified` is `false`
2. Backend generates verification token (`email_verification_tokens` table, expires in 24h)
3. Sends verification email with link: `{FRONTEND_URL}/verify-email?token=xxx`
4. User clicks → frontend calls `POST /api/v1/auth/verify-email`
5. Backend validates token → sets `email_verified = true`
6. If not verified within 24h, user can request resend

**User model changes:**

```sql
ALTER TABLE users ADD COLUMN email_verified BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE users ADD COLUMN email_verified_at TIMESTAMPTZ;
```

### Notification Email Integration

The existing `notification_service.go` gains email delivery:

```go
func (s *NotificationService) deliverEmail(notification Notification) error {
    template, err := s.emailService.GetTemplate(notification.Type)
    // render template with notification variables
    // send via configured provider
}
```

Email notifications are opt-in per user (notification preferences from Spec 3). Default: only `urgent` priority notifications go to email.

### System Email Templates

| Template | Trigger | Variables |
|----------|---------|-----------|
| `email_verification` | Registration | `userName`, `verificationUrl` |
| `password_reset` | Forgot password | `userName`, `resetUrl` |
| `invitation` | Org/project invite | `inviterName`, `orgName`, `inviteUrl` |
| `task_assigned` | Task assigned | `assigneeName`, `taskTitle`, `projectName`, `taskUrl` |
| `task_overdue` | Task past due date | `assigneeName`, `taskTitle`, `dueDate`, `taskUrl` |
| `review_requested` | Review needed | `reviewerName`, `prTitle`, `reviewUrl` |
| `agent_completed` | Agent run done | `taskTitle`, `agentName`, `duration`, `resultUrl` |
| `agent_failed` | Agent run failed | `taskTitle`, `agentName`, `errorMessage`, `logsUrl` |
| `budget_warning` | 80% budget consumed | `projectName`, `consumed`, `budget`, `costUrl` |
| `budget_exceeded` | 100% budget consumed | `projectName`, `consumed`, `budget`, `costUrl` |
| `welcome` | First login | `userName`, `orgName`, `gettingStartedUrl` |
| `digest` | Hourly/daily digest | `userName`, `summary[]` |

### API Endpoints

```
# Email verification
POST   /api/v1/auth/verify-email              — Verify email (token in body)
POST   /api/v1/auth/resend-verification        — Resend verification email

# Password reset
POST   /api/v1/auth/forgot-password            — Request reset email
POST   /api/v1/auth/reset-password             — Reset with token

# Email config (org admin)
GET    /api/v1/orgs/:orgId/email/config         — Get email config
PUT    /api/v1/orgs/:orgId/email/config         — Update config
POST   /api/v1/orgs/:orgId/email/test           — Send test email

# Email templates (org admin)
GET    /api/v1/orgs/:orgId/email/templates      — List templates
GET    /api/v1/orgs/:orgId/email/templates/:name — Get template
PUT    /api/v1/orgs/:orgId/email/templates/:name — Update template
```

### Frontend Changes

**New pages:**
- `/verify-email` — email verification page
- `/forgot-password` — password reset request
- `/reset-password` — password reset form
- Org Settings → Email tab — SMTP/API config, template editor

**New components:**
- `EmailVerificationBanner` — shown when email not verified
- `EmailConfigForm` — SMTP/API provider configuration
- `EmailTemplateEditor` — Monaco-based template editor with variable injection

**Modified:**
- Registration flow → sends verification email, shows "check your email" page
- Login → show banner if email not verified
- Notification Preferences (Spec 3) → email channel toggles
- Settings → add "Email" section for password reset, email change

### Security

- SMTP credentials stored encrypted (AES-GCM via existing secrets infrastructure)
- Verification tokens are single-use, expire in 24h
- Password reset tokens are single-use, expire in 1h
- Rate limit: max 5 verification emails per hour per user
- Rate limit: max 3 password reset emails per hour per user
- Email delivery logs are immutable (audit trail)
- Unsubscribe link in every notification email (one-click unsubscribe per type)

### Testing

- Unit: template rendering, provider abstraction
- Integration: verification flow end-to-end, password reset flow
- Mock provider: `console` provider for tests, captures sent emails in memory
- E2E: register → verify email → login → change email → re-verify
- Template: verify all templates render without errors for all variable combinations
