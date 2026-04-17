## ADDED Requirements

### Requirement: Provider capability matrix SHALL declare attachment, reaction, and thread capabilities

`PlatformCapabilities` SHALL expose `SupportsAttachments`, `MaxAttachmentSize`, `AllowedAttachmentKinds`, `SupportsReactions`, `ReactionEmojiSet`, `SupportsThreads`, `ThreadPolicySupport`, and `MutableUpdateMethod`. Every provider's `MetadataForPlatform` MUST populate these fields truthfully — missing support MUST be `false` / zero-value, not silently inherited from richer providers. `/im/health` MUST surface the new fields under `capability_matrix` so operators and the backend provider catalog can observe truth.

#### Scenario: Supported provider advertises truthful matrix
- **WHEN** Bridge is running Slack in live mode
- **THEN** `/im/health` reports `supportsAttachments=true`, `supportsReactions=true`, `supportsThreads=true` with non-empty `allowedAttachmentKinds`, `reactionEmojiSet`, `threadPolicySupport`

#### Scenario: Unsupported provider advertises zero values
- **WHEN** Bridge is running WeCom in live mode
- **THEN** `/im/health` reports `supportsAttachments=false`, `supportsReactions=false`, `supportsThreads=false`
- **AND** `mutableUpdateMethod=template_card_update` so operators know how mutable updates are implemented
