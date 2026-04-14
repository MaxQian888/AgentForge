## 1. CLI runtime contract alignment

- [x] 1.1 Audit the current official headless/runtime contract for `cursor`, `gemini`, `qoder`, and `iflow`, then encode the verified prompt transport, output mode, approval/config surface, auth prerequisites, and lifecycle metadata in bridge-owned runtime descriptors.
- [x] 1.2 Replace the current shared guessed launch recipes in `buildCliRuntimeLaunch(...)` with runtime-specific documented invocation helpers, removing stdin-only or undocumented flag assumptions where the upstream contract does not support them.
- [x] 1.3 Add lifecycle/deprecation handling for CLI runtimes, including iFlow shutdown-window diagnostics, migration guidance to Qoder, and launch gating after the published sunset date.

## 2. Bridge registry and preflight truthfulness

- [x] 2.1 Extend the CLI runtime adapter layer and `/bridge/runtimes` catalog to publish launch-contract metadata, machine-readable output support, auth/config status, and degraded or sunset diagnostics for each CLI runtime.
- [x] 2.2 Update execute preflight so CLI-backed runtimes reject unsupported prompt/output/approval/additional-directory/env requests before runtime acquisition, using the same reason codes that the catalog publishes.
- [x] 2.3 Adjust command-runtime normalization so each CLI runtime only emits canonical events from documented machine-readable modes, and degrades to text-only or explicit unsupported behavior when advanced events cannot be justified.

## 3. Go and frontend catalog consumers

- [x] 3.1 Update Go bridge client and coding-agent catalog normalization so project-scoped runtime catalogs preserve CLI launch-contract diagnostics, deprecation metadata, and stale-default warnings instead of flattening them to simple availability flags.
- [x] 3.2 Update project settings, runtime selectors, and agent/team launch surfaces to render degraded or deprecated CLI runtime warnings, block unavailable selections, and avoid submitting stale runtime tuples.
- [x] 3.3 Ensure resolved project defaults fall back away from CLI runtimes that are unavailable because of contract mismatch, missing official prerequisites, or sunset state.

## 4. Verification and documentation

- [x] 4.1 Add focused `src-bridge` tests covering Cursor/Gemini/Qoder/iFlow documented invocation, CLI catalog publishing, sunset handling, and truthful rejection for unsupported controls.
- [x] 4.2 Add or update Go/frontend tests proving that CLI launch-contract diagnostics, degraded states, and stale-default fallback behavior survive catalog propagation into settings and launch flows.
- [x] 4.3 Update operator-facing docs and runtime setup guidance with the verified CLI contract matrix, install/auth prerequisites, degraded capability boundaries, and the iFlow-to-Qoder migration notice.
