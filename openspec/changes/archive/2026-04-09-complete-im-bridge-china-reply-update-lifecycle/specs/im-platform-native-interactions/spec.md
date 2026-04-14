## ADDED Requirements

### Requirement: China-platform asynchronous completions SHALL prefer provider-owned reply or update paths before fallback
When a DingTalk, WeCom, QQ Bot, or QQ interaction or command triggers long-running work, the Bridge SHALL evaluate the preserved reply target against the active provider's native completion semantics before emitting a new plain-text message. The Bridge MUST prefer the provider-owned path that the current platform can actually honor, such as DingTalk session webhook reply, WeCom `response_url` reply, QQ Bot `msg_id`-aware markdown or text follow-up, or QQ reply-segment-aware text reply. Only when the preserved reply target is unusable or the provider does not support the requested richer behavior MAY the Bridge fall back to a new text send.

#### Scenario: DingTalk completion prefers session webhook reply
- **WHEN** a DingTalk card action or command preserves a session webhook reply target
- **THEN** asynchronous progress or terminal completion uses that session webhook path first
- **AND** the Bridge falls back to conversation-scoped or plain-text send only when the session webhook path is unavailable

#### Scenario: WeCom completion prefers callback reply context
- **WHEN** a WeCom callback-triggered action preserves a `response_url`
- **THEN** asynchronous progress or terminal completion uses the callback reply path before direct app-message send
- **AND** any fallback to direct send remains explicit in delivery metadata

#### Scenario: QQ Bot completion prefers msg-id-aware reply context
- **WHEN** a QQ Bot interaction or inbound message preserves a `msg_id` and conversation target
- **THEN** asynchronous completion uses the provider-supported reply path tied to that `msg_id` before generic follow-up send
- **AND** incompatible mutable-update requests degrade explicitly to supported markdown or text output

#### Scenario: QQ completion stays text-first
- **WHEN** a QQ command preserves conversation and message identity for later completion
- **THEN** asynchronous completion reuses that context through reply-segment-aware text delivery when possible
- **AND** the Bridge does not attempt a native payload or mutable update path that QQ does not advertise

### Requirement: China-platform downgrade metadata SHALL identify unusable completion context
When a Chinese-platform completion cannot honor the preferred provider-native reply or update path, the Bridge SHALL record a stable downgrade category that identifies why the provider-native path could not be used. The resulting metadata MUST survive direct delivery, bound progress delivery, action completion, and replay recovery.

#### Scenario: Replayed completion reports missing provider reply context
- **WHEN** a replayed DingTalk, WeCom, QQ Bot, or QQ completion no longer has a usable provider-native reply target
- **THEN** the Bridge emits a supported fallback delivery
- **AND** the resulting metadata identifies that the original provider-native completion context was unavailable during replay
