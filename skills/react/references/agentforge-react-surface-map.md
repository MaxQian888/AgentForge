# AgentForge React Surface Map

Use this reference when the task spans multiple React seams and you need a quick repo-truthful map before editing.

## Canonical Surfaces

- `app/` owns App Router entrypoints and route groups.
- `components/` owns reusable product UI and workspace composition.
- `lib/stores/` owns client-side state and API-backed projections.
- `lib/roles/` owns role-authoring draft serialization and skill resolution helpers.

## Shared Constraints

- `next.config.ts` uses static export, so root layout and provider changes must stay server-safe.
- Dashboard route-group paths can require explicit Windows-safe test invocation.
- Shared desktop shell surfaces can also affect Tauri window chrome and drag regions.

## Common Verification

- Root frontend: `pnpm lint`
- Root typecheck: `pnpm exec tsc --noEmit`
- Root tests: `pnpm test`
- Static export build: `pnpm build`
