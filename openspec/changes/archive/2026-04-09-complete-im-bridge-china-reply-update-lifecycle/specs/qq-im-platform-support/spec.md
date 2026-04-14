## ADDED Requirements

### Requirement: QQ asynchronous completion SHALL remain text-first and conversation-scoped
When a QQ command or action starts long-running work, asynchronous progress and terminal completion SHALL reuse preserved group or direct-message context through supported text delivery before considering a new unrelated send path. QQ MUST remain text-first: the Bridge SHALL not advertise or attempt provider-native payload or mutable-update semantics that QQ does not actually implement.

#### Scenario: QQ terminal completion reuses reply-aware text delivery
- **WHEN** a QQ-originated long-running action finishes and the preserved conversation or message context is still available
- **THEN** the Bridge delivers the terminal completion back into that same QQ conversation through supported text delivery
- **AND** users do not receive an invented richer payload type

#### Scenario: QQ replay falls back explicitly when reply context is stale
- **WHEN** a replayed QQ completion no longer has a usable reply-aware context
- **THEN** the Bridge emits the documented text fallback
- **AND** delivery metadata records that the original QQ completion context was unusable
