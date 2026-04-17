# im-bridge-multi-provider-runtime Specification

## Purpose
Define how a single IM Bridge process hosts multiple live provider transports concurrently — covering per-provider isolation, shared-port path routing, parallel lifecycle management, and per-provider secret overrides — so that one bridge can serve Feishu, DingTalk, WeCom, and other providers without collapsing their state into a global singleton.

## Requirements

### Requirement: Bridge runtime SHALL host multiple live providers in a single process

IM Bridge SHALL accept a comma-separated `IM_PLATFORMS` configuration and concurrently start a live transport for every declared provider within a single process. Each provider MUST run with its own `activeProvider` instance carrying independent platform metadata, reply plan, rate limiter, callback receiver mount, and capability matrix. Startup MUST fail fast with an actionable error identifying the offending provider when any declared provider has an incomplete credential or transport configuration, and MUST NOT silently fall back to a stub or drop that provider from the runtime.

#### Scenario: Three providers boot concurrently in one process
- **WHEN** Bridge is configured with `IM_PLATFORMS=feishu,dingtalk,wecom` and each provider's credentials are present
- **THEN** the runtime resolves three independent `activeProvider` instances through the shared provider contract
- **AND** each provider's live transport, capability matrix, and rate limiter are registered without sharing mutable state across providers

#### Scenario: Misconfigured provider fails the whole process
- **WHEN** `IM_PLATFORMS=feishu,dingtalk` but DingTalk credentials are missing
- **THEN** startup exits non-zero with an error naming `dingtalk` and the missing field
- **AND** Feishu does not start partially under the impression the rest of the fleet is healthy

#### Scenario: Single-provider configuration remains supported
- **WHEN** Bridge is configured with `IM_PLATFORMS=feishu` (or legacy `IM_PLATFORM=feishu`)
- **THEN** the runtime starts a single `activeProvider` with the same semantics as the prior single-provider deployment
- **AND** control-plane registration still succeeds for that single provider

### Requirement: Provider HTTP mounts SHALL share one port via path prefix routing

The notify/callback HTTP server SHALL expose every declared provider through a deterministic path prefix rooted at the single `NOTIFY_PORT`. Built-in provider-specific paths MUST be prefixed with the normalized provider id (for example `/feishu/*`, `/dingtalk/*`, `/wecom/*`), and generic endpoints under `/im/*` MUST demultiplex to the right provider using the normalized `X-IM-Source` header or an explicit query parameter. Requests that cannot be resolved to a registered provider MUST be rejected with `404 provider_not_registered`.

#### Scenario: Feishu webhook path reaches only the Feishu provider
- **WHEN** a Feishu webhook arrives at `POST /feishu/webhook` while Feishu and DingTalk are both active
- **THEN** only the Feishu provider handles the payload
- **AND** the DingTalk provider does not observe the request

#### Scenario: Generic endpoint uses X-IM-Source to select the provider
- **WHEN** a signed `/im/notify` request carries `X-IM-Source: dingtalk` and a payload targeted at an active DingTalk tenant
- **THEN** the DingTalk provider's receiver handles the delivery
- **AND** the Feishu provider does not observe the request

#### Scenario: Unknown provider prefix is rejected
- **WHEN** a request arrives at `POST /slack/webhook` but `slack` is not in `IM_PLATFORMS`
- **THEN** the server responds `404 provider_not_registered`
- **AND** no audit event is emitted on behalf of another provider

### Requirement: Provider lifecycle SHALL be managed with isolated startup and shutdown

The runtime SHALL start every provider in its own goroutine and MUST aggregate any startup errors before promoting the process to ready. On shutdown the runtime MUST drain all providers in parallel with a shared deadline, close each provider's transport independently, and log per-provider shutdown outcomes so operators can diagnose a hung provider without blocking the others.

#### Scenario: One slow provider does not starve other shutdowns
- **WHEN** the process receives SIGTERM while DingTalk's shutdown is stuck on a remote close
- **THEN** Feishu and WeCom still complete their shutdown within the shared deadline
- **AND** the operator log names DingTalk as the unfinished provider with its last-known state

#### Scenario: Provider readiness waits for all providers
- **WHEN** Feishu finishes startup but WeCom is still negotiating its callback
- **THEN** the Bridge readiness probe reports `not_ready` until WeCom completes or times out
- **AND** the control plane does not mark the bridge instance eligible for outbound delivery prematurely

### Requirement: Per-provider secret override SHALL take precedence over shared secret

The Bridge SHALL resolve the HMAC shared secret for each provider by checking `IM_SECRET_<PROVIDER>` first (for example `IM_SECRET_FEISHU`), falling back to `IM_CONTROL_SHARED_SECRET` only when no provider-specific override exists. Operators MUST NOT be required to unify secrets across providers simply to run them in one process.

#### Scenario: Feishu uses a provider-specific secret while DingTalk uses the shared secret
- **WHEN** `IM_SECRET_FEISHU=feishu-secret` and `IM_CONTROL_SHARED_SECRET=shared-secret` are both set
- **THEN** signatures on Feishu-scoped control-plane requests are verified with `feishu-secret`
- **AND** signatures on DingTalk-scoped requests are verified with `shared-secret`

#### Scenario: Missing both secrets fails startup
- **WHEN** a provider has neither `IM_SECRET_<PROVIDER>` nor `IM_CONTROL_SHARED_SECRET`
- **THEN** startup exits non-zero with an error identifying the provider
- **AND** the process does not accept control-plane requests signed with an empty key
