## 1. Codex Connector Foundation

- [x] 1.1 Add the bridge-owned Codex connector module and any required official Codex integration dependency or wrapper support under `src-bridge`.
- [x] 1.2 Split Codex off from the generic command-runtime path so `runtime=codex` resolves to the dedicated adapter while `opencode` keeps its existing command adapter.
- [x] 1.3 Extend bridge runtime and snapshot types to store Codex-specific continuity metadata needed for truthful pause and resume flows.

## 2. Registry And Lifecycle Integration

- [x] 2.1 Update the runtime registry to evaluate Codex connector readiness using real connector and authentication prerequisites instead of only executable discovery.
- [x] 2.2 Wire Codex execute, pause, status, and resume flows to the dedicated connector so canonical `/bridge/*` routes preserve resolved runtime identity and explicit configuration errors.
- [x] 2.3 Ensure persisted snapshots and restart recovery paths reuse stored Codex continuity metadata and fail explicitly when that metadata is missing or invalid.

## 3. Verification And Documentation

- [x] 3.1 Add focused bridge tests for Codex registry diagnostics, successful execute/event normalization, and launch-time configuration failures.
- [x] 3.2 Add pause/resume snapshot tests that prove Codex continuity metadata is stored, reused on resume, and rejected explicitly when unavailable.
- [x] 3.3 Update README documentation for supported Codex setup, authentication prerequisites, and the truthful meaning of pause/resume for Codex runs.
