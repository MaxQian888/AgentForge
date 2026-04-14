## ADDED Requirements

### Requirement: China-platform progress replay SHALL preserve completion-mode preference and downgrade truth
For DingTalk, WeCom, QQ Bot, and QQ long-running actions, the control-plane replay contract SHALL preserve enough provider completion metadata to let the Bridge retry the originally preferred reply or update path after reconnect. If replay can no longer honor that provider-specific path, the Bridge MUST emit the documented fallback and preserve the resulting downgrade reason instead of silently switching to an unrelated send path.

#### Scenario: DingTalk replay preserves session-aware completion mode
- **WHEN** a DingTalk progress or terminal update is replayed after reconnect
- **THEN** the replayed delivery preserves whether the original preferred path was session webhook reply or conversation-scoped reply
- **AND** fallback remains explicit if neither path is still usable

#### Scenario: WeCom replay preserves callback-versus-direct-send preference
- **WHEN** a WeCom progress or terminal update is replayed after reconnect
- **THEN** the replayed delivery preserves whether the original preferred path was `response_url` reply or direct app send
- **AND** the final delivery metadata exposes any fallback taken during replay

#### Scenario: QQ Bot or QQ replay preserves conversation-scoped completion truth
- **WHEN** a QQ Bot or QQ progress or terminal update is replayed after reconnect
- **THEN** the replayed delivery preserves the original conversation-scoped reply target data needed for provider-supported follow-up delivery
- **AND** the Bridge does not re-emit the initial acceptance message as a duplicate fallback
