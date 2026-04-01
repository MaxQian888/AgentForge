## 1. Pricing Catalog And Bridge Accounting Foundation

- [x] 1.1 Add a shared Anthropic/OpenAI pricing catalog plus model-alias normalization for the repository-supported Claude Code and Codex model options, with focused catalog tests.
- [x] 1.2 Introduce a bridge-side runtime cost accounting module that tracks cumulative usage totals, authoritative/estimated/unpriced modes, provenance metadata, and optional per-model component breakdowns.

## 2. Runtime Handler And Bridge Contract Updates

- [x] 2.1 Update Claude Code, Codex, and other relevant runtime handlers to populate the shared accounting module using native totals first, official pricing fallbacks second, and explicit unpriced states when truthful USD attribution is unavailable.
- [x] 2.2 Extend the bridge `cost_update` contract and runtime snapshot/status plumbing so Go receives the latest cumulative accounting snapshot instead of ambiguous per-step deltas.

## 3. Go Persistence And Budget Coverage

- [x] 3.1 Add persistence support for run-level accounting metadata (including migration, repository model mapping, and DTO propagation) while keeping existing scalar totals backward compatible.
- [x] 3.2 Update bridge event ingestion, agent-service cost persistence, and resource-governor recalculation so repeated updates replace the latest run totals, recompute task spend from persisted runs, and surface unpriced coverage gaps explicitly.

## 4. Cost Query And Operator Workspace

- [x] 4.1 Extend the project cost query DTOs, service, and handlers to return runtime/provider/model breakdown data plus authoritative/estimated/unpriced coverage summary metadata.
- [x] 4.2 Update the standalone cost store and cost workspace UI to render external runtime cost coverage, attribution badges, breakdown rows, and explicit warnings for unpriced runtime activity.

## 5. Verification

- [x] 5.1 Add or refresh TS bridge tests covering pricing alias resolution, native-total precedence, cumulative snapshot emission, and unpriced external-runtime paths.
- [x] 5.2 Add or refresh Go and frontend tests covering persisted accounting metadata, project cost query coverage fields, budget-coverage gap behavior, and cost workspace rendering for billed, estimated, and unpriced states.
