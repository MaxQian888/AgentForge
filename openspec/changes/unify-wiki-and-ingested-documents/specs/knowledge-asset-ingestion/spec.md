## ADDED Requirements

### Requirement: File upload creates an ingested-file asset

The system SHALL accept file uploads as the creation path for `kind=ingested_file` assets. Supported file types SHALL at minimum include `.docx`, `.xlsx`, `.pptx`, and `.pdf`, matching the parsers already present in the codebase.

#### Scenario: Upload a supported file

- **WHEN** a client sends `POST /api/v1/projects/:pid/knowledge/assets` with a multipart body containing a supported file and metadata `{kind:"ingested_file", title}`
- **THEN** the system stores the file via the object-storage seam, creates a `kind=ingested_file` asset with `ingest_status=pending`, `file_ref`, `file_size`, and `mime_type` populated, and returns the created asset

#### Scenario: Reject unsupported file type

- **WHEN** a client uploads a file whose extension is not in the supported set
- **THEN** the system rejects the request with a validation error and does not create an asset

### Requirement: Asynchronous parse and chunk pipeline

The system SHALL parse uploaded files into text chunks asynchronously after creation. The lifecycle SHALL be `pending → processing → ready | failed`.

#### Scenario: Parse succeeds

- **WHEN** the ingest worker picks up an asset with `ingest_status=pending`
- **THEN** the system transitions status to `processing`, invokes the kind-appropriate parser, persists the chunks in `asset_ingest_chunks`, populates `content_text` with the concatenated plaintext, updates `ingest_chunk_count`, and transitions status to `ready`

#### Scenario: Parse fails

- **WHEN** the parser throws an error
- **THEN** the system sets `ingest_status=failed` and records the error message on the asset; no chunks are persisted; the asset remains listable so the operator can inspect or delete it

#### Scenario: Status change broadcast

- **WHEN** `ingest_status` transitions
- **THEN** the system broadcasts `knowledge.ingest.status_changed` with `asset_id`, `old_status`, `new_status`, and on failure an `error` string

### Requirement: Reupload creates a new version

The system SHALL support reuploading the binary for an existing `ingested_file` asset. Reuploads SHALL create a new `AssetVersion` snapshotting the prior file state; the asset's `file_ref`, `file_size`, `mime_type`, and chunks SHALL be replaced with the new upload's outputs.

#### Scenario: Reupload succeeds

- **WHEN** a client sends `POST /api/v1/projects/:pid/knowledge/assets/:id/reupload` with a multipart body for an existing `ingested_file` asset
- **THEN** the system snapshots the previous state into `AssetVersion`, replaces the binary and parses it fresh, and transitions the asset through `pending → processing → ready`

#### Scenario: Reupload rejected for non-file kinds

- **WHEN** the reupload endpoint is invoked against a `wiki_page` or `template` asset
- **THEN** the system rejects the request with a validation error

### Requirement: File storage abstraction seam

The system SHALL store uploaded binaries through a `BlobStorage` service interface that returns a `file_ref` URI. The default implementation MAY be local filesystem for internal testing, provided the interface allows an S3/MinIO backing to drop in without touching callers.

#### Scenario: Asset references storage URI, not raw path

- **WHEN** an ingested-file asset is created
- **THEN** the asset's `file_ref` is a URI resolvable by the `BlobStorage` interface, not a local filesystem path embedded in the asset

### Requirement: Soft-delete removes binary and chunks

The system SHALL remove the stored binary and chunk rows when an `ingested_file` asset's soft-delete is finalized (hard-delete). Between soft-delete and hard-delete, the binary and chunks SHALL remain accessible for restore.

#### Scenario: Soft-delete preserves binary

- **WHEN** an ingested-file asset is soft-deleted
- **THEN** the asset row is marked `deleted_at=now` but `file_ref` remains resolvable and `asset_ingest_chunks` rows are preserved

#### Scenario: Hard-delete purges binary

- **WHEN** a soft-deleted ingested-file asset's retention window elapses or it is hard-deleted explicitly
- **THEN** the system removes the binary via `BlobStorage.Delete(file_ref)` and deletes `asset_ingest_chunks` rows
