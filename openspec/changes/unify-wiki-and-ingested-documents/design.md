## Context

AgentForge has grown two document concepts in parallel, each with its own store, repository, service, spec, and terminology:

| Concern | Wiki path | Ingest path |
|---|---|---|
| FE store | `lib/stores/docs-store.ts` (`DocsPage`) | `lib/stores/document-store.ts` (`ProjectDocument`) |
| BE service | `src-go/internal/service/wiki_service.go` | `src-go/internal/document/` (parsers only; no full service) |
| Model | `WikiSpace`, `WikiPage`, `PageVersion`, `PageComment` | `ProjectDocument`, `TextChunk` |
| REST root | `/api/v1/projects/:pid/wiki/pages/**` | `/api/v1/projects/:pid/documents/**` |
| WebSocket | `wiki.page.*`, `wiki.comment.*` | none formalized |
| Specs | `docs-wiki-workspace`, `block-document-editor`, `document-versioning`, `document-comments`, `document-templates`, `backlink-index`, `task-doc-bidirectional-links`, `doc-driven-task-decomposition`, `review-doc-writeback` | `knowledge-index` (declarative only) |
| Search | none | none |
| RBAC | project membership implicit; no role gate | none |

Users and downstream specs are starting to want cross-fork behavior — a PRD uploaded as `.docx` should be findable next to a PRD written inline; `[[id]]` should resolve to both; AI assist, sharing, and external sync will need one graph. Every existing spec carefully says "wiki page" when it really means "document" — adding a second storage shape on top would triple the surface.

Two constraints that shape this design:

1. **Internal-testing stage, contract-first**: per the `API stability stage` memory, breaking changes are freely permitted; "contract first" means designing the final-state contract, not adding back-compat shims.
2. **Project RBAC applies uniformly**: per the `Project RBAC scope` memory, owner/admin/editor/viewer gates both human and agent-initiated actions, and a human's `projectRole` is distinct from an agent's `roleId`. Any new asset operation must run through the same gate.

## Goals / Non-Goals

**Goals**

- One first-class entity — `KnowledgeAsset` — with a `kind` discriminator that cleanly models the editable-wiki case and the immutable-file case without subclassing via parallel tables.
- One repository, one service, one store, one WebSocket namespace, one REST root, one search endpoint, one backlink graph, one RBAC gate.
- Behavior-preserving for existing wiki requirements: all nine existing doc-area specs keep their scenarios (renamed to the asset model) unless called out as changed in `proposal.md`.
- Foundations for the sibling change `agent-artifact-doc-sync`: the unified model must be extensible with kinds like `agent_run_block` or `cost_block` in later changes without another fork.
- Foundations for later search/embeddings/sync changes: the search API's shape must be stable enough that adding a semantic provider later does not break callers.

**Non-Goals**

- CRDT/OT real-time co-editing (separate change `realtime-collaborative-editing`).
- Semantic/embedding search; this change delivers lexical + metadata search only and the schema to grow.
- External sync with GitHub/Notion (separate `external-doc-sync`).
- Multi-space-per-project (the `docs-wiki-workspace` one-space constraint remains; `multi-wiki-space-per-project` is a later change).
- Doc-level ACL beyond project RBAC (separate `document-permissions-acl`).
- Import from Notion/Confluence/Markdown trees; only the existing upload parsers are reused.
- Analytics, audit log, DLP, watermark.
- Running the indexing pipeline end-to-end. This change emits the enqueue event and defines the index contract; the actual index backend lives behind a named service interface and is implemented in a follow-up.

## Decisions

### D1: Single table with `kind` discriminator (STI) — not one table per kind

**Chosen**: One `knowledge_assets` table with a non-null `kind` column plus `content_json` (BlockNote for editable kinds, null for files), `file_ref` (object-storage URI for file kinds, null otherwise), and shared columns (title, parent_id, sort_order, path, owner, timestamps, soft-delete).

**Alternatives considered**:
- **CTI (Class-Table Inheritance)**: parent `knowledge_assets` row plus a child table per kind (`wiki_page_assets`, `ingested_file_assets`). Rejected: extra join on every read, harder to evolve a new kind (every lookup site learns the new table). STI is cheaper in our Go + GORM layer and Postgres handles kind-specific nullable columns well.
- **Polymorphic association via `entity_link`-style records**: rejected as it duplicates the existing `entity_link` machinery and hides kind semantics behind a generic mapping.

**Rationale**: assets share more than they differ (title, owner, path, versions, comments, backlinks, soft-delete, RBAC). The differences are two or three nullable columns, not two schemas.

### D2: Name the abstraction `KnowledgeAsset`, not `Document`

