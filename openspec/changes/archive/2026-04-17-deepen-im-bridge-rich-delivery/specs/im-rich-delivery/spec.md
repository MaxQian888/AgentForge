## ADDED Requirements

### Requirement: Canonical delivery envelope SHALL carry attachments

`DeliveryEnvelope` SHALL include an `Attachments []Attachment` field so outbound deliveries can carry file payloads alongside or instead of text, structured, or native content. The delivery ladder MUST evaluate attachments ahead of the text/structured/native decision so attachment-capable providers upload first and attachment-unsupported providers degrade to a text summary with `fallback_reason=attachments_unsupported`.

#### Scenario: Attachments travel through control-plane replay
- **WHEN** the backend queues a delivery with `attachments[]` for later replay
- **THEN** the replayed envelope retains each attachment's `staged_id`, `kind`, and `filename`
- **AND** the Bridge resolves the staged ids to local paths at delivery time

#### Scenario: Delivery receipt reports attachment counts
- **WHEN** a delivery with attachments completes
- **THEN** `DeliveryReceipt.AttachmentsSent` equals the number of successful uploads
- **AND** `DeliveryReceipt.AttachmentsBytes` equals the sum of `SizeBytes` across successful attachments
