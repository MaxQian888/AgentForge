## ADDED Requirements

### Requirement: Chinese-platform registration and health SHALL expose async completion truth
The active IM platform runtime SHALL expose enough metadata for operators and downstream routing to distinguish whether DingTalk, WeCom, QQ Bot, or QQ can complete long-running work through provider-native reply or update paths, reply-only paths, or text-first fallback. Health, registration, and capability matrix payloads MUST keep this async completion truth aligned with the provider's actual reply target restoration and replay behavior instead of collapsing those platforms into one generic `native_send_with_fallback` label.

#### Scenario: DingTalk publishes session-webhook completion truth
- **WHEN** the active platform is DingTalk
- **THEN** health and registration metadata expose that asynchronous completion prefers session webhook or conversation-scoped reply
- **AND** the capability matrix does not imply delayed mutable-card update parity with Feishu

#### Scenario: WeCom publishes reply-first completion truth
- **WHEN** the active platform is WeCom
- **THEN** health and registration metadata expose that asynchronous completion prefers preserved `response_url` and may fall back to direct app send
- **AND** the capability matrix makes that fallback boundary visible to operators

#### Scenario: QQ Bot publishes msg-id-aware completion truth
- **WHEN** the active platform is QQ Bot
- **THEN** health and registration metadata expose that asynchronous completion may reuse preserved `msg_id` or conversation context for markdown or text follow-up
- **AND** the capability matrix does not claim mutable native update support unless the adapter truly implements it

#### Scenario: QQ remains text-first in completion metadata
- **WHEN** the active platform is QQ
- **THEN** health and registration metadata continue to report QQ as `text_first`
- **AND** the capability matrix describes reply reuse and text fallback without advertising richer update surfaces
