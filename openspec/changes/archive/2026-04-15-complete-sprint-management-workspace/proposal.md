## Why

AgentForge already has a dedicated `/sprints` surface, sprint-aware task workspace filters, burndown metrics, milestone associations, and sprint budget query endpoints, but those pieces still behave like adjacent seams instead of one truthful planning workspace. Operators can see or edit sprint data, yet key flows still drift across frontend, store, and backend contracts, so sprint management is not yet a reliable control plane for planning and execution handoff.

## What Changes

- Add a project-scoped sprint management workspace contract for listing sprints, creating and editing sprint records, showing selected sprint health, and handing operators into sprint-scoped execution work.
- Define the sprint workspace as the planning control plane that surfaces burndown, budget detail, milestone association, and operator actions without duplicating the full task execution workspace.
- Align sprint create and update flows with the existing persisted sprint model so date ranges, milestone linkage, and status transitions round-trip truthfully between frontend and Go APIs.
- Add an explicit sprint-to-execution handoff that opens the existing project task workspace in the selected project and sprint scope instead of relying on ambient store state.
- Preserve compatibility with existing budget and milestone seams by consuming the authoritative sprint budget detail endpoint and existing milestone association model rather than introducing parallel sprint services.

## Capabilities

### New Capabilities
- `sprint-management-workspace`: Defines the operator-facing sprint planning workspace, selected sprint detail surface, milestone and budget visibility, and handoff into sprint-scoped task execution.

### Modified Capabilities
- `task-multi-view-board`: The project task workspace accepts explicit sprint-scoped handoff input so users can continue execution from the sprint workspace without reapplying sprint filters manually.

## Impact

- Frontend sprint planning surface such as `app/(dashboard)/sprints/page.tsx`, sprint-related UI components, route helpers, and localized copy for sprint workspace actions and empty states.
- Existing project task workspace entry flow in `app/(dashboard)/project/page.tsx` and shared task workspace filter initialization for sprint-scoped handoff.
- Sprint client and backend contracts in `lib/stores/sprint-store.ts`, `src-go/internal/handler/sprint_handler.go`, and related tests that currently govern sprint create, edit, metrics, and realtime updates.
- Existing budget and milestone seams including sprint budget detail consumers and sprint-to-milestone persistence paths, without introducing new backend resource types.
