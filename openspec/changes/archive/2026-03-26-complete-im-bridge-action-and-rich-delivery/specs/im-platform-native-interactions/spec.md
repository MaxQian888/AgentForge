## ADDED Requirements

### Requirement: Normalized interactive callbacks SHALL produce truthful backend outcomes
After a platform-native interactive callback is normalized into the shared action envelope, the system SHALL execute the corresponding backend action workflow or return an explicit terminal failure. It MUST preserve the originating reply target and provider metadata, and it MUST NOT claim that assignment, decomposition, approval, or change-request actions succeeded when the backend only acknowledged receipt.

#### Scenario: Slack or Discord callback returns a real action outcome
- **WHEN** a Slack Block Kit action or Discord component interaction is normalized into the shared action contract
- **THEN** the backend executes the mapped task, agent, or review workflow instead of returning a placeholder acknowledgement
- **AND** the Bridge renders the resulting started, blocked, failed, or completed outcome back through the preserved reply target

#### Scenario: Feishu or DingTalk card action preserves provider-aware completion semantics
- **WHEN** a Feishu or DingTalk card action is normalized into the shared action contract with preserved callback metadata
- **THEN** the backend returns a truthful action outcome plus the reply-target context needed for the provider-aware completion path
- **AND** the Bridge may use immediate callback response, delayed update, or explicit fallback according to the provider capability matrix

#### Scenario: Unsupported or stale callback remains explicit
- **WHEN** a normalized platform callback refers to an invalid, stale, or unsupported action transition
- **THEN** the backend returns an explicit failed or blocked outcome
- **AND** the platform response does not claim the business mutation succeeded
