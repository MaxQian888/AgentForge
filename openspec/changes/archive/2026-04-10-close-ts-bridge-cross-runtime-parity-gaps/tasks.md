## 1. Capture parity and rollback acceptance coverage

- [x] 1.1 Add or update `src-bridge` runtime-registry and server tests that lock rollback capability truth, provider-auth state, and live-control publishing against the new contract.
- [x] 1.2 Add focused OpenCode and Codex handler/transport tests for parity-sensitive execute inputs (`attachments`, `env`, `web_search`) plus structured rejection paths.

## 2. Complete OpenCode execute parity

- [x] 2.1 Extend the OpenCode transport/session APIs so canonical execute flow can carry attachments, env values, and web-search intent through Bridge-owned session, prompt, or config surfaces.
- [x] 2.2 Update the OpenCode runtime handler to preserve accepted parity-sensitive inputs, reject unsupported combinations explicitly, and keep continuity/session identity stable across later controls.

## 3. Close canonical rollback gaps across runtimes

- [x] 3.1 Implement Codex rollback through a continuity-aware native connector path and refresh stored Codex continuity metadata after rollback.
- [x] 3.2 Implement OpenCode rollback resolution through persisted upstream session bindings for both active and paused tasks.
- [x] 3.3 Normalize rollback failure handling so missing continuity, missing native primitives, and unsupported runtime states return structured runtime-aware errors instead of generic failures.

## 4. Align runtime catalog truthfulness and verify

- [x] 4.1 Refactor `interaction_capabilities` publication to derive support/degraded state from runtime probes, provider-auth readiness, and live-control availability instead of static runtime-name assumptions.
- [x] 4.2 Ensure canonical control routes reuse the same capability truth and reason-code families published by `/bridge/runtimes`.
- [x] 4.3 Run focused `src-bridge` verification for OpenCode, Codex, runtime-registry, and server control suites, then record any remaining truthful partial boundaries.
