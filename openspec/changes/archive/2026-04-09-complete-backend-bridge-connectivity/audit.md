## Backend Connectivity Audit

Date: 2026-04-09

### Canonical topology confirmed

- `src-im-bridge` does **not** call `src-bridge` directly.
- `src-im-bridge` calls Go backend APIs through `src-im-bridge/client/agentforge.go`.
- Go backend proxies runtime and lightweight AI capability calls to `src-bridge` through `src-go/internal/bridge/client.go`.
- Go backend routes progress / terminal / compatibility outbound IM delivery through `src-go/internal/service/im_control_plane.go` and `src-go/internal/service/im_service.go`.

### Files and flows audited

- `src-go/internal/bridge/client.go`
  - Canonical `/bridge/*` paths already in use.
  - Drift found: upstream non-200 failures did not consistently identify the bridge path.
- `src-go/internal/service/agent_service.go`
  - Spawn/resume already propagate `runtime` / `provider` / `model` / `team_id` / `team_role`.
  - Drift found: backend relied on bridge snapshot truth during resume but bridge did not reject conflicting resume context.
- `src-go/internal/service/im_service.go`
  - Control-plane-first send/notify path already exists.
  - Drift found: compatibility and queued deliveries did not consistently annotate delivery source / bridge binding lineage metadata.
- `src-go/internal/service/im_control_plane.go`
  - Bound progress already targets the stored `bridge_id`.
  - Drift found: queued bound progress/terminal deliveries were not recorded into delivery history with explicit source/binding metadata, which weakened operator truth.
- `src-go/internal/service/im_action_execution.go`
  - Action outcomes already preserve `replyTarget`.
  - Drift found: action results did not explicitly preserve bridge binding lineage in metadata.
- `src-im-bridge/client/agentforge.go`
  - Already uses Go proxy endpoints for `/api/v1/ai/*` and `/api/v1/bridge/*`.
  - Current code matches the intended topology.
- `src-bridge/src/server.ts`
  - Resume already blocks missing continuity.
  - Drift found: resume accepted conflicting request context instead of rejecting drift against the persisted snapshot.

### Implemented in this session

- Added source-aware IM connectivity metadata helpers in `src-go/internal/service/im_connectivity_metadata.go`.
- Added shared runtime-context comparison helpers in `src-go/internal/service/bridge_runtime_context.go` and wired spawn fallback status verification to ignore drifted runtime context.
- Tightened action result metadata, compatibility delivery metadata, and bound progress history recording.
- Tightened bridge resume flow to reject request context drift against the persisted snapshot.
- Tightened Go bridge client non-200 errors to preserve canonical bridge path details.
- Tightened IM command fallback/error phrasing so bridge-unavailable vs runtime-not-ready causes stay visible in decompose / runtimes / health paths.

### Remaining major seams

- `src-go/internal/service/agent_service.go`: runtime identity/status verification can still be tightened further on execute/status/resume reconciliation.
- `src-im-bridge/commands/*`: operator-facing fallback/error phrasing still needs a deeper pass for runtime-not-ready vs bridge-unavailable vs delivery-settlement distinctions.
- Broader cross-stack verification remains pending beyond the focused slices covered in this session.

### Focused verification completed

- `cd src-go && go test ./internal/bridge -run 'TestClientGetRuntimeCatalogUsesBridgeRoute|TestClientGetRuntimeCatalogPreservesUpstreamFailureDetails|TestClientHealthAndStatusUseBridgeRoutes' -count=1`
- `cd src-go && go test ./internal/service -run 'TestAgentService_PauseAndResumePreserveManagedRuntimeContext|TestAgentService_ResumeUsesPersistedRuntimeIdentity|TestBackendIMActionExecutor_AssignAgentUsesDispatchWorkflow|TestIMService_HandleActionPreservesReplyTargetAndMetadata|TestIMService_SendQueuesPendingDeliveryForControlPlane|TestIMControlPlane_QueueBoundProgressRecordsPendingDeliveryMetadata|TestIMControlPlane_QueueBoundProgressRecordsFailedSettlementWhenBoundBridgeIsMissing' -count=1`
- `cd src-go && go test ./internal/service -run 'TestDiffBridgeRuntimeContextDetectsMismatch|TestDiffBridgeRuntimeContextTreatsMatchingValuesAsStable' -count=1`
- `cd src-bridge && bun test src/server.test.ts --test-name-pattern 'declares canonical /bridge routes and compatibility-only aliases|compatibility aliases share canonical validation and response semantics|pauses a runtime into a resumable snapshot and resumes it with persisted continuity|rejects resume when the provided runtime context drifts from the persisted snapshot'`
- `cd src-bridge && bun test src/runtime/registry.test.ts --test-name-pattern 'publishes runtime catalog metadata and readiness diagnostics'`
- `cd src-im-bridge && go test ./client ./commands ./cmd/bridge -run 'TestBridgeRuntimeStatusMethods_ParseResponses|TestHandleIMAction_SendsCanonicalPayloadAndParsesReplyTarget|TestTaskCommand_DecomposeBridgeFirstWithProviderAndModel|TestTaskCommand_DecomposeFallsBackToLegacyAPIWhenBridgeUnavailable|TestTaskCommand_DecomposeFailureAfterBridgeAndFallbackExplainsNoSubtasksCreated|TestTaskCommand_DecomposeFallbackLabelsRuntimeNotReadyWhenBridgeCannotExecute|TestTaskCommand_DecomposeSkipsBridgeWhenCapabilityProbeFails|TestAgentCommand_RuntimesAndHealth|TestAgentCommand_RuntimesReportsRuntimeNotReadySource|TestBridgeRuntimeControl_StartProcessesDeliveriesAndStopsCleanly|TestBackendActionRelay_HandleAction_UsesRequestPlatformAndBridgeContext' -count=1`

### Verification boundary not yet covered

- No repo-wide `go test ./...` or full `bun test` pass was attempted; this session intentionally stayed on the connectivity seams changed above.
