## ADDED Requirements

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

