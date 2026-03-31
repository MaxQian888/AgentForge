# Implementation Tasks: Enhance Go Instruction Support

## 1. Instruction Router Contract

- [x] 1.1 Add the Go `internal/instruction` package with router types, handler definitions, and queue structures.
- [x] 1.2 Implement registered-definition execution across `local`, `bridge`, and `plugin` targets.
- [x] 1.3 Implement request normalization, validation, timeout handling, cancellation, dependency checks, history, and metrics.
- [x] 1.4 Add unit and integration coverage for router execution, queue ordering, dependency blocking, and cancellation semantics.

## 2. Role Runtime Contract

- [x] 2.1 Expand role parsing and storage to support the advanced manifest schema and canonical `roles/<id>/role.yaml` layout with legacy flat-file fallback.
- [x] 2.2 Implement effective-role inheritance and stricter governance merging for security and resource limits.
- [x] 2.3 Implement role preview and sandbox flows that return normalized/effective manifests, execution profiles, and readiness diagnostics.
- [x] 2.4 Add backend and frontend role contract coverage for advanced authoring sections, provenance, skill diagnostics, and runtime projection.

## 3. Bridge Transport Contract

- [x] 3.1 Wire the internal bridge websocket to accept serialized runtime events in Go without crashing on malformed payloads.
- [x] 3.2 Implement TS bridge event transport behaviors for ready signaling, reconnect buffering, and heartbeats.
- [x] 3.3 Project the currently supported runtime events in Go orchestration: output, cost updates, and terminal status changes.

## 4. Memory Contract

- [x] 4.1 Implement short-term memory with scoped snapshots, token budgets, and configurable eviction policies.
- [x] 4.2 Implement the repository-backed memory service for storing, searching, deleting, and injecting project memory context.
- [x] 4.3 Persist team learnings into project episodic memory entries.

## 5. Plugin, Team, and Workflow Contracts

- [x] 5.1 Implement the Go-managed WASM plugin runtime manager with activation, ABI validation, capability gating, health checks, and restart support.
- [x] 5.2 Implement strategy-based team startup with persisted runtime selection and team-context-aware child spawns.
- [x] 5.3 Implement workflow plugin execution with supported process modes, retries, approval pause semantics, and step router actions for agent, review, task, workflow, and approval.

## 6. Governance, Security, and Knowledge Contracts

- [x] 6.1 Implement task-level budget tracking, warning-threshold detection, and budget-exhaustion runtime stop behavior.
- [x] 6.2 Preserve and project declarative role security policies, including stricter inherited merge semantics.
- [x] 6.3 Preserve and project declarative role knowledge references into runtime knowledge context.

## 7. Artifact Alignment

- [x] 7.1 Rewrite proposal, design, and capability specs to reflect the currently implemented Go instruction-support contract rather than a future roadmap.
- [x] 7.2 Validate the `enhance-go-instruction-support` change with OpenSpec after artifact alignment.
