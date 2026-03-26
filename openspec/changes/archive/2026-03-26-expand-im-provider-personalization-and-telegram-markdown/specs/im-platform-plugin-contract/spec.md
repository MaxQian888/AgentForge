## ADDED Requirements

### Requirement: IM Bridge platform providers SHALL declare rendering profile metadata
In addition to transport and callback capabilities, each IM Bridge platform provider SHALL declare rendering profile metadata that describes provider-specific formatting modes, structured rendering preferences, message length constraints, mutable update rules, and any optional builder surfaces needed for richer provider-native payload construction. Shared Bridge paths MUST consume that metadata through the provider contract instead of maintaining a separate platform-name switch for rendering behavior.

#### Scenario: Minimal provider declares only plain-text rendering
- **WHEN** a provider supports only base send, reply, and plain-text delivery
- **THEN** its descriptor declares a minimal rendering profile with plain-text constraints only
- **AND** the Bridge continues to route shared command and notification flows through that provider without requiring richer builders

#### Scenario: Feishu provider declares richer rendering builders
- **WHEN** the Feishu provider is loaded through the provider contract
- **THEN** its descriptor declares rendering metadata for text, `lark_md`, JSON-card, template-card, and delayed-update-aware native builders
- **AND** shared delivery code can discover those richer surfaces without a Feishu-only branch outside the provider layer

#### Scenario: Telegram provider declares formatted-text constraints
- **WHEN** the Telegram provider is loaded through the provider contract
- **THEN** its descriptor declares the formatted-text and mutable-update constraints required for safe Markdown-aware delivery
- **AND** shared delivery code can choose between Markdown-capable rendering and plain-text fallback through the provider contract alone
