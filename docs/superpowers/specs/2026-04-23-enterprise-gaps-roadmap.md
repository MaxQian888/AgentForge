---
title: Enterprise Feature Gaps — Master Roadmap
date: 2026-04-23
status: approved
---

# Enterprise Feature Gaps — Master Roadmap

## Context

AgentForge targets enterprise-grade deployment (multi-org, multi-team, commercial). This roadmap identifies 11 feature gaps between current implementation and enterprise readiness, prioritized by impact, blocking relationships, and foundational depth.

## Priority Tiers

### Tier 1 — Foundation (blocks everything else)

| # | Gap | Impact | Blocks | Current |
|---|-----|--------|--------|---------|
| 1 | Organization Management + Global RBAC | Critical | 2,4,5,6,7 | 50% |
| 2 | API Key Management | High | External integrations | 20% |

### Tier 2 — Core Experience

| # | Gap | Impact | Blocks | Current |
|---|-----|--------|--------|---------|
| 3 | Notification Center UI | High | User engagement | 70% |
| 4 | Global Search | High | Enterprise usability | 35% |
| 5 | Email System | High | Verification, alerts | 15% |

### Tier 3 — Operations

| # | Gap | Impact | Blocks | Current |
|---|-----|--------|--------|---------|
| 6 | Billing & Subscriptions | Medium | Commercial viability | 40% |
| 7 | Data Export / Import | Medium | Compliance, migration | 25% |
| 8 | Onboarding Flow | Medium | Adoption | 20% |

### Tier 4 — Product Differentiation

| # | Gap | Impact | Blocks | Current |
|---|-----|--------|--------|---------|
| 9 | Multi-Agent Collaboration | High (product) | PRD core promise | 0% |
| 10 | Cross-Session Memory | Medium (product) | Agent continuity | 0% |
| 11 | Dashboard Templates | Low | Polish | 85% |

## Dependency Graph

```
Organization + Global RBAC (1)
  ├── API Key Management (2)
  ├── Email System (5)
  ├── Billing & Subscriptions (6)
  ├── Global Search (4)
  └── Data Export / Import (7)

Notification Center UI (3) — independent, can parallel with Batch 1
Onboarding Flow (8) — independent
Multi-Agent Collaboration (9) — independent
Cross-Session Memory (10) — independent
Dashboard Templates (11) — independent
```

## Delivery Batches

| Batch | Gaps | Dependencies | Est. Effort |
|-------|-------|-------------|-------------|
| 1 | Org Management + Global RBAC | None | 2-3 weeks |
| 2 | API Keys + Notification Center UI | Batch 1 | 1.5-2 weeks |
| 3 | Global Search + Email System | Batch 1 | 2 weeks |
| 4 | Billing + Data Export/Import | Batch 1 | 2-3 weeks |
| 5 | Onboarding + Dashboard Templates | None | 1-2 weeks |
| 6 | Multi-Agent Collaboration + Cross-Session Memory | None | 3-4 weeks |

## Spec Files

Each gap has its own design spec:

1. [Organization Management + Global RBAC](2026-04-23-org-management-rbac-design.md)
2. [API Key Management](2026-04-23-api-keys-design.md)
3. [Notification Center UI](2026-04-23-notification-center-design.md)
4. [Global Search](2026-04-23-global-search-design.md)
5. [Email System](2026-04-23-email-system-design.md)
6. [Billing & Subscriptions](2026-04-23-billing-subscriptions-design.md)
7. [Data Export / Import](2026-04-23-data-export-import-design.md)
8. [Onboarding Flow](2026-04-23-onboarding-flow-design.md)
9. [Multi-Agent Collaboration](2026-04-23-multi-agent-collaboration-design.md)
10. [Cross-Session Memory](2026-04-23-cross-session-memory-design.md)
11. [Dashboard Templates](2026-04-23-dashboard-templates-design.md)
