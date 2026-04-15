## ADDED Requirements

### Requirement: Governed skill inventory and actions are exposed through stable operator APIs
The repository SHALL expose stable operator-facing APIs for governed internal skills using the same registry, provenance, bundle, and mirror truth that powers internal skill verification. These APIs MUST provide machine-readable inventory, detail, verification, and sync responses instead of requiring the operator UI to parse CLI output or filesystem state directly.

#### Scenario: Inventory API reflects registry-declared skill governance truth
- **WHEN** the operator-facing skills inventory API is requested
- **THEN** it returns every registry-declared skill with its family, verification profile, canonical root, source type, docs reference, and any declared lock key or mirror targets
- **THEN** built-in bundle alignment and downstream consumer-surface metadata are reported from governed sources instead of inferred from frontend-only heuristics

#### Scenario: Verification API reports per-skill governance diagnostics
- **WHEN** the operator-facing verification API is triggered for all skills or a supported family subset
- **THEN** it evaluates governed skills against the same registry, profile, provenance, bundle, and mirror rules used by internal skill verification
- **THEN** the response reports per-skill issues, status, and affected targets instead of collapsing the result to a single boolean outcome

#### Scenario: Mirror sync API remains bounded to declared workflow mirrors
- **WHEN** the operator-facing mirror sync API is triggered
- **THEN** it only updates registry-declared workflow-mirror targets from their canonical source
- **THEN** the response identifies which targets changed, which remained unchanged, and which actions were blocked because the selected skill family does not support mirror sync
