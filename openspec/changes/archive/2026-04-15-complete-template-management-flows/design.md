## Context

AgentForge currently has two existing template systems:

- Document templates are stored as wiki pages with `is_template = true` and surfaced through `GET /api/v1/projects/:pid/wiki/templates`, `CreateTemplateFromPage`, and `CreatePageFromTemplate`.
- Workflow templates are stored in `workflow_definitions` with `status = template`, seeded on startup, and surfaced through list/clone/execute endpoints plus the workflow page templates tab.

Both systems are incomplete in materially different ways.

- The archived `add-project-docs-and-wiki-workspace` change explicitly tracked a `template-center` with create/edit/delete actions, and the main `document-templates` spec still promises create-from-scratch and delete behavior. The live docs UI only shows a simple gallery and picker; there is no create-from-scratch flow, no preview/edit/delete workspace, and page-from-template currently materializes a page with almost no operator control.
- The workflow template surface is more recent, but it is still a minimal library. `ListTemplates()` currently queries every `status = template` record globally, so project-owned custom templates would leak across projects once publish flows exist. The current workflow UI can browse, clone, and execute templates, but it cannot publish a saved workflow as a reusable template, manage a project-owned template, or show source/compatibility context before use.
- There is an active broad `enhance-frontend-panel` change with a workflow visual-builder template gallery seam, but that change is much wider than "template management completeness" and should not absorb docs/wiki template drift, backend scoping, or publish/manage contracts.

This change is intentionally narrow: complete the already-existing docs template center and workflow template library so both become truthful, source-aware management flows.

## Goals / Non-Goals

**Goals:**
- Make the docs template center match the existing `document-templates` contract: create from scratch, edit/duplicate/delete custom templates, preview before use, and guarded built-in template handling.
- Make docs "new page from template" a guided instantiation flow that collects title and target location instead of silently creating a generic page.
- Make workflow template discovery project-aware so global built-in or marketplace templates can coexist with current-project custom templates without cross-project leakage.
- Add workflow "publish as template" plus edit/delete management for project-owned templates while preserving immutable built-in and marketplace templates.
- Keep source, preview, and guard semantics understandable across both template systems so operators can tell what is reusable, editable, or immutable before they act.

**Non-Goals:**
- Do not redesign the block editor, wiki page tree, or document versioning system beyond what template management needs.
- Do not reopen the broader workflow builder canvas or attempt to finish all of `enhance-frontend-panel`.
- Do not create a generic cross-domain "template registry" abstraction shared by docs, workflows, roles, plugins, and forms.
- Do not add public template sharing, cross-project template marketplace publishing, or analytics-heavy template ranking in this change.
- Do not change prior page copies or workflow clones retroactively when a source template is later edited or deleted.

## Decisions

### Decision: Keep docs and workflow templates as separate capabilities with shared operator conventions

Docs templates and workflow templates solve a similar product problem, but they sit on different storage models, editors, and runtime actions. This change will not invent a new generic template engine. Instead, it will apply shared operator conventions across both surfaces:

- search and filter
- preview before use
- visible source or provenance badges
- clear immutability guards for built-in or marketplace content
- "customize by copying" instead of mutating canonical shipped templates

This keeps the implementation aligned with current repo seams while still making the product feel coherent.

**Alternatives considered:**
- Build one generic `template-management` subsystem for every product area. Rejected because it would expand scope into unrelated domains and force a new abstraction before the current seams are even truthful.
- Leave docs and workflow template UX completely ad hoc. Rejected because the current drift is already a usability and maintenance problem.

### Decision: Document templates remain wiki pages, and management reuses the existing docs editor surface

Document templates already live in `wiki_pages` with template flags. The cheapest truthful completion path is to keep that model and make the template center a first-class workspace on top of it:

- creating a blank custom template creates a template-mode wiki page record
- editing a custom template reuses the current docs page/editor flow with an explicit template-management mode
- built-in templates remain previewable and instantiable, but not editable or deletable in place
- duplication creates a project-owned custom template copy that can then be edited safely

This matches the original docs/wiki design decision that templates are special wiki pages instead of a separate storage system.

**Alternatives considered:**
- Add a dedicated `doc_templates` table. Rejected because it duplicates the wiki page model and makes the archived design drift worse, not better.
- Edit templates in lightweight modal fields only. Rejected because template content is already block-editor content, so a reduced editor path would be false completeness.

### Decision: Workflow templates use copy-on-publish on top of `workflow_definitions`

