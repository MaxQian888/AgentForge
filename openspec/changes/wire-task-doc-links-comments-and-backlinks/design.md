## Context

AgentForge already has a task workspace (board, list, timeline, calendar, dependencies) and will soon have a docs/wiki workspace (from `add-project-docs-and-wiki-workspace`). However, these two systems are currently independent — there is no linking between tasks and documents, no backlink index, and no way to derive tasks from documents or feed review results back into docs. This change wires them together.

**Dependency**: This change assumes the docs/wiki workspace (wiki_page, page_comment models) already exists. Implementation should be sequenced after `add-project-docs-and-wiki-workspace`.

## Goals / Non-Goals

**Goals:**
- Bidirectional links between tasks and document pages with typed relationships (requirement, design, test, retro).
- Automatic backlink index extracted from `[[entity-id]]` references in doc content and task descriptions.
- Document-driven task decomposition (select doc content → generate linked tasks).
- Review findings auto-written to linked document pages.
- Task comment threads with @-mentions and resolve lifecycle.
- Inline document preview in the task detail panel.

**Non-Goals:**
- Full-text search across linked entities — deferred to a unified search change.
- Automatic link suggestions / AI-powered link detection — future enhancement.
- Cross-project linking — links are scoped within a single project.
- Real-time collaborative editing of comments — standard request/response.

## Decisions

### D1: Entity link model → Generic `entity_link` table

**Choice**: A single `entity_link` table with `source_type`, `source_id`, `target_type`, `target_id`, `link_type`, and `created_by`.

**Alternatives considered**:
- **Per-pair join tables** (task_page_link, page_page_link): Simpler schema per table but proliferates tables as entity types grow.
- **Graph database**: Powerful for traversal but adds infrastructure complexity.

**Rationale**: A generic link table supports task↔page, page↔page, task↔task, and future entity types without schema changes. `link_type` enum (requirement, design, test, retro, reference, mention) provides semantic context. Compound index on `(source_type, source_id)` and `(target_type, target_id)` ensures fast bidirectional lookups.

### D2: Backlink extraction → Content-save hook

On every wiki page save and task description save, a service extracts `[[entity-id]]` patterns from the content and upserts entries in `entity_link` with `link_type = 'mention'`.

**Rationale**: Server-side extraction ensures backlinks are always consistent regardless of client. The extraction runs in the same transaction as the content save.

### D3: Document-driven task decomposition → Backend service

The client sends a selected text range (block IDs + optional text excerpt) to `POST /api/v1/projects/:pid/wiki/pages/:id/decompose-tasks`. The backend creates sub-tasks with a link back to the source page and block.

**Alternatives considered**:
- **Client-side decomposition**: Simpler but can't leverage AI/LLM decomposition services.
- **Async job**: Better for AI but adds latency.

**Rationale**: Synchronous service for now (manual decomposition). AI-assisted decomposition can be added as an async option later. Each created task gets an `entity_link` with `link_type = 'requirement'` pointing to the source page and stores `anchor_block_id` for paragraph-level traceability.

### D4: Review write-back → Post-review hook in review service

After a review completes, the review service checks if the reviewed task has a linked document (type = 'requirement' or 'design'). If so, it appends a "Review Findings" section to the document as new blocks.

**Rationale**: Automatic write-back closes the feedback loop. The appended section includes a link back to the review for provenance. If no linked doc exists, the write-back is skipped silently.

### D5: Task comments → Separate `task_comment` table

`task_comment` mirrors the `page_comment` schema: `task_id`, `parent_comment_id`, `body`, `mentions`, `resolved_at`, `created_by`.

**Rationale**: Keeping task comments separate from page comments avoids polymorphic complexity. The API and UI patterns are identical, enabling shared frontend components.

### D6: Inline doc preview in task detail → Fetch first N blocks

When task detail panel loads and the task has linked docs, the frontend fetches the first 5 blocks of each linked page via `GET /api/v1/projects/:pid/wiki/pages/:id?preview=true&block_limit=5`.

**Rationale**: Preview mode avoids fetching entire documents. The backend returns truncated content with a "View full page" link.

## Risks / Trade-offs

- **[Backlink extraction performance]** → Regex scan on every save. Mitigate: content is typically <100KB; extraction is <10ms. Monitor and add async path if latency appears.
- **[Review write-back conflicts]** → Writing to a doc that someone is currently editing. Mitigate: use the same optimistic locking as normal edits; write-back creates a new version if conflict detected.
- **[Link orphaning]** → Deleting a task or page leaves dangling links. Mitigate: soft-delete cascades mark links as `deleted_at` but don't remove them; UI shows "Linked entity was deleted."
- **[Decomposition quality]** → Manual decomposition may produce poorly structured tasks. Mitigate: provide a "suggested structure" hint based on document headings; AI option deferred.
