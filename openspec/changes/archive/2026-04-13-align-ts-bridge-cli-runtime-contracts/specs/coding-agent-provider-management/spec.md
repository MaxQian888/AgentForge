## ADDED Requirements

### Requirement: Project catalog preserves CLI runtime lifecycle and contract diagnostics
The system SHALL include Bridge-published CLI launch-contract and lifecycle metadata inside the project-scoped coding-agent catalog returned to settings and launch surfaces. For CLI-backed runtimes, the project catalog MUST preserve degraded or unavailable reason codes, install or auth guidance, and any sunset or migration metadata instead of flattening them into a generic availability flag.

#### Scenario: Settings render a degraded CLI runtime with migration guidance
- **WHEN** project settings load a catalog containing degraded `iflow`
- **THEN** the response SHALL include the Bridge-provided sunset and migration guidance
- **THEN** the UI SHALL be able to render a warning state instead of treating the runtime as a normal selectable default

#### Scenario: Catalog preserves runtime-specific contract hints
- **WHEN** the catalog includes `cursor` or `qoder`
- **THEN** the project-scoped catalog SHALL preserve their runtime-specific launch-contract diagnostics and capability hints
- **THEN** frontend selectors SHALL NOT invent support assumptions beyond the Bridge contract

### Requirement: Launch surfaces block deprecated or contract-invalid CLI selections
Settings, single-agent launch flows, and team launch flows SHALL prevent submission of a CLI runtime selection when the project catalog marks the selected runtime as unavailable because of launch-contract mismatch, missing official auth or config prerequisites, or published sunset state. When the runtime is degraded but still launchable, the surfaces SHALL present the same warning reason before submission and preserve that resolved runtime identity if the user proceeds.

#### Scenario: Unavailable CLI runtime cannot be submitted
- **WHEN** a user selects `qoder` or `iflow` and the project catalog marks it unavailable
- **THEN** the launch surface SHALL block submission and show the runtime-specific diagnostic
- **THEN** the backend SHALL NOT receive a launch tuple for that runtime

#### Scenario: Degraded runtime warning is shown consistently
- **WHEN** a runtime remains launchable but the catalog marks it degraded because of deprecation or headless-contract caveats
- **THEN** settings and launch surfaces SHALL show the same warning reason before launch
- **THEN** any launched run SHALL still preserve the selected runtime identity rather than silently rewriting it

### Requirement: Resolved defaults skip unavailable CLI runtimes
The system SHALL resolve project-level coding-agent defaults only from runtimes that the current Bridge catalog marks launchable. If saved defaults point to a CLI runtime that is unavailable because of missing contract prerequisites or sunset state, the backend MUST surface a diagnostic and fall back to the next supported default instead of auto-launching the stale runtime.

#### Scenario: Stale iFlow default is replaced at read time
- **WHEN** project settings still persist `iflow` as the default runtime after `iflow` is marked unavailable by sunset rules
- **THEN** the returned project catalog SHALL include a diagnostic about the stale selection
- **THEN** new launch surfaces SHALL use the bridge default or another supported fallback instead of blindly using `iflow`