**Chosen**: Go type `KnowledgeAsset`, table `knowledge_assets`, FE store `knowledge-store.ts`, REST root `/knowledge/assets`.

**Alternatives considered**:
- `Document` (flat): rejected because both existing stores already claim the word and the rename would collide continuously during migration.
- `Artifact`: rejected — already used informally for agent outputs (and reserved for the sibling change `agent-artifact-doc-sync`, where an asset of kind `wiki_page` can contain blocks referencing "agent artifacts").
- `Page`: rejected — overloaded with Next.js page terminology, and files are not pages.

**Rationale**: "Knowledge asset" is neutral across kinds and matches the existing `knowledge-index` spec naming. The dashboard route stays `/docs` for user familiarity, but the code layer speaks asset/kind.

### D3: Keep `WikiSpace` as-is; scope multi-space out

**Chosen**: Every project keeps exactly one `wiki_space`. Every `kind=wiki_page` or `kind=template` asset belongs to a `wiki_space`. Every `kind=ingested_file` asset belongs directly to the project (no space membership required) — they appear in a sibling "Uploads" pane, not inside the wiki tree.

**Alternative**: generalize `wiki_space` to `knowledge_space` now. Rejected: compounds the scope and the `multi-wiki-space-per-project` change is already slated as a separate later change. Changing the outer container while also changing the inner abstraction inflates risk.

### D4: Content representation per kind

```
knowledge_assets
├─ id                 UUID
├─ project_id         UUID  (always set)
├─ wiki_space_id      UUID? (set for kind in {wiki_page, template})
├─ parent_id          UUID? (tree, only meaningful for wiki_page)
├─ kind               ENUM (wiki_page, ingested_file, template)
├─ title              TEXT
├─ path               TEXT  (materialized path, tree kinds only)
├─ sort_order         INT
├─ content_json       JSONB? (BlockNote blocks; set for wiki_page, template)
├─ content_text       TEXT?  (derived plaintext for search/indexing)
├─ file_ref           TEXT?  (object-store URI; set for ingested_file)
├─ file_size          BIGINT?
├─ mime_type          TEXT?
├─ ingest_status      ENUM?  (pending|processing|ready|failed; set for ingested_file)
├─ ingest_chunk_count INT?
├─ template_category  TEXT?  (prd|rfc|adr|...; set for template)
├─ is_system_template BOOL
├─ is_pinned          BOOL
├─ owner_id           UUID?
├─ created_by         UUID?
├─ updated_by         UUID?
├─ created_at / updated_at / deleted_at
└─ version            BIGINT  (optimistic lock)
```

Invariants enforced at the repository layer (not just schema):
- `kind=wiki_page` → `wiki_space_id` NOT NULL, `content_json` NOT NULL, `file_ref` NULL.
- `kind=ingested_file` → `file_ref` NOT NULL, `parent_id` NULL, `wiki_space_id` NULL, `content_json` NULL, `path` NULL.
- `kind=template` → `wiki_space_id` NOT NULL, `content_json` NOT NULL, `parent_id` NULL, `is_system_template` may be true, `template_category` NOT NULL.

### D5: Versioning generalizes to `asset_versions`

- For `wiki_page` and `template`: named snapshots at user-triggered "Save Version" exactly as today. Restore replaces `content_json`.
- For `ingested_file`: re-uploading a file with the same asset id creates a new version — `file_ref`, `file_size`, `mime_type`, `content_text`, `ingest_chunk_count` all snapshot per version. No "restore" for files; instead a version can be marked active.
- `asset_versions.kind_snapshot` preserves the kind at snapshot time so future kind changes don't break old restores.

### D6: Comments anchor to `(asset_id, anchor_block_id?)`

- `anchor_block_id` is optional; page-level comments set it null.
- For `kind=ingested_file`, the repository rejects non-null `anchor_block_id` — files don't have blocks.
- Existing `document-comments` scenarios translate 1:1 with "wiki page" replaced by "asset" in the API surface.

### D7: Backlink extraction runs per kind

