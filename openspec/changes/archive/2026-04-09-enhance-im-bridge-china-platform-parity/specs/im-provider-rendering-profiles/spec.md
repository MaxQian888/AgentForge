## MODIFIED Requirements

### Requirement: Active IM provider SHALL publish a rendering profile
Each runnable IM provider SHALL define supported text formatting modes, structured rendering preferences, message length limits, mutable-update constraints, accepted native surfaces, and a readiness tier that matches the live adapter's real behavior. Chinese platform profiles MUST only advertise provider surfaces that can actually be produced by the current adapter and reply-target lifecycle, rather than inheriting Feishu's richest behavior by implication.

#### Scenario: Feishu provider publishes full native-card rendering profile
- **WHEN** the Feishu provider initializes
- **THEN** its rendering profile declares readiness tier `full_native_lifecycle`, supports Feishu-native card payloads, and reports delayed card update compatibility

#### Scenario: DingTalk provider publishes ActionCard-send profile without card-update support
- **WHEN** the DingTalk provider initializes
- **THEN** its rendering profile declares readiness tier `native_send_with_fallback`, includes `dingtalk_card` as an accepted native surface, and reports that mutable card updates are unavailable

#### Scenario: WeCom provider publishes template-card-aware fallback profile
- **WHEN** the WeCom provider initializes
- **THEN** its rendering profile declares readiness tier `native_send_with_fallback`, includes only the WeCom native surfaces the adapter can actually send, and reports richer update limits explicitly

#### Scenario: QQ Bot provider publishes markdown-first profile
- **WHEN** the QQ Bot provider initializes
- **THEN** its rendering profile declares readiness tier `markdown_first`, includes `qqbot_markdown` as an accepted native surface, and does not advertise card mutation support

#### Scenario: QQ provider publishes text-first profile
- **WHEN** the QQ provider initializes
- **THEN** its rendering profile declares readiness tier `text_first`, leaves native surfaces empty, and routes all richer requests to explicit text or link fallback
