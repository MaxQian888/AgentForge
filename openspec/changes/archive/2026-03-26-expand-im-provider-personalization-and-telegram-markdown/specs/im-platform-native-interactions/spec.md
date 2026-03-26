## ADDED Requirements

### Requirement: Telegram interaction completions SHALL honor markdown-aware mutable-update safety
When a Telegram callback query or command completion is rendered back to the originating reply target, the Bridge SHALL evaluate the completion through Telegram's rendering profile before choosing `editMessageText`, reply, or follow-up delivery. The Bridge MUST use a Telegram formatted-text path only when the content is safe for the provider's formatting and mutable-update rules, and it MUST otherwise fall back to a supported plain-text edit or reply path.

#### Scenario: Safe Telegram callback completion edits the original message
- **WHEN** a Telegram callback query finishes with content that is safe for the provider-selected text mode and the preserved reply target supports editing
- **THEN** the Bridge answers the callback query and updates the originating Telegram message in place through the Telegram-native mutable update path
- **AND** the user does not receive an unnecessary duplicate completion message

#### Scenario: Unsafe Telegram formatted completion falls back before edit
- **WHEN** a Telegram callback completion requests formatted text that cannot be rendered safely for the preserved reply target
- **THEN** the Bridge answers the callback query and falls back to a supported plain-text edit or reply
- **AND** it does not send malformed Markdown-aware content through `editMessageText`

#### Scenario: Oversized Telegram completion degrades to segmented follow-up
- **WHEN** a Telegram callback completion exceeds the provider's editable text limits for the originating message context
- **THEN** the Bridge abandons the single-message edit plan and uses a provider-supported segmented reply or follow-up strategy
- **AND** the completion remains tied to the originating Telegram interaction context through preserved reply-target metadata
