## ADDED Requirements

### Requirement: Skills workspace renders the governed internal skill inventory truthfully
The system SHALL provide a standalone `/skills` operator workspace that lists every registry-declared internal skill across the `built-in-runtime`, `repo-assistant`, and `workflow-mirror` families. The workspace MUST derive its inventory from the governed skill registry and related provenance sources instead of inferring membership from ad hoc directory scans or UI-local hardcoded lists.

#### Scenario: Operator opens the governed skills inventory
- **WHEN** the operator opens `/skills`
- **THEN** the workspace loads every registry-declared internal skill with its id, family, provenance, canonical root, preview availability, and current health status
- **THEN** the list allows the operator to distinguish built-in runtime skills, repo-assistant skills, and workflow mirrors without opening raw YAML files

#### Scenario: Inventory reflects governed provenance and downstream surfaces
- **WHEN** a listed skill is backed by `skills/builtin-bundle.yaml`, `skills-lock.json`, mirror targets, or a downstream consumer surface such as role authoring or marketplace
- **THEN** the workspace shows the relevant bundle, lock, mirror, or consumer summary for that skill
- **THEN** the workspace does not invent downstream availability that is absent from the governed backend response

### Requirement: Skills workspace detail explains package content and governance state
The `/skills` workspace SHALL provide a detail surface for the selected skill that combines structured package preview with governance diagnostics and downstream handoff targets. The detail view MUST keep package content, provenance, and health state aligned with the same backend inventory truth.

#### Scenario: Built-in runtime skill detail shows package preview and consumer handoff
- **WHEN** the operator selects a built-in runtime skill that is part of the official bundle
- **THEN** the detail view shows the skill's Markdown body, frontmatter YAML, supported agent config YAML, dependency and tool summary, and bundle alignment state
- **THEN** the detail view offers the supported downstream handoff targets such as role authoring or marketplace without pretending the skill needs installation when it is already locally available

#### Scenario: Workflow mirror or repo-assistant detail shows governance diagnostics
- **WHEN** the operator selects a workflow-mirror or repo-assistant skill
- **THEN** the detail view shows the canonical root, source type, docs reference, mirror targets or lock provenance, and any drift or validation diagnostics returned by the backend
- **THEN** the operator can understand why the skill is healthy, warning-level, or blocked without switching to a CLI transcript

### Requirement: Skills workspace actions remain truthful and explicitly bounded
The `/skills` workspace SHALL expose only the supported management actions for the selected skill or family and MUST report blocked or unsupported actions explicitly. Action responses MUST return machine-readable diagnostics that can be reflected back into the workspace state.

#### Scenario: Operator runs verification from the workspace
- **WHEN** the operator triggers internal or built-in skill verification from `/skills`
- **THEN** the workspace receives per-skill diagnostics and updates each affected skill's health state from the backend response
- **THEN** the UI does not collapse verification to a single generic success or failure toast without showing which skills passed or failed

#### Scenario: Operator syncs workflow mirrors
- **WHEN** the operator triggers mirror sync for registry-declared workflow-mirror skills
- **THEN** the workspace shows which mirror targets were updated, which remained unchanged, and whether any drift remains after the sync
- **THEN** the action remains unavailable with an explicit reason for skills that are not eligible for workflow mirror sync

#### Scenario: Unsupported management actions stay blocked
- **WHEN** the selected skill does not support a requested action such as mirror sync for a built-in skill or upstream refresh for a repo-assistant skill
- **THEN** the workspace shows the action as blocked or unsupported with a stable reason
- **THEN** the UI does not render that blocked state as a successful no-op or hide the unsupported action context entirely
