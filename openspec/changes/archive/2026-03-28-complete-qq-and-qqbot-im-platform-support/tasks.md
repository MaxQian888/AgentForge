## 1. Shared Provider And Runtime Seams

- [x] 1.1 Extend `src-im-bridge/cmd/bridge` config and provider registry to recognize `qq` and `qqbot` through the existing descriptor-driven startup path.
- [x] 1.2 Add QQ-family capability metadata, rendering profile declarations, source normalization, and control-plane registration coverage to the shared bridge/runtime seams without reintroducing provider-name switch logic in shared delivery paths.
- [x] 1.3 Add or update focused tests for shared startup, source propagation, reply-target replay, and platform-matching delivery behavior covering `qq` and `qqbot`.

## 2. QQ Provider

- [x] 2.1 Implement `src-im-bridge/platform/qq` stub and live provider entrypoints that satisfy the existing `core.Platform`-family interfaces and validate QQ live transport configuration.
- [x] 2.2 Normalize QQ inbound messages into the shared command surface, preserving QQ conversation and reply-target context for later updates and backend source propagation.
- [x] 2.3 Implement QQ outbound delivery and explicit richer-content downgrade behavior through a provider-owned rendering profile, with focused tests for supported send paths and truthful fallback metadata.

## 3. QQ Bot Provider

- [x] 3.1 Implement `src-im-bridge/platform/qqbot` stub and live provider entrypoints that satisfy the existing `core.Platform`-family interfaces and validate QQ Bot live transport configuration.
- [x] 3.2 Normalize QQ Bot inbound messages or interactions into the shared command surface, preserving QQ Bot conversation and reply-target context for later updates and backend source propagation.
- [x] 3.3 Implement QQ Bot outbound delivery and explicit richer-content downgrade behavior through a provider-owned rendering profile, with focused tests for supported send paths and truthful fallback metadata.

## 4. Docs, Smoke, And Verification

- [x] 4.1 Update `src-im-bridge/README.md`, `src-im-bridge/docs/platform-runbook.md`, and any related IM matrix docs so QQ and QQ Bot appear as runnable providers with truthful capability and downgrade notes.
- [x] 4.2 Add QQ and QQ Bot smoke fixtures or script coverage that exercises stub-mode command and notification flows through the same local smoke entrypoints used by the existing providers.
- [x] 4.3 Run `openspec validate --specs` plus scoped `src-im-bridge` verification for QQ-family provider startup, delivery, and replay seams, then mark the change ready for `/opsx:apply`.
