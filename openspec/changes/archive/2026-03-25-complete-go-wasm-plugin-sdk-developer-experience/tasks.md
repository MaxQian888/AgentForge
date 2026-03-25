## 1. Go SDK Contract And Runtime Helpers

- [x] 1.1 Expand `src-go/plugin-sdk-go` with manifest-aligned typed descriptor, capability, invocation context, result, and runtime error helpers while preserving the current Go-hosted WASM ABI entrypoints.
- [x] 1.2 Add unified export and autorun helpers so Go-hosted plugins do not need to hand-roll `agentforge_abi_version`, `agentforge_run`, or envelope plumbing in every sample or template.
- [x] 1.3 Update the Go WASM runtime integration path to consume the richer SDK descriptor and bounded execution context without regressing current activation, health, or invoke behavior.

## 2. Authoring Workflow And Samples

- [x] 2.1 Refactor the maintained Go-hosted sample or template to use the new SDK helpers and preserve manifest-aligned runtime metadata, capabilities, and supported operations.
- [x] 2.2 Extend the repository build helper flow for Go-hosted WASM plugins so the supported sample or template can build a valid artifact and manifest combination through one documented workflow.
- [x] 2.3 Add or refresh repository fixtures needed to validate the supported Go plugin authoring workflow, including manifest/module alignment and current supported-boundary examples.

## 3. Verification And Documentation

- [x] 3.1 Add focused tests for Go SDK helpers, structured result or error handling, sample build output, and Go runtime contract validation paths.
- [x] 3.2 Update `docs/GO_WASM_PLUGIN_RUNTIME.md` and related plugin design references so documentation reflects the current supported Go SDK surface, authoring workflow, and repo-truth boundaries.
- [x] 3.3 Run scoped verification for the maintained Go-hosted plugin workflow and record the exact commands or scripts that prove the SDK sample or template remains buildable and activatable.
