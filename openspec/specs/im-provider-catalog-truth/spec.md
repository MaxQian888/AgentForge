# im-provider-catalog-truth Specification

## Purpose
Define the authoritative operator-facing IM provider catalog contract so the Go backend exposes a canonical, always-available provider list that frontend operator surfaces and IM API validation can consume instead of relying on stale frontend constants or flat per-surface allowlists.
## Requirements
### Requirement: Backend SHALL expose an authoritative operator-facing IM provider catalog
The Go backend SHALL expose an operator-facing IM provider catalog at a canonical authenticated API surface so `/im`, settings, and other operator workspaces can discover the supported IM providers without relying on stale frontend constants. The catalog MUST include one entry per operator-visible built-in provider, MUST remain available even when no bridge instance for that provider is currently registered, and MUST expose enough metadata for operator surfaces to render truthful controls: normalized provider id, display label, interaction class, test-send support, channel-configuration support, and provider-specific configuration field schema.

#### Scenario: Catalog includes WeChat and Email with truthful affordances
- **WHEN** an operator requests the canonical IM provider catalog
- **THEN** the response includes entries for `wechat` and `email`
- **AND** the `wechat` entry is marked as an interactive chat provider
- **AND** the `email` entry is marked as delivery-only instead of pretending full callback or slash-command parity

#### Scenario: Catalog remains available without active bridge registration
- **WHEN** no active bridge instance is currently registered for WeChat or Email
- **THEN** the canonical IM provider catalog still returns those provider entries
- **AND** operator configuration surfaces can prepare channels and validation rules without waiting for a live bridge instance

### Requirement: IM API validation SHALL honor provider affordances per surface
Operator-facing IM APIs SHALL validate provider ids according to the authoritative provider catalog instead of using one flat platform allowlist across every surface. Interactive inbound surfaces such as message or action handling MUST accept only providers whose catalog affordance includes interactive chat input. Delivery surfaces such as channel configuration, outbound send, notify, or operator test-send MUST continue to accept delivery-capable providers even when they are delivery-only.

#### Scenario: Interactive surface accepts WeChat and rejects Email
- **WHEN** an inbound IM message or action request identifies `wechat` as the provider
- **THEN** the backend accepts the provider as a valid interactive IM source
- **AND** the same interactive validation rejects `email` as a fake chat-command provider

#### Scenario: Delivery surface accepts Email as a valid provider
- **WHEN** an operator saves an Email channel configuration or submits an Email test-send request
- **THEN** the backend accepts `email` as a valid delivery-capable provider
- **AND** the response or validation metadata keeps Email marked as delivery-only rather than upgrading it to interactive chat parity

### Requirement: Catalog SHALL expose bridge-level provider and tenant bindings per instance

The operator-facing IM provider catalog SHALL extend each catalog entry with a `bindings` array that describes which `bridge_id` instances currently advertise the provider and which `tenantId` values each of those bridges serves. Each binding entry MUST include `bridgeId`, `providerId`, `tenantId`, the readiness tier and mutable-update method reported by that provider on that bridge, and the registration-derived liveness state (`live`, `stale`, or `revoked`). The catalog MUST continue to return the provider entry even when no binding exists, with an empty `bindings` array rather than omitting the provider.

#### Scenario: Multi-provider bridge adds bindings under each provider entry
- **WHEN** bridge `A` registers with `providers=[feishu, dingtalk]` and tenants `acme`+`beta` on Feishu and `acme` on DingTalk
- **THEN** the Feishu catalog entry lists bindings `(A, feishu, acme)` and `(A, feishu, beta)`
- **AND** the DingTalk catalog entry lists binding `(A, dingtalk, acme)`
- **AND** both bindings carry the readiness tier and mutable-update method declared by bridge `A`

#### Scenario: Provider without any live binding still appears in the catalog
- **WHEN** no live bridge advertises WeCom
- **THEN** the WeCom catalog entry is still returned with `bindings=[]`
- **AND** operator surfaces can still prepare WeCom channel configuration without a live bridge

#### Scenario: Revoked or stale binding is marked, not hidden
- **WHEN** bridge `B`'s DingTalk registration is revoked after a failed heartbeat
- **THEN** the DingTalk catalog entry retains the binding with `liveness=revoked` for a bounded grace period
- **AND** interactive APIs filter out `revoked` bindings when selecting a live delivery target

### Requirement: Catalog entries SHALL advertise per-binding capability snapshots

Each `bindings[]` item in the catalog SHALL carry a capability snapshot that mirrors what the bridge reported for that `(providerId, tenantId)` pair at registration time, including the `readiness_tier`, `mutable_update_method`, structured-content support, reply-target support, and any provider-specific flags such as `template_card_update` or `openapi_patch`. The capability snapshot MUST reflect the most recent heartbeat so operator UIs see accurate per-tenant, per-provider, per-bridge affordances instead of a flat provider-level default.

#### Scenario: DingTalk capability snapshot differs between bridges
- **WHEN** bridge `A` advertises DingTalk with `mutable_update_method=openapi_only` and bridge `B` advertises DingTalk with `mutable_update_method=none`
- **THEN** the DingTalk catalog entry's `bindings[]` shows each bridge's distinct snapshot
- **AND** operator surfaces can explain why a delivery routed to `B` lacks mutable update support

#### Scenario: Heartbeat updates a capability snapshot in place
- **WHEN** bridge `A`'s next heartbeat reports a degraded `feishu` callback health
- **THEN** the Feishu catalog binding for `A` reflects the new diagnostics in its capability snapshot
- **AND** the update is visible to operator surfaces without an additional API call
