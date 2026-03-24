## ADDED Requirements

### Requirement: Bridge resolves provider and model defaults from one registry
The TypeScript Bridge SHALL maintain one canonical provider registry that defines each supported provider name, the AI capabilities it supports, and the default model used when a request omits `provider` or `model`.

#### Scenario: Request omits provider and model
- **WHEN** the Bridge receives an AI request without explicit `provider` or `model`
- **THEN** the Bridge SHALL resolve both values from the registry defaults for the requested capability
- **THEN** downstream handlers SHALL consume the resolved provider configuration instead of hard-coded fallback values

#### Scenario: Supported provider is registered for only some capabilities
- **WHEN** the Bridge starts with a provider that is configured for `text_generation` but not for `agent_execution`
- **THEN** the registry SHALL preserve that capability distinction
- **THEN** the Bridge SHALL NOT treat the provider as universally available across all AI paths

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
