## ADDED Requirements

### Requirement: Runtime catalog publishes CLI launch-contract metadata
The TypeScript bridge SHALL publish per-runtime launch-contract metadata for CLI-backed runtimes in `/bridge/runtimes`, including prompt transport mode, machine-readable output mode, supported approval controls, additional-directory support, auth or config prerequisite state, and any deprecation or sunset metadata needed for upstream gating.

#### Scenario: Catalog includes launch-contract summary for cursor
- **WHEN** an upstream consumer requests the Bridge runtime catalog
- **THEN** the `cursor` entry SHALL include launch-contract metadata that distinguishes documented headless prompt or output controls from unsupported controls
- **THEN** upstream consumers SHALL NOT need to infer Cursor CLI behavior from the runtime key alone

#### Scenario: Catalog includes iFlow sunset guidance
- **WHEN** the catalog includes `iflow`
- **THEN** the entry SHALL include lifecycle metadata such as deprecation state, sunset date, and migration guidance
- **THEN** settings and operator surfaces SHALL be able to warn or disable the runtime before launch

### Requirement: Execute preflight and catalog share CLI runtime truth
For CLI-backed runtimes, execute preflight SHALL reuse the same launch-contract and lifecycle rules that `/bridge/runtimes` publishes. If a request asks for a prompt transport, output mode, approval override, additional directory, environment override, or runtime that the selected CLI backend cannot truthfully honor under its current documented contract or lifecycle state, the bridge MUST reject the request before acquiring runtime state and MUST use the same reason codes surfaced in the catalog.

#### Scenario: Qoder request asks for an unsupported per-run approval override
- **WHEN** a caller requests a `qoder` launch with a per-run approval control that Qoder's documented CLI contract does not expose
- **THEN** execute preflight SHALL reject the request before runtime acquisition
- **THEN** the returned reason SHALL match the catalog's CLI launch-contract diagnostics

#### Scenario: Sunset runtime is requested for execution
- **WHEN** a caller requests `iflow` after its published sunset window or while its launch contract is flagged unavailable
- **THEN** the bridge SHALL reject the request before starting execution
- **THEN** no misleading running-state events SHALL be emitted for that runtime
