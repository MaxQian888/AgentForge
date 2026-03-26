## Why

Tasks and documentation live in separate silos — tasks reference external links for specs, and documents have no awareness of which tasks they drive. This forces context-switching and breaks the feedback loop between "why we're doing it" (docs) and "what we're doing" (tasks). Wiring bidirectional links, comments, and backlinks between tasks and documents creates a closed-loop where every task can trace back to its requirement source, and every document knows which work it generated.

## What Changes

- **Task ↔ Doc bidirectional binding**: Each task can link to requirement pages, design pages, test pages, and retrospective pages. Each document page shows a "Related Tasks" panel with live status.
- **Backlink index**: Automatic backlink detection when a document references another document or a task (via `[[page-id]]` or `[[task-id]]` syntax). A backlinks panel on every page/task shows inbound references.
- **Document-driven task decomposition**: From any document page, select text or a section and "Create tasks from selection" to generate a sub-task tree linked back to the source paragraph.
- **Review → Doc write-back**: When a review completes, its findings are auto-appended as a section in the task's linked requirement or ADR page.
- **IM / meeting → Doc / Task**: Convert an IM message or meeting note into a new doc page or task via a single action, preserving the source link.
- **Task comment threads**: Add comment threads to task cards with @-mentions, resolve lifecycle, and links to related doc comments.
- **Inline doc preview in task detail**: Task detail panel shows an inline preview of linked documents without full navigation.

## Capabilities

### New Capabilities
- `task-doc-bidirectional-links`: Bidirectional link registry between tasks and document pages, with link-type taxonomy (requirement, design, test, retro), live status sync, and "Related Tasks" / "Related Docs" panels.
- `backlink-index`: Automatic backlink extraction from `[[entity-id]]` references in document content and task descriptions, with a backlinks panel on both page and task views.
- `doc-driven-task-decomposition`: Select document content to generate linked sub-tasks, preserving paragraph-level source traceability.
- `review-doc-writeback`: Automatic append of review findings to the task's linked document page (requirement or ADR).
- `task-comment-threads`: Comment threads on task cards with @-mentions, resolve/archive lifecycle, and cross-links to doc comments.

### Modified Capabilities
- `task-multi-view-board`: Add "Linked Docs" column option to board/list/table views; add doc-preview popover on task cards.
- `im-bridge-progress-streaming`: Add IM actions for "convert message to doc page" and "convert message to task."
- `deep-review-pipeline`: Add write-back step that pushes review findings into linked document pages.

## Impact

- **Backend (src-go)**: New `entity_link` and `backlink_index` tables; new `task_comment` table; new service layer for link management and backlink extraction; modifications to review service for write-back.
- **Frontend**: New `components/docs/related-tasks-panel.tsx`, `components/tasks/linked-docs-panel.tsx`, `components/tasks/task-comments.tsx`; modifications to task detail content and doc page layout.
- **WebSocket**: New events for link creation/deletion, backlink updates, comment activity.
- **IM Bridge**: New action handlers for message-to-doc and message-to-task conversion.
