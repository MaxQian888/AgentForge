## 1. WASM Manifest And Registry Contract

- [x] 1.1 Extend the Go plugin model and manifest parser to support `runtime: wasm`, `spec.module`, `spec.abiVersion`, and related validation/error paths.
- [x] 1.2 Update registry-facing plugin record metadata and runtime status plumbing so WASM plugins preserve resolved module source and ABI compatibility details.
- [x] 1.3 Migrate the built-in Go-hosted plugin manifest and fixtures to the WASM contract, including clear handling or migration messaging for legacy `go-plugin` declarations.

## 2. Go WASM Runtime Manager

- [x] 2.1 Add a `wazero`-backed Go runtime manager that resolves plugin module artifacts, instantiates modules, and tracks active plugin instances.
- [x] 2.2 Refactor `PluginService` activation, health-check, and restart flows to delegate Go-hosted executable plugins to the runtime manager instead of optimistic state updates.
- [x] 2.3 Synchronize real activation, degradation, health, and restart outcomes from the Go WASM runtime back into the authoritative plugin registry and API responses.

## 3. Go WASM SDK And Sample Plugin

- [x] 3.1 Create the `go-wasm-plugin-sdk` package with ABI/version constants, exported entrypoint wrappers, JSON envelope helpers, and safe host bindings for logging/config/result handling.
- [x] 3.2 Add a sample Go WASM plugin plus build workflow that produces the module artifact referenced by its manifest and exercises the SDK contract.
- [x] 3.3 Add runtime integration coverage for SDK compatibility checks, initialization, health checks, and at least one real invocation against the sample plugin.

## 4. Verification And Documentation

- [x] 4.1 Add parser, service, and handler tests covering WASM manifest validation, registry metadata persistence, activation failure, health reporting, and restart behavior.
- [x] 4.2 Update repository documentation for the Go WASM plugin runtime, SDK authoring flow, and local verification steps.
- [x] 4.3 Run focused verification for the Go plugin APIs and sample plugin build/load workflow, then capture any residual follow-up items before apply.
