## 1. Bridge provider foundation

- [x] 1.1 Add Vercel AI SDK core and the initial real provider packages to `src-bridge/package.json`, and document the minimal provider env vars in `src-bridge/.env.example`.
- [x] 1.2 Introduce a Bridge-local provider registry/config module that defines supported providers, per-capability availability, and default models for `agent_execution` and `text_generation`.
- [x] 1.3 Add focused tests for provider registry resolution, default selection, and explicit validation errors for unknown or unsupported providers.

## 2. Provider-aware request contracts

- [x] 2.1 Extend Bridge request types and Zod schemas so execute and decomposition requests can carry optional `provider` and `model` fields while preserving repository-safe defaults.
- [x] 2.2 Update the server/handler entrypoints to resolve provider configuration before dispatch and return explicit errors for unavailable credentials, unsupported capabilities, or invalid provider selections.
- [x] 2.3 Align the Go Bridge client contract and any affected service-layer request structs with the new provider/model semantics without breaking existing default-call flows.

## 3. Real lightweight provider execution

- [x] 3.1 Replace the simulated decomposition executor with a Vercel AI SDK-backed text-generation adapter that runs against the resolved provider/model.
- [x] 3.2 Keep decomposition output schema validation in place so malformed provider responses fail cleanly instead of producing fabricated fallback data.
- [x] 3.3 Add focused Bridge tests for successful decomposition through a resolved provider path and for failure cases such as invalid output or missing provider credentials.

## 4. Agent execution alignment and verification

- [x] 4.1 Update the execute path so provider-aware requests resolve through the shared registry and only supported `agent_execution` providers can start a runtime.
- [x] 4.2 Preserve the existing Claude Agent SDK runtime for supported execute requests and ensure unsupported execution providers are rejected before a runtime is acquired.
- [x] 4.3 Run the relevant Bridge and Go verification commands for provider resolution, schema validation, and decomposition behavior, and confirm the change is ready to apply.
