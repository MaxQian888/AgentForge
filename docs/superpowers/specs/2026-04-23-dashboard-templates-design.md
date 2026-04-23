---
title: "Spec 11: Dashboard Templates"
date: 2026-04-23
status: draft
---

# Dashboard Templates

## Problem

AgentForge has highly customizable dashboards (85% complete) with widget positioning, sizing, and configuration. But users must build dashboards from scratch. There are no templates to start from, no sharing between users, and no community templates.

## Current State

- Dashboard CRUD: create, update, delete dashboards
- Widget system: 10+ widget types with configuration dialogs
- Layout persistence: position, size, configuration saved per dashboard
- Time range and category filters
- Auto-refresh
- No templates, no sharing, no marketplace

## Design

### Template Data Model

```sql
CREATE TABLE dashboard_templates (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id      UUID REFERENCES organizations(id),   -- nullable = public template
  created_by  UUID NOT NULL REFERENCES users(id),
  name        VARCHAR(128) NOT NULL,
  description TEXT,
  category    VARCHAR(32) NOT NULL,   -- engineering, pm, leadership, operations, custom
  icon        VARCHAR(64),            -- lucide icon name
  layout      JSONB NOT NULL,         -- widget layout (grid positions, sizes)
  widgets     JSONB NOT NULL,         -- widget configs (type, data source, display options)
  is_public   BOOLEAN NOT NULL DEFAULT false,
  use_count   INTEGER NOT NULL DEFAULT 0,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### Built-in Templates

| Template | Category | Widgets | Description |
|----------|----------|---------|-------------|
| Engineering Overview | engineering | Throughput chart, active agents, review queue, task status | Default for developers |
| Sprint Board | pm | Burndown chart, velocity, task status breakdown, budget | Sprint tracking |
| Executive Summary | leadership | KPI cards, cost trends, team health, agent performance | High-level overview |
| Agent Operations | operations | Agent pool, dispatch stats, queue depth, cost per agent | Agent management |
| Cost Optimization | operations | Budget forecast, spending trends, cost breakdown, velocity | Cost monitoring |
| Custom | custom | Empty canvas | Start from scratch |

### Template Creation

Two ways to create a template:

1. **Save existing dashboard as template** — from dashboard actions menu:
   - "Save as Template" → name, description, category, visibility (org/public)
   - Strips personal filters, keeps widget config and layout

2. **Create from scratch** — template editor:
   - Same interface as dashboard builder but saves as template
   - Can set placeholder data sources (e.g., "Select a project")

### Template API

```
# Template CRUD
GET    /api/v1/dashboard-templates                   — List templates (public + org)
GET    /api/v1/dashboard-templates/:id               — Get template
POST   /api/v1/dashboard-templates                   — Create template
PUT    /api/v1/dashboard-templates/:id               — Update template
DELETE /api/v1/dashboard-templates/:id               — Delete template

# Apply template
POST   /api/v1/dashboard-templates/:id/apply         — Create dashboard from template
Body: { "projectId": "uuid", "name": "My Dashboard" }

# Save dashboard as template
POST   /api/v1/dashboards/:did/save-as-template
Body: { "name": "...", "description": "...", "category": "...", "isPublic": false }

# Template usage
GET    /api/v1/dashboard-templates/:id/usage         — Usage stats
```

### Dashboard Sharing

Beyond templates, allow sharing dashboard configurations:

```sql
CREATE TABLE dashboard_shares (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  dashboard_id  UUID NOT NULL REFERENCES dashboards(id),
  shared_by     UUID NOT NULL REFERENCES users(id),
  share_token   VARCHAR(64) UNIQUE NOT NULL,  -- for link sharing
  share_type    VARCHAR(16) NOT NULL,         -- org, link
  permissions   VARCHAR(16) NOT NULL DEFAULT 'view', -- view, edit
  expires_at    TIMESTAMPTZ,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

**Sharing flow:**
1. Dashboard owner clicks "Share"
2. Chooses: org members (with role filter) or link sharing
3. Sets permission level (view-only or edit)
4. Optionally sets expiry
5. Shared users see dashboard in their "Shared with me" section

### Frontend Changes

**Template gallery:**
- Dashboard creation dialog shows template gallery first
- Categories: Engineering, PM, Leadership, Operations, Custom
- Each template shows preview thumbnail + widget count
- "Use Template" creates a new dashboard with the template's layout

**New components:**
- `DashboardTemplateGallery` — grid of template cards with category filters
- `DashboardTemplateCard` — template preview with name, category, widget count
- `SaveAsTemplateDialog` — name, description, category, visibility
- `DashboardShareDialog` — sharing configuration
- `SharedDashboardsList` — "Shared with me" section

**Modified:**
- Dashboard creation flow → template gallery as first step
- Dashboard actions → "Save as Template" and "Share" buttons
- Dashboard list → "Shared with me" tab

### Testing

- Unit: template serialization/deserialization, share token generation
- Integration: save dashboard as template → apply template → verify widget layout matches
- E2E: browse templates → apply → customize → save as new template → share with team member
