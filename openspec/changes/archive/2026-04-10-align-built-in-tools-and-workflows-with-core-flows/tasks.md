## 1. Starter catalog contract

- [x] 1.1 Extend `plugins/builtin-bundle.yaml` and any related bundle loader/types to carry `coreFlows`, `starterFamily`, `dependencyRefs`, and `workspaceRefs` for official ToolPlugin and WorkflowPlugin starters.
- [x] 1.2 Update `scripts/verify-built-in-plugin-bundle.js` and its tests to enforce the new starter-catalog metadata and to distinguish platform-native core starters from generic helper built-ins.

## 2. Built-in control tool starters

- [x] 2.1 Add the `task-control` built-in ToolPlugin package (manifest, MCP entrypoint, package validate path) that wraps existing task lookup, decomposition, and dispatch/status seams.
- [x] 2.2 Add the `review-control` built-in ToolPlugin package (manifest, MCP entrypoint, package validate path) that wraps supported review trigger and review inspection seams.
- [x] 2.3 Add the `workflow-control` built-in ToolPlugin package (manifest, MCP entrypoint, package validate path) that wraps workflow start, detail, and recent-run inspection seams.
- [x] 2.4 Add targeted tests or validation coverage proving the new control tools resolve from the official bundle and reject invalid control-scope input.

## 3. Built-in workflow starter library

- [x] 3.1 Update `standard-dev-flow` metadata so it remains the minimal sequential quickstart starter inside the new starter library.
- [x] 3.2 Add the `task-delivery-flow` WorkflowPlugin starter and supporting runtime artifact wiring for planner → coding → review sequential handoff.
- [x] 3.3 Add the `review-escalation-flow` WorkflowPlugin starter and supporting runtime artifact wiring for review → approval pause handoff.
- [x] 3.4 Add or update workflow validation/tests for starter role bindings, trigger profiles, and persisted step-output / pause-state behavior.

## 4. Docs and consumer alignment

- [x] 4.1 Update `docs/PRD.md`, `docs/part/PLUGIN_SYSTEM_DESIGN.md`, `docs/part/AGENT_ORCHESTRATION.md`, and `docs/part/REVIEW_PIPELINE_DESIGN.md` to document the official built-in starter catalog and its core-flow mapping.
- [x] 4.2 Thread the new starter metadata through the existing built-in discovery consumers that need it for workflow/plugin/operator-facing guidance.
- [x] 4.3 Run targeted verification for bundle validation, new tool package validation, and workflow starter checks; record any remaining unsupported trigger/runtime boundaries truthfully.
