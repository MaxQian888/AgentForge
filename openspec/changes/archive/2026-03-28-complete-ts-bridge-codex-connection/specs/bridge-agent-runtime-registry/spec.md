## ADDED Requirements

### Requirement: Codex resolves through a dedicated bridge-owned adapter
The TypeScript bridge SHALL resolve `runtime=codex` through a dedicated Codex adapter owned by the bridge, and it MUST NOT treat the bare external `codex` executable as if it natively implemented the bridge's command-runtime JSONL contract.

#### Scenario: Execute request targets Codex
- **WHEN** the bridge receives an execute request whose resolved runtime is `codex`
- **THEN** the runtime registry SHALL return the dedicated Codex adapter for execution
- **THEN** the bridge SHALL keep using the same canonical `/bridge/execute` surface instead of requiring Go to call a Codex-specific route

#### Scenario: Legacy raw command assumption is not treated as readiness
- **WHEN** the registry can discover a `codex` executable but the bridge-owned Codex connector contract is not configured or supported
- **THEN** the registry SHALL NOT mark `codex` as ready solely because the command exists
- **THEN** the returned diagnostics SHALL identify the missing connector requirement before execution starts

### Requirement: Codex diagnostics validate connector and authentication prerequisites
The TypeScript bridge SHALL evaluate Codex readiness against the full connector contract, including supported authentication state and any required local prerequisites, and it MUST surface actionable blocking diagnostics without acquiring execution state.

#### Scenario: Codex authentication is unavailable during diagnostics
- **WHEN** the registry evaluates `codex` readiness and no supported Codex authentication source is configured
- **THEN** the diagnostics result SHALL mark `codex` as unavailable
- **THEN** the reported reason SHALL identify the missing authentication requirement before any execute request is attempted

#### Scenario: Codex connector prerequisites are satisfied
- **WHEN** the registry evaluates `codex` readiness and the dedicated connector plus its required prerequisites are available
- **THEN** the runtime catalog SHALL report `codex` as available with its canonical provider and model metadata
- **THEN** upstream consumers SHALL not need extra Codex-specific readiness checks outside the bridge catalog
