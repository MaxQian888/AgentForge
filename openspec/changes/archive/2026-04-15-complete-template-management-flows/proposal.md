## Why

AgentForge already exposes two template surfaces, but neither currently forms a truthful management flow. The archived docs/wiki delivery and the main `document-templates` spec promised a template center with real create/edit/delete behavior, while the live docs UI only lists templates and instantiates them; the workflow page now shows templates, but the library is still starter-oriented, globally listed, and missing publish/preview/manage flows for project-owned templates. A focused completeness change is needed now so template management stops drifting between archived specs, active frontend work, and actual product behavior.

## What Changes

- Complete the docs template center as a real management workspace with search/filter, preview, create-from-scratch, edit/duplicate/delete for custom templates, and source-aware behavior for built-in templates.
- Add a guided "new page from template" flow for docs so operators can inspect a template and provide title/location before instantiating a new page.
- Introduce a dedicated workflow template library contract that merges built-in, marketplace, and current-project custom templates without leaking custom templates across projects.
- Add workflow "publish as template" and custom template management flows, including preview, variable inspection, edit/delete for project-owned templates, and immutable handling of built-in or marketplace templates.
- Add focused frontend, store, backend, and verification coverage for template scoping, preview, publish, edit, delete, and use flows without reopening the broader workflow builder redesign.

## Capabilities

### New Capabilities
- `workflow-template-library`: Project-aware workflow template discovery, preview, publishing, source-aware management, and clone or execute entry flows.

### Modified Capabilities
- `document-templates`: Expand the existing template-center and template-instantiation requirements so docs templates support truthful management completeness instead of only list/use/save flows.

## Impact

- Frontend docs surfaces: `app/(dashboard)/docs/page.tsx`, `app/(dashboard)/docs/[pageId]/page-client.tsx`, `components/docs/template-center.tsx`, `components/docs/template-picker.tsx`, and any new template preview or metadata dialogs that complete the current docs workflow.
- Frontend workflow surfaces: `app/(dashboard)/workflow/page.tsx`, `components/workflow/workflow-templates-tab.tsx`, workflow list/editor entry points, and any shared source-badge or preview components required for template management.
- State and API clients: `lib/stores/docs-store.ts`, `lib/stores/workflow-store.ts`, route helpers, translations, and focused tests around template-specific actions.
- Backend docs template contracts: `src-go/internal/model/wiki.go`, `src-go/internal/service/wiki_service.go`, `src-go/internal/handler/wiki_handler.go`, related repository code, and route wiring for template CRUD, preview metadata, and guarded built-in behavior.
- Backend workflow template contracts: `src-go/internal/model/workflow_definition.go`, `src-go/internal/repository/workflow_definition_repo.go`, `src-go/internal/service/workflow_template_service.go`, `src-go/internal/handler/workflow_handler.go`, and any marketplace-facing seams needed to surface built-in versus marketplace versus custom provenance.
