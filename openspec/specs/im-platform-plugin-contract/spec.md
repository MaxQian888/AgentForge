# im-platform-plugin-contract Specification

## Purpose
Define a plugin-compatible provider contract for IM Bridge platform startup, provider activation, and capability-driven native extensions.

## Requirements
### Requirement: IM Bridge platform providers SHALL declare a plugin-compatible descriptor
The IM Bridge SHALL represent each platform provider through a normalized descriptor that declares provider identity, supported transport modes, configuration validation rules, capability metadata, and optional native extension surfaces. The descriptor MUST be sufficient for the Bridge runtime to resolve startup behavior, health metadata, notification routing, and richer provider-native features without hard-coding provider names in the startup path.

#### Scenario: Built-in Feishu provider exposes a full descriptor
- **WHEN** the Bridge loads the built-in Feishu provider
- **THEN** it resolves a descriptor that includes the normalized provider id, supported transport modes, capability matrix, and richer native card extension metadata
- **AND** startup code does not need a Feishu-specific branch outside the provider loader

#### Scenario: Provider without a valid descriptor cannot be activated
- **WHEN** a future provider package is present but does not expose the required identity, capability, or factory metadata
- **THEN** the Bridge rejects that provider from activation
- **AND** operators receive an actionable provider contract error instead of a silent fallback

### Requirement: Active platform startup SHALL resolve through the provider contract
The IM Bridge SHALL resolve the configured `IM_PLATFORM` through the provider contract and create exactly one active provider instance per process. Built-in providers such as `feishu`, `slack`, `dingtalk`, `telegram`, `discord`, and `wecom` MUST use the same resolution path as any future plugin-backed provider descriptor. Startup MUST fail if the requested provider id is unknown, unsupported for the requested transport mode, or invalid for the current configuration.

#### Scenario: Built-in provider starts through shared resolution
- **WHEN** the Bridge starts with `IM_PLATFORM=telegram`
- **THEN** the runtime resolves the Telegram provider through the shared provider registry
- **AND** it instantiates the Telegram platform without a startup-specific hard-coded branch

#### Scenario: WeCom provider starts through shared resolution
- **WHEN** the Bridge starts with `IM_PLATFORM=wecom` and valid live or stub configuration
- **THEN** the runtime resolves the WeCom provider through the shared provider registry
- **AND** it instantiates the WeCom platform through the same descriptor-driven loader used by the other built-in providers

#### Scenario: Unknown provider id is rejected
- **WHEN** the Bridge starts with `IM_PLATFORM=line`
- **THEN** startup fails with an explicit unsupported-provider error
- **AND** the runtime does not silently substitute another provider or a local stub

### Requirement: Provider-native extensions SHALL remain optional and capability-driven
The provider contract SHALL allow a provider to opt into native structured rendering, native action callbacks, delayed update semantics, or future provider-specific message surfaces without forcing every provider to implement the same richer feature set. Shared Bridge paths MUST choose those extensions through declared capability and extension metadata, and MUST fall back to the cross-platform message path when a provider does not advertise the requested native behavior.

#### Scenario: Minimal provider still supports shared command flow
- **WHEN** a provider declares only the base send, reply, and inbound-message capabilities
- **THEN** the Bridge continues to run the shared command engine for that provider
- **AND** richer notification or callback features fall back to the supported cross-platform path

#### Scenario: WeCom provider advertises only the richer surfaces it can honor
- **WHEN** the WeCom provider is loaded through the provider contract
- **THEN** its descriptor exposes only the structured rendering, callback, and mutable-update features that the current WeCom implementation actually supports
- **AND** shared delivery code falls back explicitly instead of claiming richer parity for unsupported WeCom paths

#### Scenario: Feishu provider opts into native richer surfaces
- **WHEN** the Feishu provider advertises native card lifecycle and delayed update extensions
- **THEN** the Bridge may use those provider-native paths for Feishu notifications and interactions
- **AND** the same request falls back cleanly on providers that do not advertise those extensions

### Requirement: IM Bridge platform providers SHALL declare rendering profile metadata
In addition to transport and callback capabilities, each IM Bridge platform provider SHALL declare rendering profile metadata that describes provider-specific formatting modes, structured rendering preferences, message length constraints, mutable update rules, and any optional builder surfaces needed for richer provider-native payload construction. Shared Bridge paths MUST consume that metadata through the provider contract instead of maintaining a separate platform-name switch for rendering behavior.

#### Scenario: Minimal provider declares only plain-text rendering
- **WHEN** a provider supports only base send, reply, and plain-text delivery
- **THEN** its descriptor declares a minimal rendering profile with plain-text constraints only
- **AND** the Bridge continues to route shared command and notification flows through that provider without requiring richer builders

#### Scenario: Feishu provider declares richer rendering builders
- **WHEN** the Feishu provider is loaded through the provider contract
- **THEN** its descriptor declares rendering metadata for text, `lark_md`, JSON-card, template-card, and delayed-update-aware native builders
- **AND** shared delivery code can discover those richer surfaces without a Feishu-only branch outside the provider layer

#### Scenario: Telegram provider declares formatted-text constraints
- **WHEN** the Telegram provider is loaded through the provider contract
- **THEN** its descriptor declares the formatted-text and mutable-update constraints required for safe Markdown-aware delivery
- **AND** shared delivery code can choose between Markdown-capable rendering and plain-text fallback through the provider contract alone
