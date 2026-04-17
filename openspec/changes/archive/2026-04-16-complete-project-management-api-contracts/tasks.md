## 1. Project summary contract

- [x] 1.1 Extend the backend project read/list contract to return truthful project-management summary fields (`status`, `taskCount`, `agentCount`, and related canonical project entry data) with focused model/repository/handler coverage.
- [x] 1.2 Update `lib/stores/project-store.ts`, `app/(dashboard)/projects/page.tsx`, and `components/project/project-card.tsx` to consume the authoritative summary contract instead of frontend fallback defaults.

## 2. Workflow and template project context alignment

- [x] 2.1 Introduce the canonical explicit project-context contract for project-owned workflow/template API calls and wire the relevant routes/middleware so handlers can resolve and validate the current project scope.
- [x] 2.2 Add project ownership guards across workflow definition/template read, publish, clone, execute, update, and delete paths in the handler/service/repository seam, with focused backend tests for mismatch and missing-context failures.
- [x] 2.3 Update `lib/stores/workflow-store.ts` and workflow workspace consumers to call the canonical project-scoped contract consistently for template and definition operations.

## 3. Bootstrap and dashboard truth alignment

- [x] 3.1 Update dashboard/bootstrap aggregation inputs in `lib/stores/dashboard-store.ts` and related summary helpers so workflow-template readiness uses the same project-scoped data contract as the workflow workspace.
- [x] 3.2 Verify project entry, bootstrap handoff, and downstream workspace routing still preserve explicit project scope after the API contract alignment.

## 4. Regression coverage and verification

- [x] 4.1 Add or update focused frontend tests for project list summaries, bootstrap readiness, and workflow template scoping behavior.
- [x] 4.2 Add or update focused backend tests for project summary DTOs, explicit project-context resolution, and workflow/template ownership enforcement.
- [x] 4.3 Run scoped OpenSpec and code-level verification for the touched project-management seam and document any remaining unrelated repo baseline debt separately.
