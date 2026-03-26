## 1. Provider Rendering Profile Foundation

- [x] 1.1 Extend the IM provider descriptor in `src-im-bridge/cmd/bridge` to declare provider-owned rendering profile metadata alongside existing capability metadata and factory hooks.
- [x] 1.2 Define the shared rendering profile and rendering plan types in `src-im-bridge/core`, including formatting mode, length limits, structured/native preferences, mutable-update constraints, and fallback metadata.
- [x] 1.3 Populate default rendering profiles for the built-in providers and keep `IM_PLATFORM`, `IM_TRANSPORT_MODE`, and current environment-variable contracts unchanged.

## 2. Typed Delivery Refactor

- [x] 2.1 Refactor `core.DeliverEnvelope(...)` and related delivery helpers to resolve a provider-aware rendering plan before transport execution.
- [x] 2.2 Update `src-im-bridge/notify` action completion, compatibility `/im/send`, compatibility `/im/notify`, and replay paths to consume the shared rendering-plan flow instead of formatting provider output inline.
- [x] 2.3 Preserve and expose provider-aware fallback metadata when the rendering plan downgrades formatted text, mutable updates, or richer native content.

## 3. Telegram Markdown-Aware Delivery

- [x] 3.1 Extend the Telegram adapter request models and sender interfaces to support formatted-text delivery inputs needed for `sendMessage` and `editMessageText`.
- [x] 3.2 Implement a Telegram renderer that applies plain-first, markdown-when-safe selection, including MarkdownV2 escaping, oversized-text segmentation, and edit-versus-follow-up decision rules.
- [x] 3.3 Route Telegram structured notifications and callback completions through the Telegram renderer so inline keyboard, formatted text, and reply-target update behavior follow one consistent contract.

## 4. Feishu Message And Card Builders

- [x] 4.1 Introduce provider-owned Feishu builders for plain text, `lark_md` content blocks, JSON cards, and template cards without leaking raw Feishu payload assembly into shared delivery code.
- [x] 4.2 Align the Feishu builder outputs with preserved reply-target update policy so immediate callback response, delayed card update, reply, and send paths choose compatible richer output forms.
- [x] 4.3 Update Feishu-targeted notify and action-completion flows to prefer builder-owned card/message construction while preserving explicit fallback reasons for unusable delayed-update context.

## 5. Verification And Documentation

- [x] 5.1 Add focused tests for provider rendering profiles, rendering-plan resolution, and provider-aware fallback metadata across shared delivery paths.
- [x] 5.2 Add Telegram-focused tests for safe Markdown rendering, plain-text fallback, oversized completion handling, and mutable-update routing.
- [x] 5.3 Add Feishu-focused tests for builder-owned text/card construction, template-card inputs, delayed-update compatibility checks, and explicit fallback behavior.
- [x] 5.4 Update `src-im-bridge/README.md`, `src-im-bridge/docs/platform-runbook.md`, and related notes to document provider rendering profiles, Telegram Markdown behavior, Feishu builder surfaces, and remaining live-only verification steps.
