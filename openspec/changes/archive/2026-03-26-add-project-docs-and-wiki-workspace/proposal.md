## Why

AgentForge has a strong task execution console (project workspace, workflow config, team/role governance) but lacks a first-class **Docs / Wiki / Knowledge** layer. There is no document space in the sidebar, no block editor, no page tree, no document versioning, and no way to create or consume structured knowledge alongside tasks. Teams that use AgentForge must store PRDs, ADRs, Runbooks, and design docs in external tools, breaking the feedback loop between documentation and task execution. Adding a project-scoped docs/wiki workspace closes this gap and moves AgentForge from a "task execution console" toward a "collaboration workstation."

## What Changes

- **New sidebar route** `/docs` with project-scoped wiki space, page tree navigation, favorites, pins, and recent-access list.
- **Block-level document editor** supporting headings, lists, to-dos, callouts, tables, code blocks, formula (KaTeX), diagrams (Mermaid), and embedded cards (task cards, agent cards, review cards).
- **Page tree management**: create, rename, move, delete, drag-reorder pages and sub-pages within a project wiki space.
- **Document comment threads**: inline comments, page-level comments, @-mentions, resolve/archive, comment permalinks.
- **Document version management**: save named versions, browse version history, restore a version, share read-only version links.
- **Template center**: built-in templates for PRD, RFC, ADR, Postmortem, Onboarding, Runbook, Agent Task Brief; user-created templates.
- **Backend persistence**: new Go models, repositories, handlers, and migrations for wiki spaces, pages, page versions, comments, and templates.
- **WebSocket events** for real-time page tree updates and comment notifications.

## Capabilities

### New Capabilities
- `docs-wiki-workspace`: Project-scoped wiki space with page tree, CRUD, ordering, favorites, pins, recent-access, and page-level permissions.
- `block-document-editor`: Block-level rich-text editor supporting headings, lists, to-dos, callouts, tables, code blocks, KaTeX formulas, Mermaid diagrams, and embedded entity cards.
- `document-comments`: Inline and page-level comment threads with @-mentions, resolve/archive lifecycle, and comment permalinks.
- `document-versioning`: Named version snapshots, version history browsing, version restore, and read-only version sharing links.
- `document-templates`: Built-in and user-created document templates (PRD, RFC, ADR, Postmortem, Onboarding, Runbook, Agent Task Brief).

### Modified Capabilities
- `desktop-notification-delivery`: Add notification channels for document comment mentions, page updates, and version publishes.
- `im-bridge-progress-streaming`: Stream document-related events (page created, comment added, version published) to IM channels.

## Impact

- **Frontend**: New `app/(dashboard)/docs/` route tree, new `components/docs/` component family, new `lib/stores/docs-store.ts`. Requires adding a block editor dependency (e.g. Tiptap or BlockNote).
- **Backend (src-go)**: New models (`wiki_space`, `wiki_page`, `page_version`, `page_comment`, `doc_template`), new repository/service/handler layers, new DB migrations.
- **WebSocket (src-go/internal/ws)**: New event types for page tree changes and comment activity.
- **Sidebar**: Add "Docs" entry to navigation with page tree flyout or sub-panel.
- **Dependencies**: `@tiptap/*` or `@blocknote/core` + `@blocknote/react` for the block editor; `katex` for formula rendering; `mermaid` for diagram rendering.
