## Why

AgentForge currently runs two parallel and disconnected "document" concepts:

- **Wiki pages** (`lib/stores/docs-store.ts`, `src-go/internal/service/wiki_service.go`, `WikiSpace` / `WikiPage` / `PageVersion` / `PageComment`) — editable BlockNote JSON, the surface users actually author.
- **Ingested project documents** (`lib/stores/document-store.ts`, `src-go/internal/document/`, `ProjectDocument` with `chunkCount`, upload status, parser for `.docx/.xlsx/.pptx/.pdf`) — immutable source files parsed for RAG-style consumption.

They share the word "document" but share nothing else: not a store, not a model, not a search index, not a backlink graph, not an ACL, not a WebSocket channel. Consequences today:

- A PRD uploaded as `.docx` and a PRD authored inline live in different worlds; users can't find them in the same search.
- `knowledge-index` spec explicitly disclaims "repository cloning, symbol extraction, Bleve indexing, embeddings, and graph traversal are not yet guaranteed" — so wiki content never becomes part of the knowledge index agents actually consume.
- `[[entity-id]]` backlinks only resolve to wiki pages and tasks; ingested documents are invisible.
- Two stores, two naming conventions, duplicated update/delete/list code paths; every future feature (AI assist, search, sync, sharing) would have to be built twice.

We are in the internal-testing stage where breaking changes are freely permitted. Unifying now — before doc surface features multiply — eliminates the fork instead of letting it calcify.

## What Changes

- Introduce the unified abstraction **`KnowledgeAsset`** as the single first-class document entity in a project:
  - `kind = wiki_page` → editable BlockNote JSON (what `WikiPage` is today).
  - `kind = ingested_file` → immutable uploaded binary + parsed text chunks (what `ProjectDocument` is today).
  - `kind = template` → reserved for unified template handling.
  - Open for future kinds (`kind = embed`, `kind = external_sync`) without another fork.
- **BREAKING**: Retire the `ProjectDocument` model and `document-store.ts`. All uploaded-file behavior becomes `KnowledgeAsset` with `kind = ingested_file`.
- **BREAKING**: Retire the direct `WikiPage` repository surface for external consumers. `WikiPage` becomes an internal projection of a `KnowledgeAsset` row; services and handlers route through `KnowledgeAssetRepository`.
- **BREAKING**: Rename backend package `src-go/internal/document/` → `src-go/internal/knowledge/ingest/` and fold `wiki_service` responsibilities into `src-go/internal/knowledge/asset_service.go`. Parsers (`docx.go`, `pdf.go`, `pptx.go`, `xlsx.go`) move under `knowledge/ingest/parsers/`.
- **BREAKING**: Rename frontend `docs-store.ts` + `document-store.ts` → single `knowledge-store.ts`. The dashboard route stays at `/docs` for user familiarity, but the store, types, and API client names align with `knowledge-asset`.
- **BREAKING**: Rename WebSocket events `wiki.page.*` → `knowledge.asset.*` (with `kind` discriminator on the payload). Same for comment and backlink events.
- **BREAKING**: Rename REST routes. Old shape (`/api/v1/projects/:pid/wiki/pages`) is removed; new shape is `/api/v1/projects/:pid/knowledge/assets` with `?kind=` filter. Existing `wiki` paths are not kept as aliases — internal-testing stage, contract-first.
- Introduce an **ingest → materialize bridge**: an ingested-file asset can be "opened as wiki" — the system creates a sibling `kind=wiki_page` asset prepopulated with the parsed chunks as editable blocks, while the original file remains pinned as the source-of-record.
- Introduce a **wiki → RAG bridge**: saving a `kind=wiki_page` automatically enqueues content for the (future) knowledge index; the current `knowledge-index` spec's disclaimer about wiki content being out of scope is removed.
- Unify versioning: `PageVersion` becomes `AssetVersion` and applies to any mutable kind (currently `wiki_page` and `template`). Ingested files get a version per reupload.
- Unify comments: comments anchor to `(asset_id, optional anchor_block_id)`. For `ingested_file` assets, only page-level comments are permitted.
- Unify backlinks: `[[id]]` resolves to any `KnowledgeAsset` regardless of kind; `backlink-index` spec extends accordingly.
- Unify RBAC: every asset operation (read/create/update/delete/restore/comment/version) goes through the project RBAC gate (owner/admin/editor/viewer) and applies equally to human and agent-initiated actions.
- Introduce a unified **search API** (`GET /api/v1/projects/:pid/knowledge/search`) that spans all kinds and returns a typed result set. Semantic/embedding search is explicitly out of scope here — this change delivers lexical + metadata filtering and a schema stable enough that a future change can plug embeddings in without breaking callers.

