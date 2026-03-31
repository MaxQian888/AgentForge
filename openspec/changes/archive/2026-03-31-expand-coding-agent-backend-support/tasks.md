## 1. Shared Backend Profiles

- [x] 1.1 Introduce a checked-in coding-agent backend profile source that defines the canonical metadata for `claude_code`, `codex`, `opencode`, `cursor`, `gemini`, `qoder`, and `iflow`
- [x] 1.2 Extend shared runtime catalog DTOs and type definitions to carry richer backend metadata such as suggested model options and supported feature flags
- [x] 1.3 Add contract tests that keep the shared backend profile source aligned with Bridge runtime registration and Go fallback catalog expectations

## 2. Bridge Runtime Registry

- [x] 2.1 Expand Bridge runtime keys, execute schema validation, and runtime catalog serialization to accept the additional CLI-backed backends
- [x] 2.2 Implement a CLI-backed runtime profile adapter family in `src-bridge` and register `cursor`, `gemini`, `qoder`, and `iflow` through that shared path
- [x] 2.3 Add backend-specific readiness diagnostics for executable discovery, login or API-key prerequisites, provider-profile setup, and bounded model validation
- [x] 2.4 Enforce capability-gated lifecycle behavior so unsupported resume, fork, rollback, revert, or set-model routes fail explicitly for the new backends
- [x] 2.5 Add focused Bridge tests for runtime catalog metadata, readiness diagnostics, parser normalization, and unsupported advanced-operation behavior for the new backends

## 3. Go Catalog And Launch Resolution

- [x] 3.1 Update Go bridge client DTOs, project model DTOs, and fallback catalog builders to consume the richer runtime catalog metadata from Bridge
- [x] 3.2 Extend coding-agent selection resolution so project defaults and explicit overrides validate against the new backend profiles instead of a three-runtime static map
- [x] 3.3 Update single-agent and team launch paths to preserve explicit runtime/provider/model tuples for the new backends and reject incompatible combinations before dispatch
- [x] 3.4 Add or update Go tests for fallback catalog generation, launch tuple validation, and Team or agent launch flows using the new backend profiles

## 4. Frontend Runtime Selection

- [x] 4.1 Update project and agent stores to normalize the richer runtime catalog shape, including suggested model options and supported feature flags
- [x] 4.2 Refactor `RuntimeSelector` to honor fixed-provider backends, bounded model lists, and capability-driven constraints instead of assuming one default model per runtime
- [x] 4.3 Update settings, spawn-agent, and team-start surfaces to display backend-specific diagnostics and prevent unsupported runtime/provider/model submissions
- [x] 4.4 Add frontend tests covering provider/model constraint handling, unavailable backend diagnostics, and the new runtime catalog entries

## 5. Docs And Verification

- [x] 5.1 Update README and runtime-facing product docs with the expanded backend matrix, install/auth prerequisites, and truthful capability limitations
- [x] 5.2 Add operator-oriented verification guidance for Cursor Agent, Gemini CLI, Qoder CLI, and iFlow readiness checks
- [x] 5.3 Run targeted verification across Bridge, Go, and frontend surfaces for the expanded runtime catalog and selection flows
