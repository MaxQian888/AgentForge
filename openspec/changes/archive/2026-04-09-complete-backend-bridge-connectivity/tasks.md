## 1. Canonical backend connectivity contract

- [x] 1.1 Audit the current `src-go` ↔ `src-bridge` ↔ `src-im-bridge` runtime and delivery seams against the new specs, and mark the exact files and flows that still drift from the canonical topology.
- [x] 1.2 Update backend-facing documentation/comments/spec-adjacent source of truth so they consistently state that Go backend is the mediator between TS Bridge and IM Bridge.
- [x] 1.3 Add or tighten shared DTO/helper seams in `src-go` for preserving runtime context, bridge binding, reply-target lineage, and delivery source metadata across backend hops.

## 2. Go backend to TS Bridge completeness

- [x] 2.1 Tighten `src-go/internal/bridge/client.go` and related handlers/services so canonical `/bridge/*` and `/api/v1/ai/*` proxy paths preserve runtime/provider/model and upstream failure details truthfully.
- [x] 2.2 Update `src-go/internal/service/agent_service.go` and related backend execution paths so execute/status/resume flows preserve runtime identity and required execution context for external runtimes.
- [x] 2.3 Tighten `src-bridge` request/status/diagnostic seams so backend-consumed runtime metadata stays stable and rejects context drift during resume or diagnostics flows.
- [x] 2.4 Add or update focused tests covering Go proxy behavior, upstream failure propagation, and runtime identity preservation across execute/status/resume.

## 3. Go backend to IM Bridge control-plane completeness

- [x] 3.1 Tighten `src-go/internal/service/im_control_plane.go` so bound progress and terminal deliveries stay targeted to the originating bridge instance and expose stale/unavailable binding outcomes truthfully.
- [x] 3.2 Update `src-go/internal/service/im_service.go`, `task_progress_service.go`, and related handlers so delivery source, reply-target lineage, and settlement state remain explicit across compatibility send/notify, progress, and terminal paths.
- [x] 3.3 Add or update focused `src-go` tests for registration, targeted delivery, replay/ack, delivery source metadata, and bound-instance failure handling.

## 4. IM Bridge capability routing and action lineage

- [x] 4.1 Tighten `src-im-bridge/client/agentforge.go` and command routing so Bridge-backed capabilities always use Go proxy endpoints and backend-native workflows bypass TS Bridge intentionally.
- [x] 4.2 Update `src-go/internal/service/im_action_execution.go` and related result contracts so IM actions preserve bridge binding and reply-target-aware terminal context.
- [x] 4.3 Update `src-im-bridge` action/command handling to surface bridge-unavailable, runtime-not-ready, fallback, and delivery-settlement failures with source-aware responses.
- [x] 4.4 Add or update focused `src-im-bridge` tests covering capability routing, action lineage preservation, and truthful fallback/error messaging.

## 5. Cross-stack verification

- [x] 5.1 Run focused verification for the affected `src-go` packages covering bridge client, agent service, IM control plane, IM service, and IM action execution.
- [x] 5.2 Run focused verification for affected `src-bridge` tests covering runtime status/diagnostics, canonical route behavior, and proxy-facing schemas.
- [x] 5.3 Run focused verification for affected `src-im-bridge` tests covering startup binding, capability routing, and control-plane delivery handling.
- [x] 5.4 Record any remaining repo-wide failures that are outside this change, and ensure the change artifacts only claim the slices that were actually verified.
