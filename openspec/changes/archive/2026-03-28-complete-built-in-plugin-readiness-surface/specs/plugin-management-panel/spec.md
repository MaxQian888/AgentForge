## ADDED Requirements

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
