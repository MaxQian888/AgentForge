# plugin-development-scripts Specification

## Purpose
Define the repository-supported plugin build, debug, local run, and verification workflows so maintained manifest-backed plugins can be developed and validated without editing script internals.
## Requirements
### Requirement: Repository exposes parameterized plugin build commands
The repository SHALL provide supported root-level plugin build commands that can target a maintained plugin by manifest path, plugin path, or explicit source override without requiring developers to edit script source. The build workflow MUST resolve runtime-relevant output paths from manifest metadata when available and MUST fail with actionable validation output before invoking a compiler when the target configuration is incomplete.

#### Scenario: Maintained Go-hosted plugin builds from manifest input
- **WHEN** a developer invokes the supported build command for a maintained Go-hosted plugin by pointing at its manifest
- **THEN** the command resolves the correct entrypoint and output path, produces the artifact expected by that manifest, and reports where the artifact was written

#### Scenario: Invalid build target is rejected before compilation
- **WHEN** a developer invokes the supported build command for a plugin target whose manifest omits required runtime metadata
- **THEN** the command exits non-zero with a validation error that identifies the missing field instead of starting a partial build

### Requirement: Repository exposes a local plugin debug runner
The repository SHALL provide a supported local debug command for maintained plugins that replays the same operation, config, and payload envelope contract used by the platform runtime. The debug workflow MUST surface the plugin's structured stdout result, stderr logs, and final exit status so developers can iterate without first registering the plugin in the control plane.

#### Scenario: Go WASM debug runner replays runtime envelope
- **WHEN** a developer invokes the supported debug command for a Go-hosted plugin with a target operation, config, and payload
- **THEN** the command passes those values through the platform's `AGENTFORGE_*` runtime contract and prints the resulting structured response

#### Scenario: Debug runner reports plugin failure details
- **WHEN** a plugin returns an error envelope or invalid output during a debug run
- **THEN** the debug command exits non-zero and reports which operation failed, the observed stderr output, and why the result was considered invalid

### Requirement: Repository exposes a minimal local plugin development stack command
The repository SHALL provide a supported local run command that starts or reuses the minimal AgentForge services needed for plugin authoring and reports readiness in one place. The command MUST cover the repo-truthful Go Orchestrator and TS Bridge surfaces used by plugin activation and interaction, and MUST report missing prerequisites or unhealthy services with explicit next-step guidance.

#### Scenario: Local plugin stack reports ready services
- **WHEN** a developer starts the supported local plugin stack command on a machine with the required dependencies available
- **THEN** the command starts or reuses the Go Orchestrator and TS Bridge, waits for their health checks, and prints the active ports and health endpoints

#### Scenario: Local plugin stack reports missing prerequisite
- **WHEN** a required dependency or subprocess for the minimal plugin stack cannot be started
- **THEN** the command fails with a message that identifies the missing prerequisite or failed subprocess instead of silently hanging

### Requirement: Repository verifies maintained plugin script workflows
The repository SHALL provide a verification workflow that exercises the maintained plugin build, debug, and manifest-validation paths so repository scripts, samples, and documentation remain aligned. Verification MUST identify which plugin target and which workflow step failed.

#### Scenario: Verification covers maintained Go sample workflow
- **WHEN** the maintained plugin verification workflow runs for the Go-hosted sample plugin
- **THEN** it validates the manifest, builds the artifact, and completes at least one debug or activation smoke step against the generated module

#### Scenario: Verification pinpoints broken maintained workflow
- **WHEN** any maintained plugin script workflow drifts from the expected manifest, artifact, or command contract
- **THEN** the verification command exits non-zero and identifies the failing plugin target and workflow stage
