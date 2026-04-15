## Why

AgentForge already has project settings, task workspace, team management, docs, workflows, sprints, reviews, and dashboards, but project creation still drops operators into a fragmented experience with no canonical setup path. Users can create a project, lose project scope during navigation, land on no-project placeholders, or miss key setup steps, so "project management" still behaves like disconnected tools instead of one end-to-end control plane.

## What Changes

- Add a focused project bootstrap and handoff lifecycle that guides a newly created or partially configured project across settings, team, docs and templates, workflows, planning, and delivery surfaces.
- Define canonical project-scoped handoff actions and focus intents so navigation from `/projects`, dashboard insights, and bootstrap states preserves project context instead of relying only on ambient selection.
- Surface truthful setup readiness phases and blocking conditions using derived project state, without auto-creating fake tasks, teams, dashboards, or workflows just to satisfy a checklist.
- Keep the scope anchored to existing project-management surfaces; this change coordinates them rather than replacing task, team, dashboard, or template workspaces.

## Capabilities

### New Capabilities
- `project-bootstrap-handoff`: Guided project bootstrap lifecycle, readiness tracking, and project-scoped handoff actions across the existing management surfaces.

### Modified Capabilities
- `dashboard-insights`: Dashboard summary and empty states become lifecycle-aware entry points that preserve project scope and route operators into the next required setup or delivery surface.

## Impact

- Frontend project entry and navigation surfaces such as `app/(dashboard)/projects/page.tsx`, `app/(dashboard)/page.tsx`, shared route helpers, and adjacent workspace links.
- Project-scoped workspaces that currently depend on ambient selection, including settings, team, workflow, docs, sprints, task workspace, and project dashboard surfaces.
- Backend or store aggregation seams that summarize project readiness from existing project, settings, member, workflow, docs, and planning data.
- Focused frontend and backend verification for project creation handoff, bootstrap status derivation, and scope-preserving navigation.
