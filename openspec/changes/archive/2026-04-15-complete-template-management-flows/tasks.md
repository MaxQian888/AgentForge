## 1. Document Template Contracts

- [x] 1.1 Extend the docs template backend contracts in `src-go` so document templates support blank-template creation, custom-template update/delete, source-aware list metadata, and guided page-from-template inputs.
- [x] 1.2 Extend `lib/stores/docs-store.ts` and related types to support template discovery filters, preview metadata, custom template CRUD, duplication, and page-from-template requests with explicit title and destination.
- [x] 1.3 Add built-in-template guards so shipped document templates stay immutable in place while still supporting preview, use, and duplicate-to-custom flows.

## 2. Document Template UI Flows

- [x] 2.1 Rebuild `components/docs/template-center.tsx` into a real template management workspace with search/filter, preview, create-from-scratch, duplicate, edit, and delete flows.
- [x] 2.2 Expand docs instantiation flows in the template picker and docs pages so "new page from template" captures title and target location before materializing the page.
- [x] 2.3 Reuse the docs editor route for custom template authoring and editing with explicit template-mode cues and safe behavior for built-in templates.

## 3. Workflow Template Contracts

- [x] 3.1 Extend workflow template repository, service, and handler contracts so the library resolves built-in and marketplace templates globally plus custom templates for the current project only.
- [x] 3.2 Add publish, update, duplicate, and delete operations for project-owned workflow templates using copy-on-publish lineage from saved workflow definitions rather than in-place status mutation.
- [x] 3.3 Extend `lib/stores/workflow-store.ts` and related types to support project-aware template listing, preview metadata, publish/manage actions, and clone or execute requests with scoped overrides.

## 4. Workflow Template UI Flows

- [x] 4.1 Expand `components/workflow/workflow-templates-tab.tsx` with search/filter, source badges, preview, compatibility cues, and source-aware manage or use actions.
- [x] 4.2 Add "publish as template" and custom-template management entry points to the current workflow list and editor flows without reopening the broader workflow builder redesign.
- [x] 4.3 Ensure workflow template clone and execute dialogs expose required variable overrides and clearly communicate whether the result is a project-owned custom workflow or an execution spawned from a shipped template.

## 5. Verification

- [x] 5.1 Add or update frontend unit tests for docs and workflow template management flows, guards, preview behavior, and state transitions.
- [x] 5.2 Add or update Go handler, service, and repository tests for template scoping, publish lineage, immutable-source guards, delete behavior, and instantiate semantics.
- [x] 5.3 Add Playwright coverage for one end-to-end docs template management path and one workflow template library path.
