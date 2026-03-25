## ADDED Requirements

### Requirement: Desktop release builds produce updater-compatible signed artifacts
The desktop release pipeline SHALL generate updater-compatible signed artifacts for every release target that AgentForge advertises to desktop clients. When desktop auto-update is enabled for a releasable build, the build configuration MUST enable updater artifact generation and MUST treat missing signing inputs or missing public-key configuration as explicit release blockers instead of publishing incomplete updater outputs.

#### Scenario: Release build generates signed updater artifacts
- **WHEN** a desktop release build runs with the required signing inputs and updater configuration
- **THEN** the build produces the normal desktop bundles together with updater-compatible signatures or update bundles for each configured platform target

#### Scenario: Release build is missing updater prerequisites
- **WHEN** a desktop release build is triggered without the signing material or public updater configuration required for client verification
- **THEN** the release workflow fails with an explicit updater-configuration error before publishing a partial desktop update release

### Requirement: Desktop releases publish a static updater manifest from authoritative artifacts
The release workflow SHALL publish a static updater manifest, or an equivalent endpoint-backed payload, that references the exact signed artifacts produced for the release. The published metadata MUST use the Tauri v2 updater shape expected by desktop clients, including release version and per-platform URLs with inlined signatures.

#### Scenario: Versioned release publishes updater manifest
- **WHEN** a versioned desktop release is created from the supported release workflow
- **THEN** the workflow publishes a machine-readable updater manifest together with the released desktop artifacts
- **AND** the configured updater endpoint can resolve that manifest without requiring a separate manual post-processing step

#### Scenario: Manifest contains only complete platform entries
- **WHEN** the workflow assembles the updater manifest for a release
- **THEN** every platform entry included in the manifest has both a reachable artifact URL and the matching inlined signature
- **AND** the manifest does not publish partial or malformed platform entries that would cause Tauri clients to reject the full file

### Requirement: Private signing material stays outside the repository while public updater configuration remains reproducible
The repository SHALL keep updater signing secrets out of versioned source control. Private signing key material and any associated passwords MUST come from runtime environment or CI secret injection. Public updater values such as the verification public key, release endpoint pattern, or generated static-manifest location MUST remain reproducible by repository-supported configuration and workflow logic.

#### Scenario: Local development runs without release signing secrets
- **WHEN** a developer runs desktop development or a non-release build without updater signing secrets
- **THEN** the repository does not require committed private keys to start the app or run local validation
- **AND** updater release publication remains disabled or unavailable rather than silently producing unverifiable update metadata