Workflow templates already use the same persisted model as workflow definitions, distinguished by `status = template`, `category`, `project_id`, `template_vars`, and `source_id`. This change keeps that model but makes publish and management truthful:

- "Publish as template" copies a saved workflow definition into a new `status = template` record instead of mutating the original definition in place
- the published template stores lineage back to the source definition via `source_id`
- editing or deleting a custom workflow template only affects that template record, not the original workflow definition or previously cloned workflows
- clone or execute remains copy-on-use, preserving current runtime behavior

This avoids the failure mode where a normal workflow disappears from the definitions list just because the operator wanted a reusable template.

**Alternatives considered:**
- Turn an existing workflow definition into a template by changing its status in place. Rejected because it changes semantics of an existing workflow record and complicates active usage.
- Create a new dedicated workflow-template table. Rejected because the existing workflow definition model already has the fields needed for source, version, category, and template vars.

### Decision: Workflow template listing becomes project-aware while keeping global template sources

The current `ListTemplates()` repository query returns every template globally. That is tolerable only while templates are system-seeded. Once project-owned custom templates exist, the listing contract must distinguish:

- global templates: built-in system templates and marketplace-installed templates
- project-owned custom templates: templates whose ownership belongs to the active project only

The workflow template library contract will therefore require current project context for list and manage operations and must never surface another project's custom templates. The implementation can continue using the repo's current header-based project context conventions for template actions, but the capability contract is project-aware regardless of route shape.

**Alternatives considered:**
- Keep listing global and trust the UI to hide foreign templates later. Rejected because it is a data-leak bug waiting to happen.
- Split workflow templates into completely separate global and project routes. Possible, but unnecessary as a product contract requirement; the underlying rule is project-aware resolution.

### Decision: Template actions are source-aware and preview-first

Both template systems need a preview-first action model:

- preview shows source, category, key metadata, and what variables or content will be copied
- built-in or marketplace templates expose use/clone/duplicate actions, not destructive edits
- custom templates expose edit, duplicate, and delete actions within the current project
- instantiation dialogs collect the last required metadata before materializing a page or execution

For docs, that means title and destination before creating a page from a template. For workflows, that means required variable overrides, source context, and any immutability or compatibility cues before clone or execute.

**Alternatives considered:**
- Continue direct one-click create or execute actions everywhere. Rejected because it hides important context and is one reason current template flows feel incomplete.

### Decision: Verification must cover scoping, guards, and happy paths, not only rendering smoke tests

Current tests prove only that template components render or that list handlers respond. This change will require targeted verification at the behavior seam:

- docs template create, edit, duplicate, delete, and use flows
- workflow template project scoping, publish lineage, immutable source guards, and clone/execute with overrides
- at least one browser-level path for docs template management and one for workflow template library behavior if the repository Playwright harness is available

This is necessary because template management bugs are usually contract bugs, not visual-only bugs.

## Risks / Trade-offs

- [Shared UX conventions drift between docs and workflow surfaces] → Keep shared source/filter/preview semantics small and explicit rather than inventing a deep generic abstraction.
- [Workflow custom templates leak across projects] → Enforce project-aware repository queries and handler guards before any UI publish flow ships.
- [Template-mode editing could confuse normal docs or workflow authoring] → Add explicit template-mode banners, source badges, and action restrictions so operators know whether they are editing a reusable template or a normal page/definition.
- [Built-in template mutation could corrupt shipped baselines] → Keep built-in and marketplace templates immutable in place and route customization through duplicate/publish-copy flows only.
- [Focused change overlaps the broad active frontend panel work] → Keep this change out of workflow canvas redesign and limit workflow work to the existing library and list/editor seams.

## Migration Plan

1. Extend docs template contracts and UI around the existing wiki page model without rewriting stored template content.
2. Add workflow publish/manage contracts on top of `workflow_definitions`, preserving existing built-in template IDs and clone/execute behavior.
3. Change workflow template listing semantics to require current project context before user-owned custom templates are introduced.
4. Roll out source-aware UI actions after backend guards are in place so built-in and marketplace templates cannot be edited accidentally.
5. If the new publish/manage flow causes regressions, hide publish/edit/delete actions and fall back temporarily to the current browse/clone/execute behavior while keeping the project-aware listing fix.

## Open Questions

- Should docs template duplication preserve the full original title by default or force an explicit rename during the duplicate flow?
- Should workflow template preview show static compatibility notes only, or also compute live warnings against the current project's enabled runtimes and roles in the first iteration?
