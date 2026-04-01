# plugin-management-panel Specification

## Purpose
Define the operator-facing plugin management console for browsing installed, built-in, and marketplace plugin entries with truthful lifecycle controls and runtime details.
## Requirements
### Requirement: Operators can browse installed plugins with operational filters and details
The system SHALL provide a plugin management panel that lists installed plugins and allows operators to filter by plugin kind, lifecycle state, runtime host, source type, and free-text search. The panel SHALL allow an operator to open a detail view for any listed plugin and inspect its declared runtime, declared permissions, resolved source information, runtime metadata, restart count, last health timestamp, and last error summary.

#### Scenario: Filter the installed plugin list
- **WHEN** the operator applies one or more filters such as `ToolPlugin`, `active`, `ts-bridge`, or a search term
- **THEN** the panel shows only installed plugins matching the selected criteria
- **THEN** clearing the filters restores the full installed list without reloading the page

#### Scenario: Inspect plugin operational details
- **WHEN** the operator opens the detail view for an installed plugin
- **THEN** the panel shows the plugin's runtime declaration, runtime host, permissions, resolved source path, runtime metadata, restart count, last health timestamp, and last error state when present

### Requirement: The panel distinguishes built-in, local, and marketplace plugin sources
The system SHALL present available plugins by source channel instead of flattening every source into one list. The plugin management panel SHALL separately surface built-in plugin discoveries, installed plugins, local catalog entries, remote registry marketplace entries, and installed plugins that originated from the standalone marketplace workspace, and SHALL indicate whether each available entry is currently installable with the platform's real capabilities. Remote registry entries MUST expose registry availability or source-health state so operators can distinguish a reachable remote marketplace from a disabled or failing one. Installed plugins that originated from the standalone marketplace workspace MUST preserve marketplace item identity, selected version, and a navigation path back to the originating marketplace surface instead of collapsing into anonymous local provenance.

#### Scenario: Built-in plugin already installed
- **WHEN** a built-in plugin is already present in the installed plugin registry
- **THEN** the built-in availability section SHALL not present it as a duplicate install candidate
- **THEN** the installed section SHALL remain the authoritative place to manage that plugin

#### Scenario: Standalone marketplace-sourced plugin remains identifiable after install
- **WHEN** an installed plugin originated from the standalone marketplace workspace
- **THEN** the installed section and detail surface SHALL show the marketplace provenance, originating item identity, and selected version
- **THEN** the operator SHALL be able to navigate from the plugin detail surface back to the originating marketplace item or management workspace

#### Scenario: Marketplace entry is browse-only
- **WHEN** a marketplace plugin entry lacks a real install source supported by the current platform contract
- **THEN** the marketplace section SHALL show the entry as browse-only or coming soon
- **THEN** the panel SHALL not present an enabled install action that implies unsupported remote installation

#### Scenario: Remote registry source is unavailable
- **WHEN** the configured remote registry is disabled, unreachable, or returns an invalid response
- **THEN** the remote marketplace section SHALL show the registry source as unavailable with a stable reason
- **THEN** the rest of the plugin management panel continues to function for installed and local plugin sources

### Requirement: Lifecycle actions are gated by executable capability and current state
The system SHALL render lifecycle actions in the plugin panel according to the selected plugin's kind, runtime, and current lifecycle state. The panel SHALL expose only actions that are valid for the current record and SHALL explain unavailable actions for disabled, non-executable, or unsupported runtime combinations.

#### Scenario: Disabled plugin cannot be activated directly
- **WHEN** the operator selects a plugin whose registry state is `disabled`
- **THEN** the plugin panel SHALL not present activation as an immediately available action
- **THEN** the panel SHALL require the plugin to be re-enabled before activation can be attempted

#### Scenario: Non-executable plugin shows why runtime actions are unavailable
- **WHEN** the operator views a plugin that is not executable in the current phase, such as a declarative-only role plugin
- **THEN** the panel SHALL suppress restart and health actions that require a live runtime host
- **THEN** the panel SHALL explain that the plugin does not currently run through an executable plugin runtime

