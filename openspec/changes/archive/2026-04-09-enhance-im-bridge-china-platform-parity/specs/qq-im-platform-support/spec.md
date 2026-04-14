## MODIFIED Requirements

### Requirement: QQ outbound delivery SHALL support explicit structured downgrade semantics
The system SHALL resolve QQ-targeted typed deliveries through a text-first QQ rendering profile. QQ MUST not advertise provider-native payload surfaces or mutable-update lifecycle it does not support. When the requested richer path cannot be honored, the Bridge SHALL convert the delivery into QQ-supported reply-segment-aware text or link output for the originating conversation and preserve explicit fallback metadata instead of pretending richer delivery succeeded.

#### Scenario: Structured QQ notification becomes text-first output
- **WHEN** the notification receiver handles a QQ-targeted delivery with structured or richer content
- **THEN** the Bridge resolves that delivery into QQ-supported text or link output for the active conversation
- **AND** the delivery metadata records that QQ remained on its text-first path

#### Scenario: Native or mutable QQ request degrades explicitly
- **WHEN** a QQ-targeted typed delivery requests native payload or mutable update behavior
- **THEN** the Bridge falls back to supported QQ text delivery
- **AND** operators can see from the delivery metadata that the original richer request was unsupported for QQ
