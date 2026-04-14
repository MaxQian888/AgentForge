## 1. Dispatch attempt and queue data model expansion

- [x] 1.1 Extend `src-go` dispatch attempt models and persistence so attempts retain the canonical runtime tuple, queue linkage, trigger lineage, and machine-readable guardrail metadata required by the updated specs.
- [x] 1.2 Extend queue entry models and persistence so roster responses can surface latest guardrail verdict, recovery-vs-terminal promotion semantics, and promoted run linkage without relying only on free-form reasons.
- [x] 1.3 Add or update repository tests covering the new dispatch attempt and queue entry fields, including backward-compatible reads for existing records.

## 2. Go dispatch control-plane shaping

- [x] 2.1 Update `TaskDispatchService` so assignment-triggered dispatch writes rich attempt metadata and preserves canonical queued/blocked/skipped outcome fields for downstream consumers.
- [x] 2.2 Update `AgentService` manual spawn and queued promotion paths so promotion rechecks record the latest verdict and expose recoverable-vs-terminal queue outcomes truthfully.
- [x] 2.3 Add or tighten focused `src-go/internal/service` tests covering assignment dispatch, manual spawn, and queue promotion with the richer canonical metadata.

## 3. Operator-facing APIs and observability surfaces

- [x] 3.1 Update dispatch history and queue roster handlers/DTOs so operator-facing APIs return the richer attempt and queue truth required by the modified specs.
- [x] 3.2 Update any Go-side spawn or dispatch handlers that still collapse rich outcomes into thinner response payloads so synchronous callers get the same canonical metadata.
- [x] 3.3 Add or update focused handler tests for dispatch history, queue roster, and manual spawn responses to prove the richer machine-readable metadata survives the API layer.

## 4. IM consumer contract alignment

- [x] 4.1 Mirror the richer dispatch DTO in `src-im-bridge/client/agentforge.go`, including queue and guardrail metadata that already exists in Go responses.
- [x] 4.2 Update IM-facing dispatch reply formatting so queued, blocked, and skipped branches preserve the new canonical truth instead of degrading to generic text.
- [x] 4.3 Add or update focused `src-im-bridge` tests covering queued, blocked, and skipped dispatch outcomes with the mirrored metadata.

## 5. Focused verification and rollout proof

- [x] 5.1 Run focused `src-go` verification for the affected model, repository, service, and handler packages covering dispatch history, queue lifecycle, and manual spawn/promotion flows.
- [x] 5.2 Run focused `src-im-bridge` verification for the updated client and IM-facing dispatch formatters.
- [x] 5.3 Document any remaining out-of-scope repo noise separately so this change only claims the slices actually verified.
