## MODIFIED Requirements

### Requirement: Feishu long-running card actions SHALL coordinate immediate acknowledgement and delayed update
When a Feishu card action triggers work that cannot complete within the synchronous callback window, the Bridge SHALL acknowledge the callback within the provider's immediate response deadline and SHALL only attempt delayed card mutation after that acknowledgement succeeds. When delayed update is used, the preserved token MUST be treated as provider-scoped context with the documented 30-minute validity window and no more than two updates per token. If the token is missing, expired, exhausted, or incompatible with the requested update mode, the Bridge MUST fall back explicitly to a supported reply or send path and record the provider-aware fallback reason.

#### Scenario: Long-running card action acknowledges first and updates later
- **WHEN** a Feishu card action starts work that outlives the immediate callback window
- **THEN** the Bridge returns the callback acknowledgement within 3 seconds
- **AND** later uses the preserved delayed-update token to mutate the originating card instead of sending an unrelated duplicate message

#### Scenario: Expired or exhausted delayed-update token falls back explicitly
- **WHEN** a Feishu completion update is ready but the preserved delayed-update token is expired, exhausted, or otherwise unusable
- **THEN** the Bridge falls back to a supported Feishu reply or send path
- **AND** the delivery outcome records that native card mutation was skipped because the delayed-update context was no longer valid

#### Scenario: Delayed update is never attempted before callback acknowledgement
- **WHEN** a Feishu action completion path requires delayed update
- **THEN** the Bridge waits until the synchronous callback acknowledgement has succeeded before calling the delayed-update path
- **AND** it does not perform a parallel mutation attempt that could be reverted or rejected by the provider
