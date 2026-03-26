## 1. Database Migrations & Models

- [x] 1.1 Create migration for `wiki_spaces` table (id, project_id, created_at, deleted_at)
- [x] 1.2 Create migration for `wiki_pages` table (id, space_id, parent_id, title, content JSONB, content_text TEXT, path TEXT, sort_order INT, is_template BOOL, template_category TEXT, is_pinned BOOL, created_by, updated_by, created_at, updated_at, deleted_at)
- [x] 1.3 Create migration for `page_versions` table (id, page_id, version_number INT, name TEXT, content JSONB, created_by, created_at)
- [x] 1.4 Create migration for `page_comments` table (id, page_id, anchor_block_id TEXT NULL, parent_comment_id UUID NULL, body TEXT, mentions JSONB, resolved_at TIMESTAMP NULL, created_by, created_at, updated_at, deleted_at)
- [x] 1.5 Create migration for `page_favorites` table (page_id, user_id, created_at) and `page_recent_access` table (page_id, user_id, accessed_at)
- [x] 1.6 Create Go model structs for WikiSpace, WikiPage, PageVersion, PageComment, PageFavorite, PageRecentAccess in `src-go/internal/model/`
- [x] 1.7 Add indexes: GIN on wiki_pages.content, btree on wiki_pages.path, btree on wiki_pages(space_id, parent_id, sort_order), unique on page_favorites(page_id, user_id)

## 2. Backend Repository Layer

- [x] 2.1 Implement `wiki_space_repo.go` with Create, GetByProjectID, Delete
- [x] 2.2 Implement `wiki_page_repo.go` with Create, GetByID, Update, SoftDelete, ListTree (by space_id), ListByParent, MovePage, UpdateSortOrder
- [x] 2.3 Implement `page_version_repo.go` with Create, ListByPageID, GetByID
- [x] 2.4 Implement `page_comment_repo.go` with Create, ListByPageID, Update, SoftDelete
- [x] 2.5 Implement `page_favorite_repo.go` and `page_recent_access_repo.go`
- [x] 2.6 Write repository unit tests for all repos

## 3. Backend Service Layer

- [x] 3.1 Implement `wiki_service.go` with CreateSpace (auto on project create), CreatePage, UpdatePage, DeletePage (cascade descendants), MovePage (with circular check), GetPageTree
- [x] 3.2 Implement page versioning logic: CreateVersion (snapshot + auto-increment), RestoreVersion (copy content + create restore version)
- [x] 3.3 Implement comment service: CreateComment, ResolveComment, ReopenComment, DeleteComment, ListComments
- [x] 3.4 Implement template service: SeedBuiltInTemplates, CreateTemplateFromPage, CreatePageFromTemplate, ListTemplates
- [x] 3.5 Implement favorites/pins/recent-access service
- [x] 3.6 Add auto-save conflict detection (optimistic locking via updated_at)
- [x] 3.7 Write service unit tests

## 4. Backend Handler & Routes

- [x] 4.1 Implement `wiki_handler.go` with all REST endpoints per spec (pages CRUD, move, versions, comments, templates)
- [x] 4.2 Register wiki routes in `src-go/internal/server/routes.go` under `/api/v1/projects/:pid/wiki/`
- [x] 4.3 Add project-create hook to auto-create wiki space and seed templates
- [x] 4.4 Write handler integration tests

## 5. WebSocket Events

- [x] 5.1 Define wiki event types in `src-go/internal/ws/events.go`: wiki.page.created, wiki.page.updated, wiki.page.moved, wiki.page.deleted, wiki.comment.created, wiki.comment.resolved
- [x] 5.2 Broadcast wiki events from wiki_service on mutations
- [x] 5.3 Write event broadcast tests

## 6. Frontend Store

- [x] 6.1 Create `lib/stores/docs-store.ts` with Zustand: page tree state, current page, page content, comments, versions, templates, favorites, recent access
- [x] 6.2 Implement API client functions for all wiki endpoints
- [x] 6.3 Implement WebSocket event handlers to update store on wiki events
- [x] 6.4 Write store unit tests

## 7. Frontend Page Tree & Navigation

- [x] 7.1 Add "Docs" entry to sidebar (`components/layout/sidebar.tsx`) with icon and route
- [x] 7.2 Create `app/(dashboard)/docs/page.tsx` as the docs landing page (recent access, favorites, pinned)
- [x] 7.3 Create `app/(dashboard)/docs/[pageId]/page.tsx` for individual page view
- [x] 7.4 Create `components/docs/page-tree.tsx` — collapsible tree with drag-and-drop reorder (using dnd-kit or similar)
- [x] 7.5 Create `components/docs/page-tree-item.tsx` — individual tree node with context menu (rename, move, delete, pin, favorite)
- [x] 7.6 Create `components/docs/docs-sidebar-panel.tsx` — sidebar sub-panel with search, tree, favorites, recent

## 8. Frontend Block Editor

- [x] 8.1 Install BlockNote dependencies (`@blocknote/core`, `@blocknote/react`, `@blocknote/shadcn`), KaTeX, Mermaid
- [x] 8.2 Create `components/docs/block-editor.tsx` — lazy-loaded BlockNote editor wrapper with auto-save (2s debounce)
- [x] 8.3 Create custom KaTeX formula block extension
- [x] 8.4 Create custom Mermaid diagram block extension
- [x] 8.5 Create custom embedded entity card block (task card, agent card, review card) with live data fetching
- [x] 8.6 Create `components/docs/editor-toolbar.tsx` — toolbar with version save, template save, share actions
- [x] 8.7 Create `components/docs/editor-loading-skeleton.tsx` — skeleton shown while editor loads

## 9. Frontend Comments

- [x] 9.1 Create `components/docs/comments-panel.tsx` — side panel listing page-level and inline comments with threads
- [x] 9.2 Create `components/docs/comment-thread.tsx` — individual comment thread with reply, resolve, reopen, permalink
- [x] 9.3 Create `components/docs/comment-input.tsx` — comment input with @-mention autocomplete
- [x] 9.4 Integrate inline comment highlights in block editor (highlight blocks with comments)
- [x] 9.5 Implement detached comments section for orphaned inline comments

## 10. Frontend Versioning

- [x] 10.1 Create `components/docs/version-history-panel.tsx` — list of named versions with timestamps and creators
- [x] 10.2 Create `components/docs/version-viewer.tsx` — read-only renderer for viewing a specific version
- [x] 10.3 Implement version restore with confirmation dialog
- [x] 10.4 Implement read-only version share link generation

## 11. Frontend Templates

- [x] 11.1 Create `components/docs/template-picker.tsx` — modal for selecting a template when creating a new page
- [x] 11.2 Create `components/docs/template-center.tsx` — page listing all templates with create/edit/delete actions
- [x] 11.3 Implement "Save as Template" action on page toolbar

## 12. Notification Integration

- [x] 12.1 Add wiki notification event types to notification store and desktop notification handler
- [x] 12.2 Add wiki event types to IM bridge event forwarding config
- [x] 12.3 Write integration tests for notification delivery on comment mentions and page updates
