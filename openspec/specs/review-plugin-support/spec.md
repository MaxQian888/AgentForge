# review-plugin-support Specification

## Purpose
Define how review plugins are declared, selected, and normalized so external review extensions can participate safely in the shared Layer 2 deep-review pipeline.
## Requirements
### Requirement: Review plugin manifests define triggers, rules, and runtime entrypoints
The system SHALL accept `ReviewPlugin` manifests that declare the review runtime entrypoint, trigger conditions, file-pattern matching rules, and normalized output contract required by the Layer 2 review pipeline. Invalid review manifests MUST be rejected before enablement.

#### Scenario: Valid MCP review plugin is registered
- **WHEN** a review plugin manifest declares `runtime: mcp`, a valid transport or endpoint, trigger metadata, and a supported output format
- **THEN** the platform registers the plugin and records it as an enabled-or-disabled review extension candidate

#### Scenario: Review plugin with invalid trigger contract is rejected
- **WHEN** a review plugin manifest omits required trigger fields or declares an unsupported output format
- **THEN** the platform rejects the plugin before enablement and returns a validation error describing the contract violation

### Requirement: Enabled review plugins are selected for matching deep-review runs
The system SHALL evaluate enabled review plugins for each Layer 2 review trigger and include only the plugins whose trigger conditions, file-pattern filters, and repository scope match the review input alongside the built-in review dimensions.

#### Scenario: Matching review plugin is added to the execution plan
- **WHEN** a pull request update matches a review plugin's configured trigger event and file patterns
- **THEN** the deep-review execution plan includes that plugin together with the built-in review dimensions

#### Scenario: Non-matching review plugin is skipped
- **WHEN** a review run does not satisfy a plugin's trigger conditions or file-pattern filters
- **THEN** the platform excludes that plugin from the current execution plan without disabling it globally

### Requirement: Review plugin results are normalized into the shared finding model
The system SHALL normalize findings emitted by review plugins into the same structured finding model used by the deep-review pipeline, including plugin identity, severity, category, file context, message, and suggested remediation metadata where available.

#### Scenario: Custom review plugin findings are aggregated with built-in findings
- **WHEN** a custom review plugin returns findings for a review run
- **THEN** the deep-review pipeline stores those findings in the aggregated result with plugin provenance preserved

#### Scenario: Review plugin failure is preserved without discarding successful dimensions
- **WHEN** one review plugin fails or times out while other built-in or custom review dimensions complete
- **THEN** the aggregated review result marks that plugin execution as failed explicitly and preserves the successful findings from the remaining dimensions

### Requirement: Review execution metadata is surfaced in the Web UI
The system SHALL expose `executionMetadata` from the review backend DTO in the frontend `ReviewRecord` TypeScript type so that plugin provenance, trigger event, changed file list, and per-plugin result breakdown are available for display. The review detail panel SHALL show a structured execution metadata section when `executionMetadata` is present.

#### Scenario: Review detail panel shows execution metadata for a completed review
- **WHEN** a user opens the detail panel for a review that includes `executionMetadata`
- **THEN** the panel displays the trigger event, changed file count, and a per-plugin result breakdown (plugin name, status, finding count)
- **THEN** the display does not show empty sections when `executionMetadata` is absent

#### Scenario: Frontend ReviewRecord type includes executionMetadata
- **WHEN** the backend returns a review response containing `executionMetadata`
- **THEN** the TypeScript review store deserializes and stores the metadata without type errors
- **THEN** components can access `review.executionMetadata` without casting to `any`

### Requirement: Findings table surfaces plugin provenance and sources
The system SHALL display the originating plugin or built-in dimension name alongside each finding in the review findings table. When a finding includes `sources` metadata (plugin identity, dimension name), the table SHALL show that context in a dedicated column or expandable row.

#### Scenario: Findings table shows plugin provenance column
- **WHEN** a review has findings that include plugin or dimension source metadata
- **THEN** the findings table renders a provenance column or badge showing the source plugin/dimension name for each finding

#### Scenario: Findings without sources are still displayed without errors
- **WHEN** a review has findings that do not include source metadata
- **THEN** the findings table renders those findings without a provenance column or with an empty provenance indicator
- **THEN** no runtime error or blank column heading appears