- `backlink-index` extractor runs on `content_text` for `wiki_page` saves and on task descriptions, unchanged in spirit.
- For `ingested_file`, extraction is best-effort on the parsed `content_text` — backlinks are one-way (the file mentions assets, but we don't edit the file to remove stale mentions; stale links are garbage-collected on re-ingest).
- The backlinks panel displays `kind` alongside title so a task shows "requirements.pdf (file)" vs "PRD (wiki)" explicitly.

### D8: Search — Postgres FTS now, provider-pluggable later

- New endpoint `GET /api/v1/projects/:pid/knowledge/search?q=&kind=&tag=&updated_after=&cursor=` returns `{items: [...], nextCursor, total?}`.
- Backing store: Postgres `tsvector` column maintained by trigger on `content_text` + `title`. Adequate for tens of thousands of assets per project.
- The service layer uses a `SearchProvider` interface with a single Postgres implementation today. A later change can add a `BleveProvider` or an embedding provider without breaking the REST contract.
- Result items carry `{id, kind, title, snippet, updated_at, score, match_locations}`. `score` and `snippet` are provider-specific; the interface guarantees they are present but not stable.

### D9: Materialize-as-wiki bridge (ingested → editable)

- Action: `POST /api/v1/projects/:pid/knowledge/assets/:id/materialize-as-wiki` on an `ingested_file` asset.
- Behavior: creates a sibling `kind=wiki_page` asset under a chosen parent, with `content_json` built from the parsed chunks (one paragraph block per chunk, preceded by a `callout` block that says "Imported from {filename} on {date}" and contains a link back to the source asset via `[[asset-id]]`).
- The two assets are decoupled after creation. Re-ingesting the source does **not** re-sync the wiki page.
- The source file asset gets an entity link `source -> materialization` with a new link type `materialized_from` so the UI can show the relationship.

### D10: Wiki → knowledge index enqueue

- On every successful save of a `wiki_page` or `template` asset, the service layer emits an eventbus event `knowledge.asset.content_changed` carrying `{asset_id, kind, project_id, version, content_text_length}`.
- A stub subscriber in `internal/knowledge/index_enqueuer.go` forwards the event to an `IndexPipeline` interface.
- This change ships a `NoopIndexPipeline` (logs + metric only). The actual pipeline implementation is a follow-up change; this guarantees the enqueue contract is stable and downstream work can plug in without re-touching the asset service.
- The `knowledge-index` spec drops the "no indexing guaranteed" disclaimer and adds a requirement that wiki content is enqueued for indexing.

### D11: No REST alias — single cutover rename

Per the internal-testing/contract-first stage, the old routes are removed outright:

| Old | New |
|---|---|
| `GET /api/v1/projects/:pid/wiki/pages` | `GET /api/v1/projects/:pid/knowledge/assets?kind=wiki_page` |
| `POST /api/v1/projects/:pid/wiki/pages` | `POST /api/v1/projects/:pid/knowledge/assets` with `{kind:"wiki_page", ...}` |
| `GET /api/v1/projects/:pid/wiki/pages/:id` | `GET /api/v1/projects/:pid/knowledge/assets/:id` |
| `POST /api/v1/projects/:pid/wiki/pages/:id/versions` | `POST /api/v1/projects/:pid/knowledge/assets/:id/versions` |
| `POST /api/v1/projects/:pid/wiki/pages/:id/decompose-tasks` | `POST /api/v1/projects/:pid/knowledge/assets/:id/decompose-tasks` |
| `POST /api/v1/projects/:pid/documents` (upload) | `POST /api/v1/projects/:pid/knowledge/assets` with multipart + `kind=ingested_file` |

Routes that are still tree-shaped (list tree, move) keep their semantics but move under `/knowledge/assets/tree` with `?kind=wiki_page` implicit.

### D12: RBAC enforced in the service layer, not the handler

Per the `Project RBAC scope` memory, the same gate must apply to human and agent-initiated actions. Agent calls reach the service layer via workflow invocations and internal bridges, bypassing HTTP handlers. Therefore the RBAC check lives in `KnowledgeAssetService`, not in HTTP middleware:

- `Create/Update/Delete` require `editor` or higher.
- `Read/List/Search` require `viewer` or higher.
- `Restore/MoveTree/DeleteSystemTemplate` require `admin` or higher.
- `Publish/Share` — deferred; not in this change.

Service methods take a `PrincipalContext` that carries the project role already resolved at the edge. Handlers resolve `PrincipalContext` from the HTTP session; the workflow engine resolves it from the agent's `roleId → projectRole` mapping. Both paths funnel through the same service.

### D13: Frontend store unification strategy

- New `lib/stores/knowledge-store.ts` exports `useKnowledgeStore()` with a single `KnowledgeAsset` type discriminated by `kind`.
- Existing callers of `useDocsStore` (wiki) and `useDocumentStore` (RAG uploads) are rewritten to `useKnowledgeStore` with `kind` filters.
- Selectors like `selectWikiPageTree`, `selectIngestedFiles`, `selectTemplatesByCategory` provide kind-scoped views without exposing `kind` to every call site.
- Old store files are deleted, not kept as re-export shims — internal-testing stage allows it.

### D14: WebSocket event taxonomy

| Old | New |
|---|---|
| `wiki.page.created / updated / moved / deleted` | `knowledge.asset.created / updated / moved / deleted` with `kind` field |
| `wiki.comment.created / resolved / deleted` | `knowledge.comment.created / resolved / deleted` |
| (none for ingest) | `knowledge.ingest.status_changed` (`pending → processing → ready|failed`) |
| (none) | `knowledge.asset.content_changed` (saved, triggers index enqueue) |

Every event payload carries `{project_id, asset_id, kind, version}` at minimum.

## Risks / Trade-offs

- **Large blast radius**: every file that touches `docs-store`, `document-store`, `wiki_service`, or `/wiki/pages` routes must change in lockstep. → Mitigate by a migration task sequence (see `tasks.md`) that keeps the repo compiling after each step (rename, adapt, delete).
- **STI nullable churn**: a `wiki_page` asset has null `file_ref`, `file_size`, `ingest_*` columns, and vice versa. → Mitigate via repository-layer invariants tested with explicit cases, and by a thin `kind`-specific projection type exposed to handlers (`WikiPageView`, `IngestedFileView`) so call sites don't handle nulls manually.
- **Search contract stability**: consumers may hardcode `score` behavior that's Postgres-FTS-specific. → Mitigate by documenting `score` as provider-specific in the spec and keeping `snippet` + `match_locations` opaque strings rather than structured ranges.
- **Migration on existing dev data**: anyone with populated dev DBs loses rows unless the migration script runs. → Provide a one-off migration that maps `wiki_pages → knowledge_assets{kind=wiki_page}` and `project_documents → knowledge_assets{kind=ingested_file}`; expect most dev envs to re-seed.
- **"Materialize as wiki" drift**: the wiki copy can drift from the source file when the source is re-uploaded. → Explicitly decoupled by design (D9). UI makes this visible via the `materialized_from` link and a "source updated" hint when the source re-ingests.
- **Indexing contract shipped without an indexer**: the `NoopIndexPipeline` emits events nobody consumes until the follow-up change lands. → Accept. Keeps this change focused; next change wires a real pipeline against the already-stable contract.
- **WebSocket rename visibility**: any external consumer (IM bridge, third-party) listening for `wiki.page.*` will stop receiving events. → Accept per internal-testing stage; note it in the change summary for IM-bridge owners to adjust.

## Migration Plan

1. **Schema migration** (up-only):
   - Create `knowledge_assets`, `asset_versions`, `asset_comments`, `asset_ingest_chunks`, and the `tsvector` column + trigger.
   - Copy `wiki_pages → knowledge_assets{kind=wiki_page}` with parent/path/sort_order preserved and `content_json = content`.
   - Copy `wiki_pages{is_template=true} → knowledge_assets{kind=template}` with `template_category` populated and `wiki_space_id` kept.
   - Copy `project_documents → knowledge_assets{kind=ingested_file}` with `file_ref`, `file_size`, `ingest_status`, `ingest_chunk_count`. Chunk records move to `asset_ingest_chunks` with `asset_id` linking.
   - Copy `page_versions → asset_versions` with `kind_snapshot`.
   - Copy `page_comments → asset_comments`.
   - Drop the old tables at the end of the migration. (Internal-testing stage: single-cutover.)
2. **Backend refactor**: repackage `internal/wiki*` and `internal/document/*` into `internal/knowledge/`; handler routes renamed in one pass; WS event names renamed; eventbus types renamed.
3. **Frontend refactor**: introduce `knowledge-store`; migrate all `useDocsStore` / `useDocumentStore` call sites; delete old stores; update component imports under `components/docs/`.
4. **Spec housekeeping**: update the nine affected specs per the delta files in `specs/`.
5. **Smoke tests**: wiki create/read/update/move/delete; ingest upload → parsed → ready; materialize-as-wiki; search across both kinds; backlink from a task description to an ingested file; version snapshot/restore on wiki.

Rollback strategy: given single-cutover and no dual-run, rollback is restoring from DB backup + reverting the commit. Acceptable at internal-testing stage.

## Open Questions

- Should `kind=template` live in `knowledge_assets` or in its own `asset_templates` table? The `docs-store.ts` currently reuses `DocsPage` for templates, which argues for keeping them in the same table. If it gets in the way, splitting templates out later is an additive change — defer unless a blocker appears during implementation.
- Where does `ingested_file` blob content actually live? Current codebase lacks explicit object storage abstraction; `file_ref` should reference a path that an `internal/storage` seam resolves. If a local-filesystem default is enough for internal testing, fine; but the seam must be defined now so S3/MinIO backing can drop in later.
- Should the search index trigger fire on every save, or batched? Single-save trigger is simpler; batching is an optimization deferrable until the real indexer lands. Default: per-save with an eventbus event, let the downstream consumer batch.
- Does the `decompose-tasks` endpoint now accept `kind=template`? Probably no — decomposition should only apply to authored `wiki_page`. Add an explicit reject scenario in the delta spec.
