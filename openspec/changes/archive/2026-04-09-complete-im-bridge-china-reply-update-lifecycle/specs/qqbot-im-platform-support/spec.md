## ADDED Requirements

### Requirement: QQ Bot asynchronous completion SHALL prefer msg-id-aware reply before generic follow-up
When a QQ Bot inbound message or interaction starts long-running work, asynchronous progress and terminal completion SHALL first use preserved `msg_id` and conversation context for the provider-supported reply path. If the requested richer behavior cannot be honored in that context, the Bridge SHALL fall back to supported markdown or text follow-up and preserve explicit downgrade metadata.

#### Scenario: QQ Bot completion uses preserved msg_id reply context
- **WHEN** a QQ Bot-originated long-running action finishes while preserved `msg_id` and conversation context are still usable
- **THEN** the Bridge delivers the completion through the provider-supported reply path tied to that context
- **AND** the completion remains visible in the same user-facing conversation

#### Scenario: QQ Bot mutable-update request degrades explicitly
- **WHEN** a QQ Bot progress or terminal update requests mutable richer behavior that the preserved reply context cannot honor
- **THEN** the Bridge falls back to supported markdown or text follow-up delivery
- **AND** the resulting metadata records that the original mutable-update plan was unavailable
