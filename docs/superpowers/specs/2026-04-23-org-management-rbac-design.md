---
title: "Spec 1: Organization Management + Global RBAC"
date: 2026-04-23
status: draft
blocks: [2, 4, 5, 6, 7]
---

# Organization Management + Global RBAC

## Problem

AgentForge has project-level RBAC (owner/admin/editor/viewer) but no concept of Organization. Every user is flat; there is no multi-tenant isolation, no cross-project roles, and no global administrators. This blocks enterprise deployment where multiple teams share a platform instance but must be isolated.

## Current State

- **Project RBAC**: 50+ action IDs, four-tier roles, fine-grained permission matrix — production-ready
- **User model**: Flat `users` table with email/password, no org affiliation
- **Membership**: `members` table scoped to projects with `projectRole`
- **No org entity**: No `organizations` table, no org-level roles, no org-scoped resources

## Design

### Data Model

```sql
-- Organizations
CREATE TABLE organizations (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  slug        VARCHAR(64) UNIQUE NOT NULL,
  name        VARCHAR(256) NOT NULL,
  avatar_url  TEXT,
  plan        VARCHAR(32) NOT NULL DEFAULT 'free',  -- free/team/enterprise
  settings    JSONB NOT NULL DEFAULT '{}',
  created_by  UUID NOT NULL REFERENCES users(id),
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at  TIMESTAMPTZ
);

-- Organization membership
CREATE TABLE org_members (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id      UUID NOT NULL REFERENCES organizations(id),
  user_id     UUID NOT NULL REFERENCES users(id),
  role        VARCHAR(32) NOT NULL DEFAULT 'member', -- owner/admin/member/viewer
  joined_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(org_id, user_id)
);

-- Index for fast org lookup
CREATE INDEX idx_org_members_user ON org_members(user_id);
CREATE INDEX idx_org_members_org ON org_members(org_id);
```

### Global Role Hierarchy

```
Platform Admin (instance-wide)
  └── Organization Owner
       ├── Organization Admin
       │    └── Organization Member
       └── Organization Viewer

Project-level roles remain unchanged:
  Project Owner → Project Admin → Project Editor → Project Viewer
```

**Key rules:**
- Platform admins can manage any org and impersonate org admins
- Org owners/admins can create projects under the org
- Org members are auto-added as project viewers on new projects in the org
- Project roles are independent of org roles — a org viewer can be a project editor

### API Endpoints

```
# Organization CRUD
POST   /api/v1/orgs                          — Create org
GET    /api/v1/orgs                          — List my orgs
GET    /api/v1/orgs/:orgId                   — Get org
PUT    /api/v1/orgs/:orgId                   — Update org
DELETE /api/v1/orgs/:orgId                   — Delete org (soft)

# Org membership
POST   /api/v1/orgs/:orgId/members           — Invite member
GET    /api/v1/orgs/:orgId/members           — List members
PUT    /api/v1/orgs/:orgId/members/:uid      — Update role
DELETE /api/v1/orgs/:orgId/members/:uid      — Remove member

# Org invitations
POST   /api/v1/orgs/:orgId/invitations       — Create invitation
GET    /api/v1/orgs/:orgId/invitations       — List invitations
POST   /api/v1/orgs/:orgId/invitations/:id/revoke — Revoke

# Org projects (scoped listing)
GET    /api/v1/orgs/:orgId/projects          — List org's projects

# Org settings
GET    /api/v1/orgs/:orgId/settings          — Get settings
PUT    /api/v1/orgs/:orgId/settings          — Update settings
```

### Project-Org Relationship

Projects gain an optional `org_id` column:

```sql
ALTER TABLE projects ADD COLUMN org_id UUID REFERENCES organizations(id);
CREATE INDEX idx_projects_org ON projects(org_id) WHERE deleted_at IS NULL;
```

- Projects created without an org remain personal (backward compatible)
- Projects created within an org context get `org_id` set automatically
- Org admins can see all org projects; project-level RBAC still controls editing

### Middleware Changes

**Auth middleware chain** becomes:

```
JWT auth → org membership check (if org-scoped route) → project RBAC (if project-scoped route)
```

New middleware: `OrgRBAC` — checks `org_members.role` for the requested org. Action IDs follow the existing pattern:

```
org:read, org:update, org:delete
org:member:list, org:member:invite, org:member:update, org:member:remove
org:project:list, org:project:create
org:settings:read, org:settings:update
```

### Frontend Changes

**New pages:**
- `/orgs` — org list (user's orgs)
- `/orgs/:orgId` — org dashboard (members, projects, settings)
- `/orgs/:orgId/members` — member management
- `/orgs/:orgId/settings` — org settings

**Modified pages:**
- Project creation dialog gains org selector
- Sidebar adds org switcher
- Settings page adds org-level settings tab (for org admins)
- User menu shows current org context

**New stores:**
- `org-store.ts` — org CRUD, current org context
- `org-member-store.ts` — org membership management

### Migration Strategy

1. Deploy `organizations` and `org_members` tables (additive, no existing data touched)
2. Add `org_id` to `projects` table (nullable, existing projects stay personal)
3. New middleware runs only on `/api/v1/orgs/*` routes — no impact on existing flows
4. Existing project-level RBAC remains fully functional during migration
5. UI shows org context only when user belongs to at least one org

### Testing

- Unit: org service CRUD, membership logic, role inheritance
- Integration: API endpoint tests for all org routes
- E2E: org creation → invite member → create project → verify isolation
- RBAC matrix: verify each org role can/cannot perform each action
- Migration: test backward compatibility with projects that have no org_id
