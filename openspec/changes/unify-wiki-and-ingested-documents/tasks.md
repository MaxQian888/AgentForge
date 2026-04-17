## 1. Schema & migration

- [ ] 1.1 Define GORM models `KnowledgeAsset`, `AssetVersion`, `AssetComment`, `AssetIngestChunk` under `src-go/internal/model/knowledge.go` with `kind` enum, nullable kind-specific columns, soft-delete, optimistic-lock `version`
- [ ] 1.2 Write migration that creates `knowledge_assets`, `asset_versions`, `asset_comments`, `asset_ingest_chunks`, and a Postgres `tsvector` column + trigger on `knowledge_assets`
- [ ] 1.3 Extend migration to copy `wiki_pages → knowledge_assets{kind=wiki_page}`, split `wiki_pages{is_template=true} → knowledge_assets{kind=template}`, copy `project_documents → knowledge_assets{kind=ingested_file}`
- [ ] 1.4 Migrate `page_versions → asset_versions` with `kind_snapshot`, `page_comments → asset_comments`, chunk storage → `asset_ingest_chunks`
- [ ] 1.5 Drop deprecated tables `wiki_pages`, `page_versions`, `page_comments`, `project_documents`, and their chunk tables at the end of the migration
- [ ] 1.6 Add repository-layer invariant checks (wiki_page requires content_json + wiki_space_id; ingested_file requires file_ref and null wiki_space_id/parent_id; template requires template_category)

## 2. Backend: knowledge package restructure

- [ ] 2.1 Create `src-go/internal/knowledge/` package; move `internal/document/*.go` parsers under `internal/knowledge/ingest/parsers/`
- [ ] 2.2 Create `KnowledgeAssetRepository` interface + Postgres implementation covering create/read/update/soft-delete/restore/list/tree/move/list-by-kind
- [ ] 2.3 Create `AssetVersionRepository`, `AssetCommentRepository`, `AssetIngestChunkRepository`
- [ ] 2.4 Create `KnowledgeAssetService` with `PrincipalContext` parameter on every method; implement RBAC gate (viewer/editor/admin) inside the service, not the handler
- [ ] 2.5 Delete `internal/service/wiki_service.go`; move remaining behaviors (tree ops, template seeding) into `KnowledgeAssetService`
- [ ] 2.6 Define `BlobStorage` interface for ingested-file binaries; ship a local-filesystem default; wire it through `KnowledgeAssetService.UploadIngestedFile`
- [ ] 2.7 Implement async ingest worker: consumes `pending` assets, runs kind-appropriate parser, persists chunks, populates `content_text`, transitions status, emits `knowledge.ingest.status_changed`
- [ ] 2.8 Implement `AssetVersionService.SnapshotForReupload` invoked before file reupload replaces asset state
- [ ] 2.9 Implement `MaterializeAsWiki` service method: creates sibling `kind=wiki_page` asset, prepopulates blocks from chunks, adds a header callout block, creates `entity_link{link_type=materialized_from}`
- [ ] 2.10 Implement `MarkMaterializationsAsSourceUpdated` hook that runs after successful reupload transitions
- [ ] 2.11 Define `SearchProvider` interface with Postgres-FTS default implementation; implement `(project_id, query, filters, cursor) → results`
- [ ] 2.12 Define `IndexPipeline` interface + `NoopIndexPipeline` default; wire subscriber that listens for `knowledge.asset.content_changed` events

## 3. Backend: HTTP routes & handlers

- [ ] 3.1 Create `internal/handler/knowledge_asset_handler.go` exposing `/api/v1/projects/:pid/knowledge/assets` CRUD, tree-list, move, soft-delete, restore
- [ ] 3.2 Add multipart upload handler for `POST /knowledge/assets` with `kind=ingested_file`
- [ ] 3.3 Add reupload handler `POST /knowledge/assets/:id/reupload`
- [ ] 3.4 Add materialize handler `POST /knowledge/assets/:id/materialize-as-wiki`
- [ ] 3.5 Add versioning handlers (`/versions` list, `/versions` create, `/versions/:vid/restore`)
- [ ] 3.6 Add comments handlers (`/comments` list/create, resolve, reopen, delete)
- [ ] 3.7 Add decompose-tasks handler `POST /knowledge/assets/:id/decompose-tasks`
- [ ] 3.8 Add search handler `GET /api/v1/projects/:pid/knowledge/search`
- [ ] 3.9 Delete `wiki_handler.go`, `project_document_handler.go` (or equivalents), and remove their route registrations from `internal/server/routes.go`
- [ ] 3.10 Update `task-doc-bidirectional-links` link handler to accept `target_kind` / `source_kind` fields when linking to knowledge assets

## 4. Backend: eventbus & WebSocket

