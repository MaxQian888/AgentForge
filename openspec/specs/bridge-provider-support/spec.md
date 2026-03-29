# bridge-provider-support Specification

## Purpose
Define the canonical TypeScript Bridge contract for provider-aware AI execution, including shared provider/model resolution, capability validation, credential checks, and Vercel AI SDK-backed lightweight generation paths.
## Requirements
### Requirement: Bridge resolves provider and model defaults from one registry

The canonical provider registry SHALL define supported provider names, AI capabilities, default models, and IM platform capability descriptors. For IM providers, the registry entry SHALL include the full capability matrix: command surface, structured-message surface, callback mode, async update mode, message scope, mutability, and card support.

WeCom, QQ, and QQ Bot providers SHALL have complete capability matrix entries in the registry, matching the level of detail already present for Feishu, Slack, Telegram, Discord, and DingTalk.

#### Scenario: WeCom provider registered with complete capabilities
- **WHEN** the provider registry initializes
- **THEN** WeCom entry declares `{commandSurface: "callback", structuredMessage: "text", callback: "callback", asyncUpdate: "appMessage", card: false, cardUpdate: false}`

#### Scenario: QQ provider registered with complete capabilities
- **WHEN** the provider registry initializes
- **THEN** QQ entry declares `{commandSurface: "websocket", structuredMessage: "text", callback: "websocket", asyncUpdate: "reply", card: false, cardUpdate: false}`

#### Scenario: QQ Bot provider registered with complete capabilities
- **WHEN** the provider registry initializes
- **THEN** QQ Bot entry declares `{commandSurface: "webhook", structuredMessage: "text", callback: "webhook", asyncUpdate: "openapi", card: false, cardUpdate: false}`

### Requirement: Bridge validates provider-aware requests before execution
The TypeScript Bridge SHALL validate `provider` and `model` values before invoking any runtime, and it MUST fail requests with explicit errors when the provider is unknown, unavailable, or does not support the requested capability.

#### Scenario: Request references an unknown provider
- **WHEN** a request specifies a provider name that is not present in the registry
- **THEN** the Bridge SHALL reject the request before invoking any upstream model runtime
- **THEN** the error SHALL identify that the provider is unsupported or unknown

#### Scenario: Request targets an unsupported capability
- **WHEN** a request specifies a provider that exists in the registry but does not support the requested capability
- **THEN** the Bridge SHALL reject the request before starting execution
- **THEN** the error SHALL identify that the provider does not support that capability

### Requirement: Bridge uses Vercel AI SDK for lightweight multi-provider generation
The TypeScript Bridge SHALL execute lightweight text-generation workflows through Vercel AI SDK and its provider packages for every provider marked as supporting `text_generation`.

#### Scenario: Lightweight request uses a configured real provider
- **WHEN** the Bridge resolves a lightweight AI request to a provider that supports `text_generation`
- **THEN** it SHALL invoke that provider through Vercel AI SDK rather than returning a simulated placeholder response
- **THEN** the Bridge SHALL validate the structured output before returning it to the caller

#### Scenario: Required provider credentials are missing
- **WHEN** a lightweight AI request resolves to a provider whose required credentials are not configured
- **THEN** the Bridge SHALL fail the request explicitly
- **THEN** it SHALL NOT silently fall back to a different provider unless that fallback was configured as the default before request validation

### Requirement: Lightweight AI operations use canonical bridge endpoints
The TypeScript Bridge SHALL expose provider-aware lightweight AI operations through the canonical `/bridge/decompose`, `/bridge/classify-intent`, and `/bridge/generate` endpoints, and Go-side callers plus live project documentation MUST use those endpoints as the primary contract. Compatibility aliases MAY remain for migration, but they SHALL NOT become the preferred entrypoints for new integration work.

#### Scenario: Go task decomposition uses the canonical bridge route
- **WHEN** the Go bridge client requests task decomposition
- **THEN** it SHALL call `/bridge/decompose`
- **THEN** the bridge SHALL apply the same provider-resolution and validation rules defined for lightweight AI requests

#### Scenario: Intent classification documentation avoids legacy primary routes
- **WHEN** project documentation describes IM intent classification through the TS Bridge
- **THEN** it SHALL identify `/bridge/classify-intent` as the primary live endpoint
- **THEN** it SHALL not describe `/api/classify-intent` or similar historical paths as the current canonical contract

#### Scenario: Compatibility alias keeps provider semantics aligned
- **WHEN** a legacy alias such as `/ai/decompose` or `/ai/generate` is invoked for a supported lightweight AI operation
- **THEN** the bridge SHALL enforce the same provider, model, and credential validation behavior as the corresponding canonical `/bridge/*` endpoint
- **THEN** the response semantics SHALL remain equivalent to the canonical route

### Requirement: Go backend exposes generate and classify-intent as REST endpoints
Go backend SHALL proxy lightweight AI operations to Bridge, exposing `POST /api/v1/ai/generate` and `POST /api/v1/ai/classify-intent` behind authentication middleware.

#### Scenario: Generate with explicit provider
- **WHEN** client calls `POST /api/v1/ai/generate` with `{"prompt": "summarize", "provider": "openai", "model": "gpt-4o"}`
- **THEN** Go handler calls Bridge `/bridge/generate` with same payload and returns result

#### Scenario: Generate with default provider
- **WHEN** client calls `POST /api/v1/ai/generate` with `{"prompt": "summarize"}` (no provider)
- **THEN** Bridge resolves default provider from registry and returns result

#### Scenario: Classify intent for IM processing
- **WHEN** IM handler needs to classify user message intent
- **THEN** service calls `bridge.ClassifyIntent()` which routes to `/bridge/classify-intent`

### Requirement: Frontend RuntimeSelector includes provider and model selection
The shared `RuntimeSelector` component SHALL allow selecting provider and model in addition to runtime. Provider options SHALL be filtered based on selected runtime's compatible providers. Model options SHALL default to the provider's default model.

#### Scenario: Provider changes when runtime changes
- **WHEN** user selects runtime `codex` which only supports provider `openai`
- **THEN** provider dropdown auto-selects `openai` and model dropdown shows OpenAI models

#### Scenario: User overrides default model
- **WHEN** user selects provider `anthropic` and changes model from default to `claude-sonnet-4-20250514`
- **THEN** spawn request includes the overridden model value

