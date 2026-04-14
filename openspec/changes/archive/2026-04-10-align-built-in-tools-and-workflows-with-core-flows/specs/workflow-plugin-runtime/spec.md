## ADDED Requirements

### Requirement: Built-in workflow starters declare executable trigger profiles truthfully
Official built-in workflow starters SHALL declare structured trigger profiles that map to currently supported execution entrypoints such as manual start or supported task-driven activation. A built-in starter MUST NOT be presented as executable for a trigger profile unless the current workflow runtime and control-plane seam can actually start it through that profile.

#### Scenario: Starter declares a supported manual trigger profile
- **WHEN** an official built-in workflow starter declares a manual trigger profile supported by the current workflow runtime
- **THEN** the platform treats that trigger profile as executable for activation and start flows

#### Scenario: Unsupported trigger profile remains non-executable
- **WHEN** an official built-in workflow starter declares a trigger profile that the current workflow runtime or control plane cannot execute yet
- **THEN** the platform reports that trigger profile as unavailable or non-executable instead of implying the starter can already run that way

### Requirement: Built-in workflow starter library keeps dependency diagnostics after installation
The workflow plugin runtime SHALL preserve starter-library dependency diagnostics for installed official built-in workflow starters, including current role binding status and any declared service dependency gaps needed for execution. Activation and execution paths MUST fail before the first step begins when a built-in starter's current dependency state is no longer satisfied.

#### Scenario: Installed built-in starter shows dependency-ready state
- **WHEN** an installed official built-in workflow starter still resolves all declared roles and required services
- **THEN** activation and execution proceed through the normal workflow runtime path with dependency-ready diagnostics

#### Scenario: Installed built-in starter fails before execution on missing dependency
- **WHEN** an installed official built-in workflow starter no longer satisfies one of its declared role or service dependencies
- **THEN** the runtime refuses activation or execution before any workflow step begins
- **THEN** the returned error and diagnostics identify the missing dependency explicitly
