## ADDED Requirements

### Requirement: WeCom asynchronous completion SHALL prefer callback reply and explicit direct-send fallback
When a WeCom message or callback starts long-running work, asynchronous progress and terminal completion SHALL first use the preserved WeCom callback reply context when it is still valid. If callback reply is unavailable, the Bridge SHALL fall back to the documented direct app-message send path and preserve metadata that the completion left the original reply context.

#### Scenario: WeCom terminal completion uses preserved response_url
- **WHEN** a WeCom callback-triggered action finishes while the preserved `response_url` is still valid
- **THEN** the Bridge delivers the terminal completion through that callback reply path
- **AND** the completion remains tied to the original WeCom conversation context

#### Scenario: WeCom progress falls back to direct send when callback context is unavailable
- **WHEN** a queued or replayed WeCom progress update no longer has a usable callback reply context
- **THEN** the Bridge falls back to direct app-message send using the preserved chat or user target
- **AND** delivery metadata records that callback reply context was unavailable
