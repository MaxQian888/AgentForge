## Context

AgentForge is a React 19 + Next.js 16 + Tauri 2.9 desktop app with a Go backend (PostgreSQL + Redis). The frontend uses Zustand stores, shadcn/ui components, and Tailwind v4. The backend follows a handler → service → repository layering with WebSocket event broadcasting. There is currently no document/wiki infrastructure — the "memory" page is a search interface over agent memory, not a document workspace.

This design covers adding a full project-scoped docs/wiki workspace with a block editor, page tree, comments, versioning, and templates.

## Goals / Non-Goals

**Goals:**
- Every project gets a wiki space with a hierarchical page tree.
- Users can create rich documents using a block editor with headings, lists, tables, code, math, diagrams, and embedded entity cards.
- Documents support inline and page-level comment threads with @-mentions.
- Named versions can be saved, browsed, compared, and restored.
- Built-in and user-created templates accelerate common document types.
- Real-time page tree and comment updates via WebSocket.

**Non-Goals:**
- Real-time collaborative editing (multi-cursor, CRDT) — deferred to a future change.
- Full-text search across all wiki spaces — deferred to a unified search change.
- Board/whiteboard canvas — separate change (`wire-task-doc-links-comments-and-backlinks` and a future board change).
- Document-level access control (page-level permissions) — deferred; initially all project members can read/write all pages.
- Offline document editing in Tauri — uses standard API; no offline-first sync.

## Decisions

### D1: Block editor library → BlockNote

**Choice**: Use `@blocknote/core` + `@blocknote/react` + `@blocknote/mantine` (or shadcn adapter).

**Alternatives considered**:
- **Tiptap**: More mature, but requires assembling extensions individually; heavier bundle for comparable features.
- **Plate**: shadcn-native but still early; plugin ecosystem smaller.
- **Custom ProseMirror**: Maximum control but enormous build effort.

**Rationale**: BlockNote provides a batteries-included block editor with built-in slash menu, drag handles, side menu, and a React-first API. It outputs a structured JSON block tree that maps cleanly to our versioning and search needs. The block-level granularity aligns with our plan for task-aware embedded blocks and sync blocks in future changes.

### D2: Document storage model → JSON block tree in `content` column

Each `wiki_page` row stores:
- `content JSONB` — the BlockNote JSON block array (source of truth)
- `content_text TEXT` — a flattened plain-text extraction for full-text search (generated on save)
- `content_html TEXT` — optional server-rendered HTML snapshot for read-only sharing

**Rationale**: JSONB gives us queryable block-level access for features like backlink extraction and embedded card resolution. Plain-text extraction supports PostgreSQL `tsvector` search without an external engine.

### D3: Page tree → Materialized path + `sort_order`

Each page has `parent_id UUID NULL`, `path TEXT` (materialized path like `/proj/abc/def`), and `sort_order INT` within its parent.

**Alternatives considered**:
- **Nested set**: Fast reads but expensive writes on reorder.
- **Closure table**: Good for deep queries but high storage overhead.
- **Adjacency list only**: Simple but requires recursive CTE for tree fetch.

**Rationale**: Materialized path gives O(1) subtree queries (`WHERE path LIKE '/proj/abc/%'`), fast reorders (update `sort_order`), and simple depth calculation. Combined with `parent_id` for direct-child queries.

### D4: Versioning → Snapshot table with diff-on-demand

`page_version` stores the full `content JSONB` snapshot for each named version. We do NOT store incremental diffs — snapshots are simpler to restore and the content size per page is manageable (typical doc < 100KB JSON).

**Rationale**: Named versions are infrequent (user-initiated), so snapshot storage cost is acceptable. Diff computation happens on-demand in the frontend using `json-diff` for version comparison views.

### D5: Comments → Threaded model with anchor

`page_comment` stores:
- `anchor_block_id TEXT NULL` — the block ID for inline comments (NULL = page-level)
- `parent_comment_id UUID NULL` — for threading
- `resolved_at TIMESTAMP NULL` — resolve lifecycle

**Rationale**: Block-level anchoring leverages BlockNote's stable block IDs. When a block is deleted, orphaned comments surface in a "detached comments" section rather than being lost.

### D6: Templates → Stored as special wiki pages

Templates are wiki pages with `is_template = true` and `template_category TEXT` (e.g., 'prd', 'adr', 'runbook'). Creating a page from a template copies the content blocks.

**Rationale**: This avoids a separate template storage system and lets users edit templates with the same block editor. Built-in templates are seeded on project creation.

### D7: API structure

```
GET    /api/v1/projects/:pid/wiki/pages          — list page tree
POST   /api/v1/projects/:pid/wiki/pages          — create page
GET    /api/v1/projects/:pid/wiki/pages/:id       — get page with content
PUT    /api/v1/projects/:pid/wiki/pages/:id       — update page
DELETE /api/v1/projects/:pid/wiki/pages/:id       — soft-delete page
PATCH  /api/v1/projects/:pid/wiki/pages/:id/move  — move/reorder page
GET    /api/v1/projects/:pid/wiki/pages/:id/versions  — list versions
POST   /api/v1/projects/:pid/wiki/pages/:id/versions  — create named version
GET    /api/v1/projects/:pid/wiki/pages/:id/versions/:vid  — get version content
POST   /api/v1/projects/:pid/wiki/pages/:id/versions/:vid/restore  — restore version
GET    /api/v1/projects/:pid/wiki/pages/:id/comments  — list comments
POST   /api/v1/projects/:pid/wiki/pages/:id/comments  — create comment
PATCH  /api/v1/projects/:pid/wiki/pages/:id/comments/:cid  — update/resolve comment
DELETE /api/v1/projects/:pid/wiki/pages/:id/comments/:cid  — delete comment
GET    /api/v1/projects/:pid/wiki/templates       — list templates
```

## Risks / Trade-offs

- **[BlockNote bundle size]** → BlockNote adds ~150KB gzipped. Mitigate with dynamic import (`next/dynamic`) so the editor only loads on doc pages.
- **[JSONB content size]** → Very large documents (>1MB JSON) may slow page loads. Mitigate with pagination hint in API and lazy block loading in a future iteration.
- **[No real-time collab]** → Last-write-wins on concurrent edits. Mitigate with optimistic locking via `updated_at` check and conflict toast. Real-time collab (Yjs/CRDT) is a future change.
- **[Template drift]** → If built-in templates are seeded once, they can't be updated centrally. Mitigate by marking built-in templates with `system = true` and allowing re-seed on project settings.
- **[Comment anchor stability]** → If a user restructures a document heavily, inline comment anchors may become orphaned. Mitigate by surfacing orphaned comments in a "detached" section.
