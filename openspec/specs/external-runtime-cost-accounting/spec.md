# external-runtime-cost-accounting Specification

## Purpose
TBD - created by archiving change improve-external-agent-cost-accounting. Update Purpose after archive.
## Requirements
### Requirement: External runtime accounting prefers native totals before pricing fallback
The system SHALL account for external runtime spend using one truthful precedence order for each run: authoritative native runtime totals first, official provider pricing applied to cumulative usage totals second, and explicit `unpriced` or `plan_included` coverage states last when the active billing surface cannot be truthfully expressed as billable USD.

#### Scenario: Claude Code authoritative result total is preserved
- **WHEN** a Claude Code run emits a native authoritative total cost for the current query together with cumulative usage data
- **THEN** the bridge SHALL record that run using the native total instead of recomputing USD from a local default price table
- **THEN** the persisted accounting metadata SHALL identify the run as authoritative and preserve the native source description

#### Scenario: Codex API-key usage falls back to official API pricing
- **WHEN** a Codex run provides cumulative usage totals but does not provide a trustworthy native USD total
- **THEN** the bridge SHALL calculate USD using the official OpenAI model pricing for the resolved model and billing mode
- **THEN** the persisted accounting metadata SHALL mark the run as estimated from official API pricing rather than authoritative native billing

#### Scenario: Subscription or credits-only runtime usage is not fabricated into USD
- **WHEN** an external runtime is executing under a billing surface that exposes usage limits, credits, or other non-USD quotas without a truthful per-run USD mapping
- **THEN** the system SHALL mark the run as `plan_included` or `unpriced`
- **THEN** the system SHALL NOT fabricate a billable USD amount from an unrelated model fallback

### Requirement: External runtime accounting uses an official pricing catalog with model alias normalization
The system SHALL resolve provider and model pricing through one checked-in catalog sourced from current official Anthropic and OpenAI pricing contracts. The catalog MUST normalize repository-supported model aliases and pricing dimensions such as cached input, cache read, or cache creation instead of silently falling back to unrelated default models.

#### Scenario: Claude model alias resolves to the current Anthropic pricing tier
- **WHEN** a run resolves to a repository-supported Claude alias such as `claude-sonnet-4-5` or `claude-haiku-4-5`
- **THEN** the pricing catalog SHALL map that alias to the correct Anthropic pricing entry for that model family
- **THEN** the bridge SHALL NOT fall back to an unrelated `claude-sonnet-4` default solely because the exact alias was not hardcoded before

#### Scenario: OpenAI cached input pricing is distinct from regular input pricing
- **WHEN** a Codex or OpenAI-backed run reports cached input token totals for a model whose official pricing distinguishes cached input from regular input
- **THEN** the pricing catalog SHALL apply the cached-input rate for those tokens
- **THEN** the resulting estimated cost SHALL remain consistent with the official model pricing contract for that model

#### Scenario: Unsupported pricing alias degrades to explicit unpriced coverage
- **WHEN** a run's resolved model cannot be matched to a supported pricing alias and no authoritative native total is available
- **THEN** the system SHALL record the run as unpriced instead of applying an unrelated fallback model's pricing
- **THEN** coverage metadata SHALL explain that the run was excluded from truthful USD attribution because pricing was unavailable

### Requirement: Bridge and Go share one cumulative runtime accounting snapshot contract
The bridge SHALL emit external runtime cost updates as the latest cumulative accounting snapshot for that run, including cumulative usage totals, cumulative cost totals, and accounting provenance metadata. Go SHALL persist the latest snapshot for the run and recalculate task or project spend from persisted run totals instead of treating repeated updates as additive deltas.

#### Scenario: Repeated periodic updates do not double-count the same run
- **WHEN** the bridge emits multiple cost updates for the same in-flight run as its totals increase over time
- **THEN** each update SHALL represent the latest cumulative accounting snapshot for that run
- **THEN** Go SHALL persist the latest run totals and recompute aggregate spend from persisted runs without double-counting earlier snapshots

#### Scenario: Token totals reflect whole-run accounting instead of per-step deltas
- **WHEN** a runtime produces multiple native steps or turns before completion
- **THEN** the accounting snapshot persisted for that run SHALL expose cumulative input, output, and cache token totals for the whole run seen so far
- **THEN** downstream cost queries SHALL NOT be forced to infer whole-run totals from partial per-step token values

#### Scenario: Multi-model native breakdown is preserved for later aggregation
- **WHEN** a runtime exposes authoritative per-model usage or cost components for a single run
- **THEN** the bridge SHALL persist that component breakdown in the run's accounting metadata alongside the run-level totals
- **THEN** downstream summaries SHALL be able to distinguish the run's resolved launch model from the actual priced model mix when the runtime used more than one model

