## 1. Capability Model And Shared Contracts

- [ ] 1.1 Expand `src-im-bridge/core` platform metadata from coarse booleans into a structured capability matrix that can describe command surface, structured surface, callback mode, async update mode, message scope, and mutability.
- [ ] 1.2 Extend `core.ReplyTarget` and `src-go/internal/model/IMReplyTarget` to carry the native update hints required for edit, follow-up, thread reply, delayed card update, and session-webhook delivery without breaking existing serialized paths.
- [ ] 1.3 Introduce a canonical structured notification payload plus renderer selection seam so platform adapters no longer depend on `core.Card` alone to express interactive output.

## 2. Native Reply And Update Strategies

- [ ] 2.1 Implement a shared reply-strategy layer that resolves whether a delivery should use reply, thread reply, edit, follow-up, session webhook, or plain send based on platform capabilities and preserved reply target.
- [ ] 2.2 Update IM control-plane delivery handling and compatibility `/im/send` / `/im/notify` paths to route through the new reply strategy before falling back to plain text.
- [ ] 2.3 Preserve and restore provider-native reply-target context across Bridge command execution, backend binding, control-plane replay, and reconnect recovery.

## 3. Platform-Specific Native Interaction Support

- [ ] 3.1 Add Slack-native rendering and callback normalization for Block Kit, threaded replies, `response_url`, and modal or interactive action submissions.
- [ ] 3.2 Add Discord-native rendering and callback normalization for deferred acknowledgements, follow-up delivery, original-response editing, and component-driven actions.
- [ ] 3.3 Add Telegram-native rendering and callback normalization for inline keyboards, callback queries, message edits, and topic or thread-aware reply targets where available.
- [ ] 3.4 Add Feishu-native rendering and callback normalization for interactive cards, 3-second callback responses, delayed card updates, and button-driven action payloads.
- [ ] 3.5 Add DingTalk-native rendering and callback normalization for session-webhook replies, ActionCard-style interactions or explicit downgrade behavior, and conversation-scoped reply restoration.

## 4. Backend And API Alignment

- [ ] 4.1 Expand backend IM action and delivery models so `/im/action`, IM bindings, and downstream workflows can consume a single normalized action envelope with provider metadata and preserved reply targets.
- [ ] 4.2 Update backend IM service and control-plane logic to store, replay, and inspect the richer capability matrix without hard-coding provider names for progress delivery decisions.
- [ ] 4.3 Add explicit future-provider gap handling for `wecom` so model-level enums, docs, and runtime activation rules truthfully indicate that the provider is planned but not yet runnable.

## 5. Verification, Docs, And Rollout Safety

- [ ] 5.1 Add focused unit and contract tests for capability-matrix resolution, reply-target restoration, and fallback selection across Slack, Discord, Telegram, Feishu, and DingTalk.
- [ ] 5.2 Add platform-focused adapter tests that cover native action callbacks and native update behavior for each current live provider instead of text-only happy paths.
- [ ] 5.3 Update `src-im-bridge/README.md`, `src-im-bridge/docs/platform-runbook.md`, and related IM design notes with the new platform-feature matrix, downgrade rules, smoke-test steps, and explicit future-provider gaps.
- [ ] 5.4 Run the relevant `src-im-bridge` and backend IM test suites plus scoped smoke validation, then capture any remaining provider-specific manual verification steps required before rollout.
