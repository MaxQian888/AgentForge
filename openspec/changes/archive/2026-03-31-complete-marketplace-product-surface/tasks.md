## 1. Runtime And Contract Foundation

- [x] 1.1 Replace the string-only marketplace installed state with typed install/consumption DTOs and routes in `src-go` so plugin, skill, and role items can report version, status, provenance, consumer surface, and failure details truthfully.
- [x] 1.2 Move the default `src-marketplace` runtime contract off the IM Bridge port, then align `src-marketplace/internal/config/config.go`, `docker-compose.dev.yml`, env defaults, and health wiring so the standalone marketplace service can run alongside the rest of the stack.
- [x] 1.3 Add or update Go tests for marketplace service availability, typed consumption responses, misconfigured `MARKETPLACE_URL`, and port-safe standalone runtime expectations.

## 2. Marketplace Service Lifecycle

- [x] 2.1 Extend `src-marketplace/internal/{handler,service,repository}` so the workspace can manage the full item lifecycle: publish, update, delete, version upload, version yank, review, verify, and feature state with stable failure semantics.
- [x] 2.2 Implement the frozen artifact validation and extraction contract for plugin, skill, and role marketplace items, including canonical zip-package handling for `roles/<id>/role.yaml` and `skills/<id>/SKILL.md`, so downstream installs fail explicitly instead of stopping at download-only placeholders.
- [x] 2.3 Add targeted `src-marketplace` tests for version management, moderation actions, artifact persistence, and cross-type item validation.

## 3. Marketplace Workspace Frontend

- [x] 3.1 Refactor `lib/stores/marketplace-store.ts` to consume the new typed install/consumption contract, expose explicit loading or empty or unavailable states, and stop relying on silent failures or `Set<string>` install tracking.
- [x] 3.2 Complete `app/(dashboard)/marketplace/page.tsx` and `components/marketplace/*` so the standalone workspace supports truthful browse/detail states, version management UI, moderation controls, install confirmation, and downstream handoff actions.
- [x] 3.3 Add explicit side-load UX for supported local file or path flows by reusing the repository's existing local source and platform file-selection seams, and render blocked reasons when the requested flow is unsupported.
- [x] 3.4 Add or update frontend tests for the marketplace store and standalone workspace, including page-level rendering, unavailable states, install warnings, version workflows, moderation affordances, and side-load actions.

## 4. Consumer Integration

- [x] 4.1 Update the plugin install bridge and `plugin-management-panel` surface so marketplace-installed plugins preserve marketplace provenance, selected version, and a navigation path back to the standalone marketplace workspace.
- [x] 4.2 Implement role marketplace install handoff into the existing roles discovery seam using the canonical role zip package contract so installed marketplace roles appear in `GET /api/v1/roles` and the roles workspace without manual filesystem steps.
- [x] 4.3 Implement skill marketplace install handoff into the authoritative role skill catalog seam using the canonical skill zip package contract so installed marketplace skills appear in `GET /api/v1/roles/skills` and role authoring selectors.
- [x] 4.4 Ensure the marketplace workspace can distinguish installed assets from assets already opened or managed in downstream consumer surfaces, and expose the appropriate deep-link or manage action for each type.

## 5. Deployment And Verification

- [x] 5.1 Update repository docs and runtime wiring for standalone marketplace deployment and separated web or desktop integration, including the correct marketplace URL, dedicated port, health endpoint, and artifact storage expectations.
- [x] 5.2 Run targeted verification for `src-marketplace`, `src-go`, and frontend marketplace surfaces so publish/version/install/consumer handoff flows are covered by real tests instead of documentation claims.
- [x] 5.3 Validate that the local stack can run the marketplace service concurrently with the Go orchestrator, TS bridge, and IM bridge without port conflicts, and record the scoped verification boundary if any runtime path remains unverified.

Verification note: `7777` (Go orchestrator), `7778` (TS bridge), `7779` (IM bridge), and `7781` (marketplace) were started concurrently and returned healthy responses; the IM bridge required one retry after the backend became ready because its first boot exited during control-plane registration.
