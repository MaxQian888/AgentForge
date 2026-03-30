## 1. Shared Cost Query Backend

- [x] 1.1 Introduce typed standalone cost query DTOs/services in `src-go/internal/model` and `src-go/internal/service` that assemble project cost summary, budget snapshot, recorded trend, sprint/task breakdowns, and period rollups from existing persisted seams.
- [x] 1.2 Extend the relevant repository/query layer in `src-go/internal/repository` (and supporting readers such as task/sprint access where needed) so project cost summary, cost-aligned velocity points, and truthful performance buckets can be loaded without frontend-side synthesis.
- [x] 1.3 Rewire `src-go/internal/handler/cost_handler.go`, `src-go/internal/handler/stats_handler.go`, and server construction so `/api/v1/stats/cost`, `/api/v1/stats/velocity`, and `/api/v1/stats/agent-performance` all serve the new authoritative contracts while keeping the route surface stable.

## 2. Cost Workspace Alignment

- [x] 2.1 Update `lib/stores/cost-store.ts` to consume the authoritative response wrappers/shapes, normalize them for the existing cost components, and remove `agent-store`-driven headline fallback behavior.
- [x] 2.2 Update `app/(dashboard)/cost/page.tsx` and `components/cost/*` so the workspace renders explicit no-project, loading, empty, and failure states, and so performance labels/copy match the truthful aggregation bucket semantics.
- [x] 2.3 Update cost-related translations and any page/component tests that currently assume misleading “per-agent” wording or silent hidden sections.

## 3. Lightweight Consumer Compatibility

- [x] 3.1 Update `src-im-bridge/client/agentforge.go` and `src-im-bridge/commands/cost.go` so IM `/cost` reads the canonical project cost summary or its documented compatibility projection instead of a drifted standalone schema.
- [x] 3.2 Ensure the cost summary contract exposes the period/budget fields needed by lightweight consumers without requiring a second hidden endpoint or client-side recomputation path.

## 4. Verification

- [x] 4.1 Add or update Go tests for the new summary/velocity/performance service and handler paths, covering populated project data, empty project data, and truthful performance labeling semantics.
- [x] 4.2 Add or update frontend tests for `lib/stores/cost-store.ts`, `app/(dashboard)/cost/page.tsx`, and affected cost components to cover wrapper normalization, no-project behavior, explicit empty/error states, and removal of unrelated fallback totals.
- [x] 4.3 Add or update IM bridge tests to prove `/cost` still renders a valid summary from the canonical cost query contract after the backend/frontend alignment.
