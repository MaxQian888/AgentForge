## ADDED Requirements

### Requirement: Operators can install plugins through truthful source-aware flows
The system SHALL let operators initiate plugin installation only through source flows the current platform contract supports. The plugin management panel SHALL provide explicit local-path install and catalog-entry install actions, and MUST keep browse-only entries non-installable when no supported install source is available.

#### Scenario: Operator installs a plugin from a local path
- **WHEN** the operator provides a filesystem path to a plugin directory or manifest file
- **THEN** the panel submits an explicit local install request instead of relying on discovery side effects
- **THEN** the installed plugin appears in the installed registry section only after the install request succeeds

#### Scenario: Operator installs a built-in or catalog entry explicitly
- **WHEN** the operator chooses an installable built-in or catalog entry from the availability sections
- **THEN** the panel triggers an explicit install action for that entry
- **THEN** the entry does not become an installed plugin record until that install action completes successfully

#### Scenario: Browse-only marketplace entry remains non-installable
- **WHEN** an availability entry lacks a real install source supported by the current platform contract
- **THEN** the panel shows the entry as browse-only or coming soon
- **THEN** the panel MUST NOT render an install action that implies unsupported remote installation

### Requirement: Operators can inspect trust, release, and runtime diagnostics for installed plugins
The plugin management panel SHALL surface operator-facing diagnostics for the selected installed plugin, including trust state, approval state, release metadata, runtime metadata, and recent operational history when available. For ToolPlugin records with MCP runtime metadata, the panel SHALL expose transport details, capability counts, latest interaction summary, and on-demand MCP refresh and inspection actions. For WorkflowPlugin records, the panel SHALL expose recent workflow run history through the existing control-plane contracts.

#### Scenario: Operator reviews trust and release metadata
- **WHEN** the operator opens the detail view for an installed plugin that carries trust, approval, digest, signature, or release metadata
- **THEN** the panel shows those operator-visible source and release fields alongside the plugin's runtime details
- **THEN** the panel distinguishes verified, pending, approved, rejected, or untrusted states without requiring raw JSON inspection

#### Scenario: Operator inspects MCP diagnostics for a tool plugin
- **WHEN** the operator selects an active ToolPlugin that reports MCP runtime metadata
- **THEN** the panel shows the current MCP transport, capability counts, and latest interaction summary
- **THEN** the panel allows the operator to trigger supported MCP refresh or inspection flows through the existing Go control-plane APIs

#### Scenario: Operator inspects recent audit events and workflow runs
- **WHEN** the operator opens diagnostics for an installed plugin with event audit history or workflow execution history
- **THEN** the panel shows recent plugin events from the audit stream when available
- **THEN** the panel shows recent workflow runs for WorkflowPlugin records without requiring a separate route change

### Requirement: Operators can manage the full supported plugin lifecycle from the panel
The plugin management panel SHALL expose every lifecycle action already supported by the current platform contract, including enable, disable, activate, deactivate, restart, health, update, and uninstall, but only when the selected plugin's source, runtime, lifecycle state, and trust state permit that action. When an action is unavailable, the panel SHALL explain the blocking condition instead of silently omitting all context.

#### Scenario: Active plugin can be deactivated without uninstalling it
- **WHEN** the operator selects a plugin whose lifecycle state is `active` and the platform supports deactivation for that record
- **THEN** the panel offers a deactivate action distinct from disable and uninstall
- **THEN** successful deactivation returns the plugin to an enabled-but-not-active state in the installed registry view

#### Scenario: Untrusted external plugin shows activation as blocked
- **WHEN** the operator selects an external plugin whose trust or approval state still blocks activation
- **THEN** the panel does not present activation as immediately available
- **THEN** the panel explains that the plugin must satisfy the required trust or approval path before activation can proceed

#### Scenario: Update action reflects real source metadata
- **WHEN** the operator selects a plugin that lacks a real update source or available release metadata
- **THEN** the panel does not present update as a ready action
- **THEN** the panel explains that no supported update source is currently available for that plugin

