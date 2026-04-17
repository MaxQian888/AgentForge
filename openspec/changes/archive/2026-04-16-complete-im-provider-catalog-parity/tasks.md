## 1. Backend provider catalog and surface-specific validation

- [x] 1.1 Add an authoritative IM provider catalog model/service in `src-go` that describes operator-visible built-in providers, including `wechat` and `email`, with interaction class, test-send support, and configuration field schema.
- [x] 1.2 Expose the canonical provider catalog through an authenticated backend API route and wire it into the existing IM operator/control-plane handler surface.
- [x] 1.3 Split IM platform validation by surface so interactive inbound message or action flows accept `wechat` but reject delivery-only `email`, while delivery-oriented surfaces such as channel config and test-send still accept `email`.
- [x] 1.4 Add or update focused Go tests for the provider catalog payload, `wechat` / `email` validation truth, and the touched IM handler or service contracts.

## 2. Frontend operator catalog consumption

- [x] 2.1 Update `lib/stores/im-store.ts` to fetch and store the canonical provider catalog, and widen the frontend IM platform handling to cover `wechat` and `email`.
- [x] 2.2 Refactor `components/im/im-channel-config.tsx` and related shared platform metadata so channel configuration fields and provider affordances come from the backend catalog instead of stale hardcoded schema definitions.
- [x] 2.3 Update `/im` and settings IM surfaces (`app/(dashboard)/im/page.tsx`, `components/im/*`, `app/(dashboard)/settings/_components/section-im-bridge.tsx`) to render catalog-driven provider cards, delivery-only labeling, and truthful test-send affordances.
- [x] 2.4 Add or update focused frontend tests covering catalog loading, `wechat` availability, `email` delivery-only behavior, and the updated `/im` / settings operator flows.

## 3. Bridge registry and documentation truth sync

- [x] 3.1 Update `src-im-bridge/README.md` and `src-im-bridge/docs/platform-runbook.md` so the built-in provider list, transport matrix, and manual verification guidance truthfully include `wechat` and `email`.
- [x] 3.2 Add or update focused `src-im-bridge` checks that lock the expected built-in provider truth for `wechat` and `email` against the operator-facing catalog assumptions used by backend and docs.

## 4. Verification and rollout evidence

- [x] 4.1 Run focused `src-go` verification for the provider catalog endpoint, touched IM validation, and related handler or service slices.
- [x] 4.2 Run focused frontend verification for `/im` and settings catalog consumption plus the updated operator interaction tests.
- [x] 4.3 Run focused `src-im-bridge` verification for provider registry or docs-related slices, then record any remaining repo-wide failures outside this change boundary without overstating completion.
