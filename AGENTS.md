# Repository Guidelines

**Subproject guides:** [Go backend](src-go/AGENTS.md) · [Marketplace](src-marketplace/AGENTS.md) · [Bridge](src-bridge/AGENTS.md) · [IM Bridge](src-im-bridge/AGENTS.md) · [Tauri](src-tauri/AGENTS.md) · [Skills](.agents/AGENTS.md)


## Project Structure & Module Organization

- `app/` Next.js App Router (routes: `page.tsx`, `layout.tsx`, global styles in `globals.css`).
  - `app/(auth)/` — Login, registration, invitation flows.
  - `app/(dashboard)/` — Main dashboard routes: overview, agents, employees, teams, reviews, cost, scheduler, memory, roles, plugins, marketplace, settings, IM, docs, workflow, sprints, documents, skills, debug.
  - `app/forms/` — Standalone form pages.
- `components/ui/` — 60+ shadcn/ui components (Radix UI + class-variance-authority).
- `components/` — Domain-specific component folders: `agent/`, `agents/`, `automations/`, `cost/`, `dashboard/`, `docs/`, `im/`, `knowledge/`, `marketplace/`, `qianchuan/`, etc.
- `hooks/` — React hooks, co-located with `*.test.ts` coverage.
- `lib/stores/` — Zustand stores (70+ store files), co-located with `*.test.ts` coverage.
- `lib/i18n/` — `next-intl` localization (messages in `messages/`).
- `lib/utils.ts` — `cn()` utility (clsx + tailwind-merge).
- `public/` — Static assets (SVGs, icons).
- `src-tauri/` — Tauri desktop wrapper (Rust code, config, icons).
- `src-go/` — Go orchestrator backend.
- `src-bridge/` — TypeScript/Bun agent bridge with ACP runtime integration.
- `src-im-bridge/` — Go IM bridge for multi-provider connectivity.
- `src-marketplace/` — Standalone Go marketplace microservice.
- `plugins/` — Plugin catalog: built-in workflows, integration plugins, WASM binaries.
- `scripts/` — Build, dev, plugin, skill, and i18n automation scripts.
- Root configs: `next.config.ts`, `tsconfig.json`, `eslint.config.mjs`, `postcss.config.mjs`, `components.json`, `jest.config.ts`, `jest.setup.ts`.

## Build, Test, and Development Commands

- `pnpm dev` — Run Next.js in development.
- `pnpm build` — Create a production build (static export to `out/`).
- `pnpm start` — Serve the production build.
- `pnpm lint` — Run ESLint. Use `--fix` to auto-fix.
- `pnpm test` — Run Jest unit/integration tests.
- `pnpm test:watch` — Run Jest in watch mode.
- `pnpm test:coverage` — Run Jest with coverage report.
- `pnpm test:e2e` — Run Playwright end-to-end tests.
- `pnpm test:tauri` — Run Tauri Rust unit tests.
- `pnpm tauri dev` — Launch desktop app (requires Rust toolchain).
- `pnpm tauri build` — Build desktop binaries.
- `pnpm desktop:dev:prepare` — Build backend + bridge + im-bridge for current platform.
- `pnpm desktop:standalone:check` — Verify desktop sidecar health.
- `pnpm desktop:standalone:dev` — Run desktop with sidecars in standalone mode.
- `pnpm dev:all` — Start full local stack (infra + Go + Bridge + IM Bridge + frontend).
- `pnpm dev:all:status` / `pnpm dev:all:stop` / `pnpm dev:all:logs` / `pnpm dev:all:verify` — Stack lifecycle commands.
- `pnpm dev:backend` — Start backend-only (PG + Redis + Go + Bridge + IM Bridge).
- `pnpm dev:backend:watch` — Backend with air hot-reload for Go.
- `pnpm plugin:build` / `pnpm plugin:debug` / `pnpm plugin:dev` / `pnpm plugin:verify` — Plugin toolchain.
- `pnpm skill:sync:mirrors` / `pnpm skill:verify:internal` / `pnpm skill:verify:builtins` — Skill toolchain.
- `pnpm i18n:audit` — Audit missing i18n keys.

## Coding Style & Naming Conventions

- Language: TypeScript with React 19 and Next.js 16.
- Linting: `eslint.config.mjs` is the source of truth; keep code warning-free.
- Styling: Tailwind CSS v4 (utility-first). Co-locate minimal component-specific styles.
- Components: PascalCase names/exports; files in `components/ui/` mirror export names.
- Routes: Next app files are lowercase (`page.tsx`, `layout.tsx`).
- Code: camelCase variables/functions; hooks start with `use*`.

## Testing Guidelines

- **Test runner**: Jest with `next/jest` integration, jsdom environment, and `jest.setup.ts`.
- **E2E**: Playwright (`pnpm test:e2e`).
- **Tauri Rust tests**: `cargo test` via `pnpm test:tauri` with coverage gates (`pnpm test:tauri:coverage`).
- Name tests `*.test.ts`/`*.test.tsx`; co-locate next to source.
- Coverage thresholds: branches/functions 60%, lines/statements 70%.
- Prioritize `lib/` utilities, stores, hooks, and complex UI logic for coverage.
- Jest ignores: `src-bridge/`, `src-tauri/`, `plugins/reviews/`, `plugins/tools/`, `e2e/`.

## Commit & Pull Request Guidelines

- Prefer Conventional Commits: `feat:`, `fix:`, `docs:`, `refactor:`, `chore:`, `ci:`.
- Link issues in the footer: `Closes #123`.
- PRs should include: brief scope/intent, screenshots for UI changes, validation steps, and pass `pnpm lint`.
- Keep changes focused; avoid unrelated refactors.

## Security & Configuration Tips

- Use `.env.local` for secrets; do not commit `.env*` files.
- Only expose safe client values via `NEXT_PUBLIC_*`.
- Tauri: minimize capabilities in `src-tauri/tauri.conf.json`; avoid broad filesystem access.
- **Git hooks**: Husky + lint-staged are configured. Staged `*.{js,jsx,ts,tsx,mjs,cjs}` files automatically run `eslint --fix --max-warnings=0` on commit.
