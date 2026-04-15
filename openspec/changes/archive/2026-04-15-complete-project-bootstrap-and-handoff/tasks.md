## 1. Bootstrap Contract And Readiness Derivation

- [x] 1.1 Add a project bootstrap summary contract that derives lifecycle readiness from existing project, settings, member, docs, workflow, planning, and delivery state instead of a separate checklist store.
- [x] 1.2 Expose the bootstrap summary and recommended handoff actions through the project-entry and dashboard aggregation seams used by the current frontend.
- [x] 1.3 Define backward-compatible route-context helpers so explicit project handoffs can override stale ambient selection safely.

## 2. Project Entry And Dashboard Handoff UX

- [x] 2.1 Update the project creation and `/projects` entry flow so the created or selected project becomes the active bootstrap scope with visible next-step guidance.
- [x] 2.2 Update dashboard insights and empty states to render lifecycle-aware next actions for incomplete projects instead of generic static shortcuts only.
- [x] 2.3 Add project-scoped handoff actions from bootstrap and dashboard entry surfaces into settings, team, docs, workflow, planning, and visibility workspaces.

## 3. Destination Scope Consumption

- [x] 3.1 Make project-scoped destination workspaces consume explicit project context before falling back to ambient `selectedProjectId` or no-project placeholders.
- [x] 3.2 Add destination-specific focus handling for the first lifecycle intents, such as governance setup, member onboarding, workflow templates, and first-task or first-sprint creation.
- [x] 3.3 Refresh derived bootstrap state after key setup actions so resolved phases stop prompting while unresolved phases remain visible.

## 4. Verification And Operator Docs

- [x] 4.1 Add focused frontend and backend tests for project creation handoff, bootstrap state derivation, and explicit project-context navigation.
- [x] 4.2 Add focused tests for lifecycle-aware dashboard guidance and destination intent consumption across the affected workspace family.
- [x] 4.3 Update operator-facing documentation or API notes so project bootstrap and handoff semantics are documented alongside the existing project-management surfaces.
