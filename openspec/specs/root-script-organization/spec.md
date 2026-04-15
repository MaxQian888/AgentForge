# root-script-organization Specification

## Purpose
Define the canonical organization, caller migration contract, and verification rules for root repository automation scripts so AgentForge can group scripts by functional domain without leaving stale supported entrypoints behind.
## Requirements
### Requirement: Repository groups root automation scripts by functional domain
The repository SHALL organize the root `scripts/` directory into canonical function-oriented groups instead of keeping unrelated automation entrypoints in one flat folder. Scripts, wrappers, fixtures, and shared helpers that primarily serve the same workflow family MUST live under the same domain-owned script area.

#### Scenario: Maintainer locates a script by workflow family
- **WHEN** a maintainer inspects the root `scripts/` directory to find a build, development, plugin, governance, updater, or audit workflow
- **THEN** the canonical script path is grouped with other files from the same functional domain
- **AND** the maintainer does not need to infer ownership from a flat list of unrelated root-level script files

### Requirement: Repository-supported callers use canonical script locations
The repository SHALL update every repo-supported caller of a moved root script to the canonical path for that script family. Root package commands, plugin-local package commands, GitHub workflows, repository docs, tests, fixtures, and intra-script imports MUST NOT rely on removed legacy flat paths after the migration completes.

#### Scenario: Supported package and workflow entrypoints stay valid after migration
- **WHEN** a developer runs a supported package command or CI workflow step that invokes a reorganized root script
- **THEN** that caller resolves the script from its canonical post-migration location
- **AND** the workflow does not require manual path edits to recover from the reorganization

#### Scenario: Plugin and documentation call sites stay aligned with canonical paths
- **WHEN** a maintainer follows a repo-documented `node ...` invocation or a plugin-local package command that targets a reorganized root script
- **THEN** the referenced path matches the canonical script location for that workflow family
- **AND** the repository no longer contains a repo-supported caller that still points at a removed flat script path

### Requirement: Migration verification catches stale root script path references
The repository SHALL provide deterministic verification for the root script reorganization that detects unresolved repo-owned references to removed script paths and broken script-family wiring. The verification MUST identify which caller or validation stage still references an outdated location when the migration is incomplete.

#### Scenario: Verification fails on stale legacy path
- **WHEN** a repo-owned package command, workflow, doc reference, test, or script import still references a removed legacy root script path
- **THEN** the migration verification exits non-zero
- **AND** the failure identifies the stale caller or path family that still needs to be updated

#### Scenario: Verification passes when migration is fully synchronized
- **WHEN** every supported caller and moved script family has been updated to the canonical domain-based layout
- **THEN** the migration verification succeeds without reporting stale legacy path references
- **AND** focused validation for the moved script families still resolves the reorganized scripts correctly
