## ADDED Requirements

### Requirement: Bridge SHALL load tenant definitions from a declarative YAML configuration

IM Bridge SHALL read tenant definitions from the file referenced by `IM_TENANTS_CONFIG` at startup and whenever a SIGHUP is delivered. Each tenant entry MUST provide a stable `id`, a backend `projectId`, a display `name`, a list of resolver bindings, a per-provider credential reference block, and an optional plugin allow-scope. The runtime MUST fail startup with an actionable error when the file is missing, malformed, references an unknown provider, or defines duplicate tenant ids.

#### Scenario: Valid tenants.yaml produces a tenant registry
- **WHEN** `IM_TENANTS_CONFIG=./tenants.yaml` points at a file declaring tenants `acme` and `beta`
- **THEN** the runtime exposes both tenants to the resolver and the client factory after startup
- **AND** each tenant's `projectId`, credentials, and plugin scope are accessible through the registry

#### Scenario: Duplicate tenant id rejected at startup
- **WHEN** `tenants.yaml` declares two entries with `id: acme`
- **THEN** startup exits non-zero with an error that names the duplicated id
- **AND** the bridge does not register any tenant derived from that file

#### Scenario: SIGHUP reloads tenant definitions without process restart
- **WHEN** an operator edits `tenants.yaml` to add tenant `gamma` and sends SIGHUP
- **THEN** the runtime swaps in the new tenant registry atomically
- **AND** in-flight messages for existing tenants continue against the snapshot they started under

### Requirement: Tenant resolver SHALL select the tenant for every inbound message

The runtime SHALL invoke a `TenantResolver` on every inbound message after provider-level normalization and before engine dispatch. The resolver MUST support at least the chat-id, workspace-id, and domain resolvers, and MUST be composable so a single resolver chain can combine multiple strategies. Resolution MUST return exactly one tenant or a miss; on miss, the runtime MUST apply the configured default-tenant fallback if enabled, and MUST otherwise reject the message.

#### Scenario: Chat-id resolver maps a Feishu group to the ACME tenant
- **WHEN** a Feishu message arrives from chat `oc_abc123` and the resolver maps `oc_abc123` to tenant `acme`
- **THEN** the runtime tags the message with `TenantID=acme` before engine dispatch
- **AND** downstream reply-target construction carries `TenantID=acme`

#### Scenario: Resolver miss with fallback disabled rejects the message
- **WHEN** a Slack command arrives from a workspace that is not mapped to any tenant and no default fallback is configured
- **THEN** the engine does not dispatch the message
- **AND** the bridge optionally replies to the chat with an explicit "this conversation is not bound to a tenant" notice when per-tenant replies are allowed
- **AND** an audit event with `status=rejected metadata.reason=tenant_unresolved` is emitted

#### Scenario: Resolver miss with default fallback routes to the fallback tenant
- **WHEN** a resolver miss occurs and `IM_TENANT_DEFAULT=acme` is configured
- **THEN** the message is tagged `TenantID=acme` and proceeds through engine dispatch
- **AND** the audit event notes `metadata.tenant_source=default_fallback`

### Requirement: Tenant context SHALL flow from inbound routing through outbound delivery

Every `Message`, `ReplyTarget`, and `DeliveryEnvelope` produced inside the bridge after resolver dispatch MUST carry a non-empty `TenantID`. Client calls to the AgentForge backend MUST route through a tenant-aware factory that binds the outgoing request to the tenant's `projectId` and resolved API credential, and MUST NOT use a process-global `AGENTFORGE_PROJECT_ID`. Control-plane delivery envelopes inbound from the backend MUST also preserve `TenantID` for capability routing and audit.

#### Scenario: Tenant-scoped backend call is routed to ACME's project
- **WHEN** a command handler running under tenant `acme` calls the client factory to create a task
- **THEN** the outgoing HTTP request carries `acme`'s `projectId` and API credential
- **AND** the response is handled in the same tenant scope for reply and audit

#### Scenario: Outbound delivery envelope includes TenantID
- **WHEN** the backend queues an outbound delivery for tenant `beta`
- **THEN** the delivery envelope delivered to the bridge carries `tenantId=beta`
- **AND** the bridge uses `beta`'s provider credential when the delivery fans out to the IM platform

#### Scenario: Cross-tenant leakage is blocked at the factory
- **WHEN** a handler accidentally invokes `factory.For(tenantA)` while dispatching a message resolved to `tenantB`
- **THEN** the factory returns an error rather than a silently cross-bound client
- **AND** the audit event captures `metadata.reason=tenant_scope_mismatch`

### Requirement: Tenant-scoped credentials SHALL be resolved through declared sources

Tenant credential references in `tenants.yaml` MUST accept at minimum `source: env` with a `keyPrefix` (the runtime reads `${KEY_PREFIX}APP_ID`, `${KEY_PREFIX}APP_SECRET`, etc.) and `source: file` with an absolute path to a credential file. The runtime MUST redact tenant credentials in logs and audit payloads and MUST NOT ship tenant credentials to the backend except through the opaque binding identifier already used by the control plane.

#### Scenario: Env-prefixed credentials resolve at startup
- **WHEN** tenant `acme` declares `source: env keyPrefix: FEISHU_ACME_` and env vars `FEISHU_ACME_APP_ID` / `FEISHU_ACME_APP_SECRET` are set
- **THEN** the Feishu provider uses those values when acting under tenant `acme`
- **AND** no AD-hoc global fallback credential is consulted

#### Scenario: Missing credential for an active tenant fails the tenant
- **WHEN** tenant `beta` declares credentials with prefix `DINGTALK_BETA_` but the env vars are absent
- **THEN** startup exits non-zero with an error naming tenant `beta` and the provider
- **AND** tenants whose credentials are present still participate in routing

#### Scenario: Credentials are redacted from logs
- **WHEN** an audit or error log would otherwise reference a tenant's API secret
- **THEN** the logged representation shows `***redacted***` in place of the secret value
- **AND** no secret bytes are written to the audit writer payload
