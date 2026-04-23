---
title: "Spec 6: Billing & Subscriptions"
date: 2026-04-23
status: draft
depends_on: [1]
---

# Billing & Subscriptions

## Problem

AgentForge has comprehensive cost tracking (token usage, per-task costs, budget monitoring) but no commercial billing layer. There are no subscription plans, no usage metering for billing, no payment processing, and no invoice generation. Enterprise deployment requires a monetization layer.

## Current State

- `cost_service.go` — tracks token usage, costs per task/project/sprint
- Budget governance with 80%/100% thresholds
- Cost CSV export
- Cost visualization (spending trends, agent performance, velocity charts)
- No subscription, billing, or payment infrastructure

## Design

### Subscription Plans

```typescript
type PlanId = "free" | "team" | "enterprise";

interface Plan {
  id: PlanId;
  name: string;
  price: { monthly: number; annual: number }; // USD
  limits: {
    agents: number;           // max concurrent agents
    projects: number;         // max projects
    members: number;          // max org members
    storageGb: number;        // max storage
    apiCallsPerMonth: number; // rate limit budget
    tokensPerMonth: number;   // included token budget (0 = unlimited)
  };
  features: string[];         // feature flags enabled by this plan
}
```

| Plan | Monthly | Agents | Projects | Members | Storage | Tokens/mo |
|------|---------|--------|----------|---------|---------|-----------|
| Free | $0 | 2 | 3 | 5 | 1 GB | 500K |
| Team | $49 | 10 | 25 | 25 | 50 GB | 5M |
| Enterprise | Custom | Unlimited | Unlimited | Unlimited | Unlimited | Unlimited |

### Data Model

```sql
-- Subscriptions
CREATE TABLE subscriptions (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id          UUID NOT NULL UNIQUE REFERENCES organizations(id),
  plan_id         VARCHAR(32) NOT NULL DEFAULT 'free',
  status          VARCHAR(16) NOT NULL DEFAULT 'active', -- active, past_due, canceled, expired
  billing_cycle   VARCHAR(16) NOT NULL DEFAULT 'monthly', -- monthly, annual
  current_period_start TIMESTAMPTZ NOT NULL,
  current_period_end   TIMESTAMPTZ NOT NULL,
  cancel_at_period_end BOOLEAN NOT NULL DEFAULT false,
  stripe_customer_id   VARCHAR(256),      -- nullable until payment method added
  stripe_subscription_id VARCHAR(256),
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Usage metering (materialized monthly)
CREATE TABLE usage_records (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id      UUID NOT NULL REFERENCES organizations(id),
  period      VARCHAR(7) NOT NULL,           -- "2026-04" (YYYY-MM)
  metric      VARCHAR(64) NOT NULL,          -- tokens_input, tokens_output, api_calls, storage_bytes, agents_peak
  value       BIGINT NOT NULL DEFAULT 0,
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(org_id, period, metric)
);

-- Invoices
CREATE TABLE invoices (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id      UUID NOT NULL REFERENCES organizations(id),
  stripe_invoice_id VARCHAR(256),
  amount_cents   INTEGER NOT NULL,           -- USD cents
  currency    VARCHAR(3) NOT NULL DEFAULT 'usd',
  status      VARCHAR(16) NOT NULL DEFAULT 'draft', -- draft, open, paid, void, uncollectible
  period_start TIMESTAMPTZ NOT NULL,
  period_end   TIMESTAMPTZ NOT NULL,
  line_items  JSONB NOT NULL DEFAULT '[]',
  pdf_url     TEXT,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  paid_at     TIMESTAMPTZ
);
```

### Usage Metering

The existing cost tracking system writes to `usage_records` as a side effect:

```go
// In cost_service.go — after recording task cost
func (s *CostService) recordUsage(ctx context.Context, orgID uuid.UUID, metric string, value int64) {
    period := time.Now().Format("2006-01")
    // UPSERT into usage_records, incrementing value
}
```

**Metered metrics:**

| Metric | Source | Unit |
|--------|--------|------|
| `tokens_input` | Agent runs | Count |
| `tokens_output` | Agent runs | Count |
| `api_calls` | API middleware | Count |
| `storage_bytes` | File uploads | Bytes |
| `agents_peak` | Agent pool | Concurrent count |

### Plan Enforcement

Middleware checks subscription limits before allowing actions:

```go
func PlanLimitGuard(metric string) echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            orgID := getOrgID(c)
            sub := subscriptionService.GetActive(orgID)
            usage := usageService.GetCurrent(orgID)
            plan := plans[sub.PlanID]

            if usage[metric] >= plan.Limits[metric] {
                return echo.NewHTTPError(http.StatusPaymentRequired, "Plan limit exceeded")
            }
            return next(c)
        }
    }
}
```

Applied to: agent spawn (agent limit), project creation (project limit), member invite (member limit), API calls (monthly limit).

### Payment Integration (Stripe)

Stripe handles all payment flows. AgentForge never touches raw card data.

**Stripe resources:**
- Customer → maps to Organization
- Subscription → maps to Subscription record
- Invoice → synced to Invoice record
- PaymentMethod → managed via Stripe Customer Portal

**Webhook handlers:**

```
POST /api/v1/billing/webhook (Stripe webhook)

Handled events:
  customer.subscription.created   → create subscription record
  customer.subscription.updated   → update plan/status
  customer.subscription.deleted   → mark canceled
  invoice.paid                    → mark invoice paid
  invoice.payment_failed          → mark past_due, send notification
  checkout.session.completed      → activate subscription
```

### API Endpoints

```
# Billing (org admin)
GET    /api/v1/orgs/:orgId/billing/subscription    — Current subscription
GET    /api/v1/orgs/:orgId/billing/usage            — Current period usage
GET    /api/v1/orgs/:orgId/billing/invoices         — Invoice history
GET    /api/v1/orgs/:orgId/billing/invoices/:id     — Invoice detail

# Checkout
POST   /api/v1/orgs/:orgId/billing/checkout         — Create checkout session
POST   /api/v1/orgs/:orgId/billing/portal           — Customer portal URL
POST   /api/v1/orgs/:orgId/billing/cancel           — Cancel subscription

# Plans (public)
GET    /api/v1/plans                                 — List available plans

# Webhook (no auth, Stripe signature verification)
POST   /api/v1/billing/webhook                       — Stripe webhook
```

### Frontend

**New pages:**
- `/orgs/:orgId/billing` — subscription overview, usage meters, plan comparison
- `/orgs/:orgId/billing/invoices` — invoice history with PDF download

**New components:**
- `SubscriptionCard` — current plan, usage bars, upgrade CTA
- `UsageMeter` — circular or linear progress for each metric vs limit
- `PlanComparison` — side-by-side plan features
- `InvoiceTable` — paginated invoice list with status badges

**Modified:**
- Org Settings → add Billing tab
- Sidebar → show plan badge next to org name
- Header → show usage warning when approaching limits

### Self-Hosted / No-Stripe Mode

For organizations that self-host AgentForge and don't want Stripe:

- `BILLING_PROVIDER=none` disables all Stripe integration
- All plan limits are configured via environment variables
- No payment processing, no invoices — limits only
- Useful for internal enterprise deployment with license-based access

### Testing

- Unit: usage metering aggregation, plan limit checks
- Integration: Stripe webhook handling (using Stripe test mode)
- Mock: Stripe client mock for local testing
- E2E: upgrade plan → usage meter updates → limit enforced → downgrade
- Edge: subscription expires mid-task-run (graceful degradation, not kill)