### Requirement: Operators can install plugins through truthful source-aware flows
The system SHALL let operators initiate plugin installation only through source flows the current platform contract supports. The plugin management panel SHALL provide explicit local-path install, catalog-entry install, and supported remote-registry install actions, and MUST keep browse-only or unavailable entries non-installable when no supported install source is available.

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

#### Scenario: Operator installs a plugin from the remote registry marketplace
- **WHEN** the operator chooses an installable remote marketplace entry from the remote registry section
- **THEN** the panel triggers the supported remote install action through the Go control plane instead of direct browser download
- **THEN** the entry only moves into the installed registry section after the remote install and verification flow succeeds

### Requirement: Remote registry install and availability states stay operator-visible in the panel
The plugin management panel SHALL surface remote registry availability, in-progress install state, install failure categories, and resulting source provenance for remote marketplace entries. Remote-source failures MUST remain scoped to the remote marketplace section and MUST NOT collapse installed-plugin diagnostics or local install actions.

#### Scenario: Remote install failure stays visible on the selected marketplace entry
- **WHEN** a remote marketplace install fails because of registry reachability, download, or trust verification
- **THEN** the panel shows the corresponding failure reason on the affected remote entry or detail surface
- **THEN** the operator can retry or inspect the failure without losing the rest of the plugin console state

#### Scenario: Successful remote install updates source provenance in details
- **WHEN** a remote marketplace entry installs successfully and becomes an installed plugin record
- **THEN** the plugin detail view shows the remote registry source provenance and selected version alongside trust and runtime metadata
- **THEN** the remote marketplace section reflects that the entry is already installed

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

### Requirement: The panel surfaces built-in readiness and setup guidance
The plugin management panel SHALL display evaluated readiness state, blocking reasons, maintained docs references, and next-step setup guidance for official built-in plugins in both the built-in availability section and installed-plugin diagnostics when applicable. The panel MUST distinguish "installable" from "ready to activate" so operators can understand whether a built-in can be added now, used now, or first requires setup work.

#### Scenario: Built-in availability card shows setup guidance
- **WHEN** the built-in availability section includes an official built-in plugin whose readiness is not `ready`
- **THEN** the card shows a readiness indicator, the blocking reason, and the next setup step for that built-in
- **THEN** the card does not imply that the plugin is immediately runnable if activation would still fail

#### Scenario: Installed built-in keeps readiness guidance in details
- **WHEN** an operator selects an installed official built-in plugin that is still blocked by missing prerequisite or configuration
- **THEN** the plugin detail surface continues to show the readiness state and setup guidance for that installed plugin
- **THEN** lifecycle controls explain the blocking condition instead of failing without context

#### Scenario: Host-unsupported built-in is rendered as blocked
- **WHEN** an official built-in plugin is unsupported on the current host
- **THEN** the panel renders that built-in as blocked or browse-only with an explicit reason
- **THEN** the panel does not present a misleading ready-to-run state for that entry

### Requirement: Operators can inspect plugin-role dependency health from the plugin panel
The plugin management panel SHALL surface role dependency health for any selected plugin whose current contract depends on roles or is consumed by role-scoped tool references. WorkflowPlugin records MUST show each referenced role binding, whether it currently resolves from the authoritative role registry, and whether any missing or stale role binding is blocking activation or execution. ToolPlugin or MCP-backed plugin records that are referenced by current roles through role-scoped tool or MCP identifiers MUST show the referencing roles and their current dependency state instead of hiding that relationship behind raw ids.

#### Scenario: Workflow plugin shows a stale role binding
- **WHEN** the operator opens the detail view for an installed workflow plugin whose manifest references a role id that no longer resolves from the current role registry
- **THEN** the panel shows that role binding as missing or stale instead of rendering only the raw role id list
- **THEN** the panel explains that activation or execution is currently blocked by that dependency gap and provides a path back to the roles workspace

#### Scenario: Tool plugin shows referencing roles
- **WHEN** the operator opens the detail view for a tool or MCP-backed plugin whose plugin id is referenced by one or more current roles through role-scoped tool configuration
- **THEN** the panel shows the referencing roles and the current dependency status for each reference
- **THEN** the operator can navigate from that plugin detail context to the affected roles without manually searching raw ids

