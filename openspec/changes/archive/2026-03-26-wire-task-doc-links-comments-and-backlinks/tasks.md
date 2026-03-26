## 1. Database Migrations & Models

- [x] 1.1 Create migration for `entity_links` table (id, project_id, source_type, source_id, target_type, target_id, link_type ENUM(requirement,design,test,retro,reference,mention), anchor_block_id TEXT NULL, created_by, created_at, deleted_at)
- [x] 1.2 Create migration for `task_comments` table (id, task_id, parent_comment_id UUID NULL, body TEXT, mentions JSONB, resolved_at TIMESTAMP NULL, created_by, created_at, updated_at, deleted_at)
- [x] 1.3 Add indexes: compound on entity_links(source_type, source_id), compound on entity_links(target_type, target_id), btree on task_comments(task_id, created_at)
- [x] 1.4 Create Go model structs for EntityLink and TaskComment in `src-go/internal/model/`

## 2. Backend Repository Layer

- [x] 2.1 Implement `entity_link_repo.go` with Create, Delete, ListBySource, ListByTarget, UpsertMentionLinks, DeleteMentionLinksForSource
- [x] 2.2 Implement `task_comment_repo.go` with Create, GetByID, Update, SoftDelete, ListByTaskID
- [x] 2.3 Write repository unit tests

## 3. Backend Service Layer — Entity Links & Backlinks

- [x] 3.1 Implement `entity_link_service.go` with CreateLink, DeleteLink, ListLinksForEntity (bidirectional), GetRelatedDocs, GetRelatedTasks
- [x] 3.2 Implement backlink extraction function: parse `[[entity-id]]` from content string, return list of (entity_type, entity_id) tuples
- [x] 3.3 Integrate backlink extraction into wiki page save path (wiki_service.UpdatePage) — extract, diff, upsert/delete mention links in same transaction
- [x] 3.4 Integrate backlink extraction into task description save path (task_service) — same pattern
- [x] 3.5 Write service unit tests for link CRUD and backlink extraction

## 4. Backend Service Layer — Task Comments

- [x] 4.1 Implement `task_comment_service.go` with CreateComment, ReplyToComment, ResolveComment, ReopenComment, DeleteComment, ListComments
- [x] 4.2 Extract @-mentions from comment body and trigger notifications
- [x] 4.3 Write service unit tests

## 5. Backend Service Layer — Doc-Driven Decomposition

- [x] 5.1 Implement `doc_decomposition_service.go` with DecomposeTasksFromBlocks: accept page_id, block_ids, optional parent_task_id; create tasks linked to source page with anchor_block_id
- [x] 5.2 Write service unit tests

## 6. Backend Service Layer — Review Write-Back

- [x] 6.1 Add post-completion hook in review_service: on review complete, find linked docs (requirement/design), create version snapshot, append review findings blocks
- [x] 6.2 Handle optimistic locking conflict with retry
- [x] 6.3 Log write-back result in review log
- [x] 6.4 Write service unit tests

## 7. Backend Handlers & Routes

- [x] 7.1 Implement `entity_link_handler.go` with POST/GET/DELETE endpoints under `/api/v1/projects/:pid/links`
- [x] 7.2 Implement `task_comment_handler.go` with CRUD endpoints under `/api/v1/projects/:pid/tasks/:tid/comments`
- [x] 7.3 Implement decompose endpoint at `POST /api/v1/projects/:pid/wiki/pages/:id/decompose-tasks`
- [x] 7.4 Register all new routes in `routes.go`
- [x] 7.5 Write handler integration tests

## 8. WebSocket Events

- [x] 8.1 Define event types: link.created, link.deleted, task_comment.created, task_comment.resolved
- [x] 8.2 Broadcast events from entity_link_service and task_comment_service
- [x] 8.3 Write event broadcast tests

## 9. Frontend Store

- [x] 9.1 Create `lib/stores/entity-link-store.ts` — links state, create/delete link, list links for entity
- [x] 9.2 Create `lib/stores/task-comment-store.ts` — task comments state, CRUD, resolve/reopen
- [x] 9.3 Add WebSocket handlers for link and comment events
- [x] 9.4 Write store unit tests

## 10. Frontend — Task Detail Linked Docs Panel

- [x] 10.1 Create `components/tasks/linked-docs-panel.tsx` — shows related docs grouped by link type, with add/remove link actions
- [x] 10.2 Create `components/tasks/doc-link-picker.tsx` — modal to search and select a doc page to link
- [x] 10.3 Add inline doc preview (first 5 blocks) in task detail for linked requirement/design docs
- [x] 10.4 Integrate linked-docs-panel into `task-detail-content.tsx`

## 11. Frontend — Document Related Tasks Panel

- [x] 11.1 Create `components/docs/related-tasks-panel.tsx` — shows linked tasks with live status, assignee, due date
- [x] 11.2 Create `components/docs/task-link-picker.tsx` — modal to search and select a task to link
- [x] 11.3 Integrate related-tasks-panel into the wiki page layout

## 12. Frontend — Backlinks Panel

- [x] 12.1 Create `components/shared/backlinks-panel.tsx` — shows inbound mention-type links with entity title, type, and navigation
- [x] 12.2 Integrate backlinks panel into wiki page layout and task detail panel

## 13. Frontend — Task Comments

- [x] 13.1 Create `components/tasks/task-comments.tsx` — comment list with threads, resolve/reopen, @-mention autocomplete
- [x] 13.2 Create `components/tasks/task-comment-input.tsx` — input with @-mention autocomplete
- [x] 13.3 Integrate task-comments into task-detail-content.tsx

## 14. Frontend — Doc-Driven Decomposition

- [x] 14.1 Add "Create Tasks" context menu action in block editor for selected blocks
- [x] 14.2 Create `components/docs/decompose-tasks-dialog.tsx` — confirmation dialog showing blocks to decompose, optional parent task selector
- [x] 14.3 Show task-count badges on blocks that have generated tasks

## 15. Frontend — Board View Enhancements

- [x] 15.1 Add optional "Linked Docs" column to table/list view column config
- [x] 15.2 Add doc-preview popover on task card linked-doc indicator

## 16. IM Bridge — Message Conversion Actions

- [x] 16.1 Add "Save as Doc" IM action handler in im_action_execution: create wiki page from message content
- [x] 16.2 Add "Create Task" IM action handler: create task from message content with origin=im
- [x] 16.3 Write action handler tests