## Capabilities

### New Capabilities

- `knowledge-asset-model`: the unified `KnowledgeAsset` entity, its kinds, lifecycle, soft-delete, ownership, and project-scoped addressability.
- `knowledge-asset-ingestion`: upload → parse → chunk → persist pipeline, kind=ingested_file lifecycle, reupload-as-new-version, and the materialize-as-wiki bridge.
- `knowledge-asset-search`: project-scoped lexical search across kinds with filters (kind, tag, updated range, owner), pagination, and a schema extensible to semantic providers.
- `knowledge-asset-wiki-bridge`: automatic enqueue of wiki page saves into the knowledge corpus, plus the inverse ingested-file → editable-wiki-page materialization.

### Modified Capabilities

- `docs-wiki-workspace`: `WikiSpace` remains, but its page tree is a view over `kind=wiki_page` assets; API routes and events are renamed; the tree still excludes non-wiki kinds.
- `document-versioning`: generalized to `AssetVersion`; applies across editable kinds; adds reupload-versions for `ingested_file`.
- `document-comments`: comments anchor to `(asset_id, optional anchor_block_id)`; ingested-file assets reject anchor_block_id.
- `document-templates`: templates move to `kind=template` within the unified model; the "is_template" flag on WikiPage is deprecated in favor of the kind discriminator.
- `backlink-index`: `[[id]]` resolves across all kinds; panel displays kind alongside title.
- `knowledge-index`: removes the "declarative only, no index/embeddings guaranteed" disclaimer; wiki-page saves enqueue into the indexing pipeline.
- `review-doc-writeback`: updated to address assets by id/kind; unchanged behaviorally for `kind=wiki_page`.
- `doc-driven-task-decomposition`: unchanged behaviorally but now formally scoped to `kind=wiki_page` only.
- `task-doc-bidirectional-links`: `target_type=wiki_page` generalizes to `target_type=knowledge_asset` with a kind field.

## Impact

- **Schema**: new `knowledge_assets` table replaces `wiki_pages` and `project_documents`; new `asset_versions` replaces `page_versions`; `asset_comments` replaces `page_comments`; `asset_ingest_chunks` replaces the chunk storage in the ingest subsystem. Migration rewrites existing rows — acceptable given the internal-testing stage.
- **REST API**: `/api/v1/projects/:pid/wiki/pages/**` and `/api/v1/projects/:pid/documents/**` are removed and replaced by `/api/v1/projects/:pid/knowledge/assets/**` and `/api/v1/projects/:pid/knowledge/search`.
- **WebSocket**: `wiki.page.*`, `wiki.comment.*`, `document.*` → `knowledge.asset.*`, `knowledge.comment.*`, `knowledge.ingest.*`.
- **Frontend**: `docs-store.ts` and `document-store.ts` merge into `knowledge-store.ts`; components under `components/docs/` keep their directory but import types from the new store; `components/` callers of `useDocumentStore` are migrated.
- **Go backend**: `internal/service/wiki_service.go`, `internal/document/`, `internal/repository/wiki_*`, `internal/repository/project_document_*` consolidate under `internal/knowledge/`.
- **RBAC**: every handler and service entry point runs through the project-RBAC gate; agent-initiated operations (from workflows, automation rules, review-writeback) continue to carry an agent `roleId` that is mapped to a `projectRole` at the edge, per the established RBAC model.
- **Spec housekeeping**: nine existing specs get deltas (listed above). No specs are deleted in this change; the `knowledge-index` change sheds its disclaimer but keeps its declarative-references requirements intact.
- **Downstream**: `agent-artifact-doc-sync` (the sibling change that embeds live agent-run blocks into docs) depends on this unified asset model being in place.
- **Out of scope**: CRDT/OT real-time co-editing, embeddings/semantic search, external (GitHub/Notion) sync, doc ACL beyond project RBAC, import/export. Each is its own change.
