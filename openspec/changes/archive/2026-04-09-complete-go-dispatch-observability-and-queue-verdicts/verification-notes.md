## Verification Notes

### Focused verification completed in this change

- `go test ./internal/model ./internal/repository ./migrations` from `src-go`
- `go test ./internal/service -run "TaskDispatchService|RequestSpawnQueuesWhen|RequestSpawnBlocksWhenBridgeHealthCheckFailsAfterRetries|UpdateStatusPromotesQueuedAdmissionAfterTerminalRelease|UpdateStatusRequeuesPromotionWhenBudgetCheckBlocks|CancelQueueEntry|ListQueueEntriesSortsAndFiltersStatuses"` from `src-go`
- `go test ./internal/handler -run "AgentHandler_Spawn_ReturnsAcceptedWhenDispatcherQueuesAdmission|DispatchHistoryHandler|QueueManagementHandler"` from `src-go`
- `go test ./client ./commands -run "AssignTask|SpawnAgent|FormatAgentSpawnReply|formatTaskDispatchReply|FormatTaskDispatchReply"` from `src-im-bridge`

### Out-of-scope repo noise

A full `go test ./internal/service` run in `src-go` still fails outside this change on:

- `TestVerifySpawnStartedFallsBackToGetStatusWithoutBridgeActivity`
- failure mode: panic / nil pointer dereference in `AgentService.verifySpawnStarted(...)`
- stack anchor observed at `src-go/internal/service/agent_service.go:2235`

This change does **not** claim that broader `internal/service` package issue as fixed. The scoped verification above is the evidence boundary for this rollout.
