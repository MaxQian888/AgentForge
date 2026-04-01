## 1. Shared Dependency Evaluation

- [x] 1.1 Add shared Go-side models and helpers that derive workflow role bindings, role-scoped plugin or MCP dependencies, and reverse consumer summaries from the current role store and plugin registry.
- [x] 1.2 Extend plugin service and DTO shaping so plugin list or detail responses expose role dependency diagnostics and reverse role consumer summaries.
- [x] 1.3 Extend role list/get/preview/sandbox shaping so role responses expose plugin dependency summaries, downstream plugin consumers, and action-ready impact metadata.
- [x] 1.4 Extend the role tool contract so role manifests and execution profiles preserve plugin function bindings instead of only plugin ids.

## 2. Runtime And Destructive Guards

- [x] 2.1 Revalidate workflow role dependencies during plugin enable or activate flows and return explicit stale-role errors when bindings drift.
- [x] 2.2 Revalidate workflow execution against the current role registry before step execution starts so stale role bindings fail fast without partial runs.
- [x] 2.3 Block role deletion when installed plugins still reference the target role and return the blocking consumer details in the API error response.
- [x] 2.4 Treat missing or unusable role-scoped plugin or MCP dependencies as blocking readiness in preview, sandbox, spawn, and workflow role execution paths.

## 3. Operator Surface Integration

- [x] 3.1 Update plugin store types and plugin detail components to render role dependency health, reverse role consumers, blocked-action messaging, and roles workspace deep links.
- [x] 3.2 Update role store types, role management helpers, and role workspace or context rail components to render plugin dependency health and downstream plugin consumer summaries.
- [x] 3.3 Update role destructive-action UX so dependency blockers and remediation guidance are visible before delete attempts.
- [x] 3.4 Update the role capabilities editor so operators can select installed plugins, inspect their declared functions, and persist plugin function bindings in the current draft.

## 4. Verification

- [x] 4.1 Add or extend Go tests covering shared dependency evaluation, workflow stale-role guards, role delete guards, and role readiness diagnostics for missing plugin dependencies.
- [x] 4.2 Add or extend frontend tests covering plugin detail role diagnostics, role workspace dependency summaries, and blocked delete messaging.
- [x] 4.3 Run targeted backend and frontend verification for the touched seams and record any remaining repo-level blockers separately from this change.
- [x] 4.4 Add targeted bridge schema or serialization verification for plugin function binding round-trip.
