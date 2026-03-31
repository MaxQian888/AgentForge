---
name: TypeScript
description: Use when changing TypeScript contracts, bridge payloads, store shapes, schema validation, or safe refactors across AgentForge frontend and bridge packages.
tools:
  - code_editor
  - terminal
---

# TypeScript

Keep type contracts explicit across transport, runtime, store, and UI boundaries so changes do not drift silently between packages.

## Guardrails

- Decide where transport payloads are normalized and keep that boundary explicit. Do not mix API-shape objects and app-shape objects in the same layer.
- Update schema validation, runtime handling, store normalization, and consumer types together when a field changes.
- Prefer narrowing and additive evolution over `any`, unchecked casts, non-null assertions, or silent fallback coercion.
- Preserve optional and nullable semantics. Do not turn missing data into empty strings, zeros, or booleans unless the boundary contract explicitly requires it.
- Keep public types small and intentional. Push one-off helper types down to the feature or module that owns them.

## Cross-Stack Discipline

- When a field crosses Go, Bridge, and UI boundaries, inspect all three surfaces in one pass before editing.
- Keep serialization deterministic for role configs, bridge requests, workflow payloads, and operator-facing diagnostics.
- Update Zod schemas, request/response tests, and normalization helpers alongside the implementation so the contract is enforced instead of implied.
- Prefer feature-local helpers for mapping or normalization over repeated inline rewrites inside components and handlers.

## Verification

- Run the package-local typecheck that matches the touched surface: root `pnpm exec tsc --noEmit` or `bun run typecheck` in `src-bridge`.
- Re-run targeted tests for schema parsing, normalization logic, and handler payloads after contract changes.
- Treat leftover type noise outside the touched seam as unverified scope, not as proof that your change is complete.

## References

- Read [references/cross-stack-contracts.md](references/cross-stack-contracts.md) when the change crosses frontend, bridge, and Go boundaries and you need the canonical contract touchpoints.
