## 1. Built-in Skill Bundle Foundation

- [x] 1.1 Add `skills/builtin-bundle.yaml` as the repo-owned truth source for official built-in skills and seed it with the current built-in packages that should appear in the marketplace surface.
- [x] 1.2 Normalize the current built-in skill packages under `skills/*` so each bundled skill produces the minimum market-facing metadata and preview inputs required by the new bundle and preview contracts.

## 2. Skill Preview And Consumption Contracts

- [x] 2.1 Implement built-in skill bundle loading plus built-in skill marketplace list or detail DTOs in `src-go`, reusing the existing skill package parsers instead of creating a parallel parser.
- [x] 2.2 Extend `src-marketplace` skill artifact upload or item detail handling so marketplace-published skill items can return the shared `SkillPackagePreview` shape without requiring the frontend to download and parse archives.
- [x] 2.3 Extend the typed marketplace consumption contract in `src-go` so official built-in skills surface as already discoverable local assets with truthful provenance, local path, and `role-skill-catalog` consumer-surface state.

## 3. Marketplace Workspace Integration

- [x] 3.1 Refactor `lib/stores/marketplace-store.ts` to load the built-in skill feed, remote marketplace items, preview data, and partial-unavailable states without collapsing repo-owned built-ins into remote pagination totals.
- [x] 3.2 Update `app/(dashboard)/marketplace/page.tsx` and `components/marketplace/*` so the workspace renders a dedicated built-in skill section, provenance-aware skill detail, and truthful next actions for built-in versus remote skill items.
- [x] 3.3 Ensure skill detail and handoff UX distinguishes install-required items from skills that are already locally discoverable through the role skill catalog, including downstream deep links for role authoring.

## 4. Rendering, Verification, And Docs

- [x] 4.1 Add the frontend Markdown/YAML rendering dependencies and implement a shared skill content renderer that uses mature libraries rather than hand-written Markdown or YAML serializers.
- [x] 4.2 Add or extend backend and frontend tests for built-in skill bundle drift, skill preview DTO generation, built-in consumption synthesis, partial-unavailable marketplace states, and structured skill detail rendering.
- [x] 4.3 Update the relevant docs and verification entrypoints so the built-in skill marketplace surface, preview contract, and bundle-alignment checks remain truthful as `skills/` packages evolve.
