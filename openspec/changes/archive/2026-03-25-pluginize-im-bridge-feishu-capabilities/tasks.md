## 1. Provider Contract Foundation

- [x] 1.1 Introduce a bridge-local provider descriptor, registry, and loader in `src-im-bridge` that can represent built-in and future plugin-backed IM providers with normalized identity, transport support, capability metadata, and factory hooks.
- [x] 1.2 Wrap the existing `feishu`, `slack`, `dingtalk`, `telegram`, and `discord` adapters behind the new provider contract without changing the current `IM_PLATFORM`, `IM_TRANSPORT_MODE`, or credential environment variables.
- [x] 1.3 Replace `cmd/bridge/platform_registry.go` startup branching with provider-contract-based resolution and actionable validation errors for unknown providers, unsupported transport modes, and incomplete configuration.

## 2. Capability And Control-Plane Alignment

- [x] 2.1 Move provider-owned capability metadata and richer extension declarations into the new contract so Bridge health, registration, and reply-plan selection no longer infer behavior from provider name checks alone.
- [x] 2.2 Update the runtime bootstrap, health reporting, and control-plane registration surfaces to consume the provider descriptor output while preserving the current single-active-provider-per-process model.
- [x] 2.3 Add explicit unsupported-provider handling for roadmap-only ids such as `wecom` so startup failures distinguish “not yet runnable” from ordinary misconfiguration.

## 3. Feishu Rich Card Lifecycle

- [x] 3.1 Introduce a typed Feishu-native card payload model that can represent both raw JSON cards and template cards with template id, optional version, and template variables without overloading `core.StructuredMessage`.
- [x] 3.2 Extend the Feishu provider send/reply path to deliver JSON cards and template cards through the native Feishu message APIs, while preserving truthful fallback behavior when native delivery is unavailable.
- [x] 3.3 Normalize Feishu `card.action.trigger` callbacks into the shared backend action contract using the current callback schema, preserving message identity, operator identity, callback token, and delayed-update context.
- [x] 3.4 Implement the Feishu dual-phase card interaction lifecycle so immediate callback acknowledgements, toast responses, and delayed card updates use the correct native path before falling back to plain reply or send behavior.

## 4. Notify And Delivery Integration

- [x] 4.1 Extend `src-im-bridge/notify` and related delivery helpers so provider-native Feishu card payloads can be selected ahead of generic structured-message fallback when the target platform and reply target support that path.
- [x] 4.2 Record explicit fallback reasons when Feishu delayed-update context is missing, expired, exhausted, or otherwise invalid, so operators can distinguish native card failures from generic delivery errors.
- [x] 4.3 Ensure long-running Feishu card actions reuse preserved reply-target context for native updates instead of silently collapsing into duplicate plain-text notifications.

## 5. Verification And Documentation

- [x] 5.1 Add focused tests for provider descriptor resolution, startup validation, and unsupported-provider behavior across the built-in IM providers.
- [x] 5.2 Add Feishu-focused tests for JSON card delivery, template-card delivery, callback normalization, immediate callback responses, delayed updates, and explicit fallback when delayed-update context cannot be used.
- [x] 5.3 Update `src-im-bridge/README.md`, `src-im-bridge/docs/platform-runbook.md`, and any related IM Bridge notes to document the provider contract, Feishu richer card lifecycle, rollout expectations, and remaining future-provider gaps.
- [x] 5.4 Run the relevant `src-im-bridge` test suites plus any focused smoke fixtures for Feishu/provider bootstrap changes, then capture any remaining live-only verification steps required before rollout.
