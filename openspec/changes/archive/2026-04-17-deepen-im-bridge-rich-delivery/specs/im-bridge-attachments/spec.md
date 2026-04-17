## ADDED Requirements

### Requirement: Bridge SHALL expose attachments as a first-class delivery primitive

IM Bridge SHALL model attachments (files, images, logs, patches, reports) as a first-class payload on outbound envelopes and on inbound messages. `core.Attachment` MUST carry `ID`, `Kind`, `MimeType`, `Filename`, `SizeBytes`, `ContentRef`, `ExternalRef`, and `Metadata`. `DeliveryEnvelope.Attachments` and `Message.Attachments` MUST be wired through the delivery ladder so attachments can be uploaded and referenced without collapsing them to text.

#### Scenario: Outbound envelope with attachments uploads through provider
- **WHEN** the backend posts `/im/send` with a non-empty `attachments` array and the active provider implements `AttachmentSender`
- **THEN** Bridge resolves each attachment to a local `ContentRef`, calls `UploadAttachment` followed by `SendAttachment` or `ReplyAttachment`
- **AND** the delivery receipt reports `attachments_sent` and `attachments_bytes`

#### Scenario: Inbound message exposes attachment metadata
- **WHEN** a platform adapter receives a file-bearing inbound message
- **THEN** the adapter populates `Message.Attachments` with `Kind` + `ExternalRef` (and `Filename` when known)
- **AND** the engine dispatches the message without losing attachment context

### Requirement: Attachment ingress SHALL stage files through a managed directory

IM Bridge SHALL stage inbound attachment bytes under `${IM_BRIDGE_ATTACHMENT_DIR}` (defaulting to `${IM_BRIDGE_STATE_DIR}/attachments`). `POST /im/attachments` MUST accept either a `multipart/form-data` upload or a raw body plus `X-Attachment-*` headers, write the bytes to a per-attachment file, and return a `staged_id` that callers MUST reference in subsequent `/im/send` payloads.

#### Scenario: Multipart upload returns a staged id
- **WHEN** an operator posts a multipart form with a `file` part to `/im/attachments`
- **THEN** Bridge writes the file bytes to `${IM_BRIDGE_ATTACHMENT_DIR}/<uuid>-<sanitized-name>` with mode `0o600`
- **AND** the response JSON includes `id`, `path`, `size_bytes`, and `kind`

#### Scenario: Base64 payload on /im/send stages automatically
- **WHEN** `/im/send` is posted with `attachments[].data_base64`
- **THEN** Bridge decodes and stages the payload before resolving it to an `Attachment.ContentRef`
- **AND** the staged file is cleaned up by TTL or capacity GC when its task finishes

### Requirement: Staging directory SHALL enforce TTL and capacity limits

The staging directory SHALL be cleaned at startup, TTL-evicted (default 1 hour) by a background worker, and capacity-limited (default 2 GB total) with oldest-first GC. Operators MUST be able to override TTL and capacity via `IM_BRIDGE_ATTACHMENT_TTL` / `IM_BRIDGE_ATTACHMENT_CAPACITY_BYTES`.

#### Scenario: Startup sweeps residual files
- **WHEN** Bridge starts with files already present in the staging directory
- **THEN** every residual file is removed before the receiver accepts traffic

#### Scenario: Capacity threshold triggers oldest-first eviction
- **WHEN** staging a new file pushes total size above the configured capacity
- **THEN** Bridge deletes the oldest staged files until total size falls under the threshold

### Requirement: Attachment delivery SHALL degrade with a fallback reason when unsupported

When the active provider lacks attachment support (either `SupportsAttachments=false` or no `AttachmentSender` implementation) or when an attachment violates the provider's size/kind limits, Bridge SHALL degrade to a text summary that lists filenames and URLs (when public), and MUST record `fallback_reason=attachments_unsupported` or `attachment_size_exceeded`/`attachment_kind_rejected` in the delivery metadata.

#### Scenario: Unsupported provider downgrades to a text summary
- **WHEN** `/im/send` targets a provider with `SupportsAttachments=false` and carries two attachments with URLs
- **THEN** Bridge emits a text message that includes the filenames and URLs
- **AND** `fallback_reason` on the receipt is `attachments_unsupported`

#### Scenario: Oversized attachment is skipped without failing the delivery
- **WHEN** an attachment exceeds the provider's `MaxAttachmentSize`
- **THEN** Bridge skips that attachment, records `attachment_size_exceeded`, and continues the text + remaining-attachments path
