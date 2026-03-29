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

### Requirement: Repository verifies maintained built-in plugins by host family
The repository SHALL provide supported verification workflows for the official built-in plugin bundle grouped by host family instead of relying on one monolithic live smoke run. Maintained Go-hosted wasm plugins, MCP tool plugins, MCP review plugins, and workflow starters MUST each have a documented verification path that identifies the target plugin and the workflow stage being validated.

#### Scenario: MCP built-in plugin verification validates maintained entrypoints
- **WHEN** a developer or CI workflow verifies an official built-in ToolPlugin or ReviewPlugin
- **THEN** the repository validation checks the maintained manifest, declared entrypoint or command contract, and family-specific prerequisites without requiring edits to script internals

#### Scenario: Workflow starter verification pinpoints starter-specific failures
- **WHEN** a maintained built-in workflow starter drifts from its declared manifest, role references, or execution contract
- **THEN** the verification workflow exits non-zero and identifies the failing workflow starter and validation stage instead of reporting a generic plugin failure

### Requirement: Live smoke remains explicit for prerequisite-heavy built-ins
The repository SHALL distinguish deterministic default verification from optional live smoke for official built-ins that require network access, secrets, or third-party local services. Default verification MUST remain bounded and reproducible, while live smoke MUST be opt-in and clearly identify the missing prerequisite when it cannot run.

#### Scenario: Default verification skips secret-dependent live execution
- **WHEN** an official built-in plugin requires secrets or external network access for live runtime execution
- **THEN** the default verification path validates the maintained contract without silently attempting a flaky live run

#### Scenario: Opt-in live smoke reports missing prerequisite
- **WHEN** a developer explicitly triggers live smoke for a prerequisite-heavy built-in without satisfying its dependencies
- **THEN** the live smoke command fails with a message that identifies the missing prerequisite instead of hanging or reporting an unrelated runtime error

### Requirement: Repository verifies official built-in readiness contracts
The repository SHALL provide a bounded verification workflow that validates official built-in readiness metadata and deterministic readiness preflight behavior without requiring secret-dependent or network-dependent live execution. Verification MUST identify which built-in entry and which readiness stage failed.

#### Scenario: Readiness verification catches bundle metadata drift
- **WHEN** an official built-in bundle entry declares malformed or incomplete readiness metadata
- **THEN** the readiness verification workflow exits non-zero
- **THEN** the failure identifies the affected built-in entry and readiness field

#### Scenario: Readiness verification reports missing deterministic prerequisite
- **WHEN** a readiness preflight evaluates a built-in plugin that depends on a missing local tool or host capability
- **THEN** the verification output reports that deterministic prerequisite failure for the affected built-in
- **THEN** the workflow does not replace that result with an unrelated generic plugin failure