- [ ] 4.1 Rename eventbus types `wiki.page.*`, `wiki.comment.*`, `document.*` → `knowledge.asset.*`, `knowledge.comment.*`, `knowledge.ingest.*`
- [ ] 4.2 Emit `knowledge.asset.content_changed` on every wiki_page/template content save
- [ ] 4.3 Emit `knowledge.ingest.status_changed` on every ingest status transition
- [ ] 4.4 Update `internal/ws/events.go` with the new event catalog and payload shapes
- [ ] 4.5 Audit IM-bridge and frontend WebSocket subscribers for references to the old names; update

## 5. Backend: tests

- [ ] 5.1 Unit tests for repository invariants (wiki_page requires content_json; ingested_file rejects wiki_space_id; template requires template_category)
- [ ] 5.2 Unit tests for service RBAC gate — viewer can read/search but cannot create; editor can update but cannot restore; admin can restore; agent-principal uses same rules
- [ ] 5.3 Service tests for create/update/move/delete tree cascades
- [ ] 5.4 Service tests for ingest pipeline (pending → processing → ready/failed; reupload snapshots version)
- [ ] 5.5 Service tests for `MaterializeAsWiki` including reject on non-ready, reject on wrong kind, decoupling after creation
- [ ] 5.6 Service tests for `SearchProvider` Postgres-FTS implementation — match on title, match on content_text, kind filter, updated-after filter, cursor pagination, soft-deleted excluded
- [ ] 5.7 Integration tests for HTTP handlers covering each spec scenario
- [ ] 5.8 Integration test: backlink extraction runs in-transaction for wiki_page; runs in ingest pipeline for ingested_file
- [ ] 5.9 Integration test: `review-doc-writeback` targets only wiki_page assets, skips otherwise

## 6. Frontend: store unification

- [ ] 6.1 Create `lib/stores/knowledge-store.ts` with `KnowledgeAsset` type discriminated by `kind`, plus kind-scoped selectors
- [ ] 6.2 Migrate `lib/stores/docs-store.ts` callers to `knowledge-store.ts` with `kind=wiki_page` filter
- [ ] 6.3 Migrate `lib/stores/document-store.ts` callers to `knowledge-store.ts` with `kind=ingested_file` filter
- [ ] 6.4 Delete `lib/stores/docs-store.ts` and `lib/stores/document-store.ts`
- [ ] 6.5 Add `useKnowledgeSearchStore` or extend `knowledge-store.ts` with search-result handling

## 7. Frontend: components & routes

- [ ] 7.1 Update `components/docs/*` to consume `knowledge-store`; rename internal types to `KnowledgeAsset` without changing user-visible copy
- [ ] 7.2 Add an "Uploads" pane under the docs workspace sidebar listing `kind=ingested_file` assets (separate from the wiki tree)
- [ ] 7.3 Wire the "Open as wiki page" action on ingested-file detail view to call the materialize endpoint
- [ ] 7.4 Add `materialized_from` pill to wiki-page header when the asset has that entity_link
- [ ] 7.5 Add "Materialized as" panel to ingested-file detail view listing linked wiki pages
- [ ] 7.6 Add `source_updated_since_materialize` hint banner on wiki pages that are flagged; support dismiss
- [ ] 7.7 Add a global search UI that calls `/knowledge/search` and groups results by `kind`
- [ ] 7.8 Add reupload affordance on ingested-file detail view

## 8. Frontend: tests

- [ ] 8.1 Port docs-store/document-store tests to knowledge-store tests
- [ ] 8.2 Component tests for uploads pane, materialize action, materializations panel, source-updated hint
- [ ] 8.3 Component tests for global search UI covering kind filters and pagination

## 9. Spec housekeeping (auto-handled by archive)

- [ ] 9.1 Verify new specs `knowledge-asset-model`, `knowledge-asset-ingestion`, `knowledge-asset-search`, `knowledge-asset-wiki-bridge` land under `openspec/specs/` on archive
- [ ] 9.2 Verify modified specs `docs-wiki-workspace`, `document-versioning`, `document-comments`, `document-templates`, `backlink-index`, `knowledge-index`, `review-doc-writeback`, `doc-driven-task-decomposition`, `task-doc-bidirectional-links` reflect the updated requirements after archive
- [ ] 9.3 Run `openspec verify --change unify-wiki-and-ingested-documents` before archiving

## 10. Smoke verification

- [ ] 10.1 Fresh-DB smoke: create a project, author a wiki page, upload a PDF, verify both appear in `GET /knowledge/assets`
- [ ] 10.2 Smoke: perform a search that hits both the wiki title and the PDF content
- [ ] 10.3 Smoke: `[[ingested-file-id]]` in a task description renders as a backlink on the file's detail view
- [ ] 10.4 Smoke: save a wiki page and observe `knowledge.asset.content_changed` being dispatched to `NoopIndexPipeline` (log/metric)
- [ ] 10.5 Smoke: materialize an uploaded PDF as a wiki page, edit the wiki page, reupload the PDF — the two remain decoupled and the wiki page shows the source-updated hint
- [ ] 10.6 Smoke: complete a review on a task linked to a wiki page — findings append + named version created; no write-back to linked ingested files
