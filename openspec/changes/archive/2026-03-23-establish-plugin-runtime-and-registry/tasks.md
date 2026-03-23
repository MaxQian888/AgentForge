## 1. Shared Plugin Contract

- [x] 1.1 Define the unified plugin manifest schema and validation rules for plugin identity, kind, runtime, source, and permissions.
- [x] 1.2 Add shared lifecycle state definitions and health metadata fields used by both the Go orchestrator and the TS bridge.
- [x] 1.3 Document the allowed first-phase runtime mappings for executable plugin kinds and reject unsupported kind/runtime combinations.

## 2. Go Registry Foundation

- [x] 2.1 Implement the Go-side plugin registry model and repository interface for authoritative plugin records.
- [x] 2.2 Add registry operations for built-in discovery, local-path registration, listing, filtering, enable, and disable state changes.
- [x] 2.3 Wire integration-plugin runtime state and health updates into the registry lifecycle fields.

## 3. TS Tool Runtime Integration

- [x] 3.1 Implement tool-plugin runtime management in the TS bridge using the unified manifest contract.
- [x] 3.2 Add activation, health, restart, and error reporting from the TS bridge back to the Go registry.
- [x] 3.3 Ensure disabled tool plugins cannot be activated until the registry re-enables them.

## 4. Verification And Rollout Readiness

- [x] 4.1 Add tests for manifest validation, runtime routing, lifecycle transitions, and registry reconciliation across Go and TS ownership boundaries.
- [x] 4.2 Document the MVP boundaries for plugin runtime and registry work, including deferred items such as remote registry, signing, WASM, and full plugin-marketplace support.
