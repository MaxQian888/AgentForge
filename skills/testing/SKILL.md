---
name: Testing
description: Use when adding or repairing automated regression coverage, reproducing a bug, or choosing the correct verification surface across AgentForge web, bridge, backend, IM bridge, or desktop code.
tools:
  - code_editor
  - terminal
---

# Testing

Drive changes from a reproducible failure and verify them on the correct package boundary before trusting broader repo signals.

## Guardrails

- Reproduce the bug or contract drift with a focused failing test before changing production code whenever practical.
- Assert both contract-level behavior and user-visible outcomes when runtime-facing data changes.
- Prefer package-level or surface-level verification before broad repo-wide sweeps, especially in a dirty mixed-stack workspace.
- Separate product regressions from environment or sandbox issues. Report that boundary explicitly instead of silently treating them as the same failure.

## Choose The Right Surface

- Use root web checks for App Router, shared UI, and store work: `pnpm lint`, `pnpm exec tsc --noEmit`, `pnpm test`, `pnpm test:coverage`, and `pnpm build` when the change affects static export output.
- Use `src-bridge` checks for runtime, schemas, MCP, or bridge handlers: `bun test ...` and `bun run typecheck` from `src-bridge`.
- Use `go test ./...` for `src-go` backend work.
- Use `go test ./...` in `src-im-bridge` for IM bridge behavior, plus targeted smoke fixtures when platform-specific delivery changes.
- Use `pnpm test:tauri` and `pnpm test:tauri:coverage` for desktop runtime logic under `src-tauri`.

## Windows Notes

- Route-group paths such as `app/(dashboard)/...` can break naive shell invocations. Prefer explicit test paths and careful quoting over broad globs.
- Re-run the smallest failing command after each fix so you know which change actually moved the failure.

## Completion

- Finish with the narrow regression proof plus the relevant package or repo gate for the touched seam.
- If a broader gate stays red outside the changed area, record the remaining failing command and why it is outside the verified scope.

## References

- Read [references/verification-surfaces.md](references/verification-surfaces.md) when you need the repository-specific test and validation surface matrix before choosing commands.
