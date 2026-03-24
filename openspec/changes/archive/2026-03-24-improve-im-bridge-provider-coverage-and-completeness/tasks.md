## 1. Runtime descriptors and shared platform seams

- [x] 1.1 Introduce a `PlatformDescriptor` registry plus explicit `IM_TRANSPORT_MODE=live|stub` handling so platform selection, config validation, capabilities, and live/stub factories are no longer hardcoded in `cmd/bridge/main.go`.
- [x] 1.2 Extend shared IM Bridge abstractions (`core`, `client`, `notify`) to carry normalized platform source, capability metadata, and capability-aware rich-message fallback without breaking existing command handlers.
- [x] 1.3 Add targeted tests for platform normalization, descriptor lookup, live/stub mode validation, and startup failure behavior for incomplete platform configuration.

## 2. Bring existing providers to live transport completeness

- [x] 2.1 Replace the Feishu stub-first runtime path with a live transport adapter that prefers long connection for supported event or callback flows and keeps any required HTTP callback seam explicit.
- [x] 2.2 Implement Slack live transport using Socket Mode, including envelope acknowledgement, reconnect or refresh handling, command normalization, and structured reply fallback.
- [x] 2.3 Implement DingTalk live transport using Stream mode by default, including credential validation, inbound message mapping, reply delivery, and structured notification fallback.

## 3. Add Telegram and Discord platform support

- [x] 3.1 Add a Telegram platform adapter that supports one official update intake mode at a time, starts with long polling first, and maps Telegram commands and replies into the shared engine contract.
- [x] 3.2 Add a Discord platform adapter that registers or syncs application commands, validates interaction requests, sends required initial acknowledgements on time, and completes user-visible replies through the supported follow-up path.
- [x] 3.3 Wire Telegram and Discord into backend source propagation, notification platform matching, capability reporting, and platform selection documentation so they behave like first-class active platforms.

## 4. Verification, docs, and operational readiness

- [x] 4.1 Add contract and integration coverage for the live transport rules defined in the spec, including Slack ack behavior, Telegram intake exclusivity, Discord deferred response flow, and platform mismatch rejection.
- [x] 4.2 Keep explicit stub or fake transport test seams for local development, and add smoke-test fixtures or scripts for each supported platform so regressions can be reproduced without a full third-party environment.
- [x] 4.3 Update `src-im-bridge/README.md` and related IM Bridge docs with credential requirements, preferred transport mode per platform, rollout and rollback guidance, and a manual verification matrix for Feishu, Slack, DingTalk, Telegram, and Discord.
