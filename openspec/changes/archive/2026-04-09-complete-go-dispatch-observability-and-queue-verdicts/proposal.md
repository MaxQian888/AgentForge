## Why

Go 派发器的 started / queued / blocked / skipped 主链路已经跑通，但它对外暴露的 truth 仍然偏薄：dispatch history 只保留最基础 outcome，queue entry 也缺少 latest guardrail verdict 与 promotion recheck 语义，导致 Web、IM 和 operator 面拿到的只是“发生了什么”，而不是“为什么这样发生、现在还能不能恢复”。现在前端、IM 与运维表面都已经在消费这些 dispatch 结果，如果不把 Go 侧的留痕与队列 verdict 收紧成权威合同，后续每个入口都会继续靠 reason 字符串和局部上下文各自猜一套。

## What Changes

- 补齐 Go dispatch observability contract，让 dispatch attempt 不再只记录 outcome/reason，而是保留 runtime tuple、queue context、trigger lineage，以及支撑 operator 诊断的 machine-readable metadata。
- 补齐 queue lifecycle truthfulness，让 queue roster 与 promotion recheck 能区分“仍在排队等待恢复”和“已经终态失败”，并保留最新 guardrail verdict，而不是只剩一个 reason 字符串。
- 对齐 assignment dispatch、manual spawn、queued promotion 的结构化非启动结果，使 queued / blocked / skipped 在同步 API、dispatch history、queue roster 和 IM 消费面保持同一份 canonical metadata。
- 收紧 IM-facing dispatch DTO 与 reply formatting，使 Go 已有的 queue / guardrail / budget metadata 不再在 `src-im-bridge` 客户端层被削薄或折叠。
- 保持范围聚焦在 Go dispatch control-plane 及其紧耦合 consumer contract；本次不扩展到 Go 去消费 TS Bridge 全量新 live-control/capability matrix，也不重做前端界面。

## Capabilities

### New Capabilities
<!-- None. -->

### Modified Capabilities
- `agent-task-dispatch`: dispatch history、queued/blocked/skipped outcome、以及 IM-facing assignment result 需要保留更完整的 machine-readable dispatch metadata，而不是只暴露基础 reason。
- `agent-pool-control-plane`: queue roster 与 promotion lifecycle 需要保留 runtime tuple、latest guardrail verdict、以及 recoverable-vs-terminal queue semantics。
- `agent-spawn-orchestration`: manual spawn 与 queued promotion 需要输出和 assignment dispatch 一致的结构化 dispatch truth，而不是只返回简化的 started/queued 文案。

## Impact

- **Go models / repositories**: `src-go/internal/model/dispatch_attempt.go`, `src-go/internal/model/task_dispatch.go`, `src-go/internal/model/agent_pool.go`, `src-go/internal/repository/dispatch_attempt_repo.go`, `src-go/internal/repository/agent_pool_queue_repo.go`.
- **Go services / handlers**: `src-go/internal/service/task_dispatch_service.go`, `src-go/internal/service/agent_service.go`, `src-go/internal/handler/dispatch_observability_handler.go`, `src-go/internal/handler/queue_management_handler.go`, `src-go/internal/handler/agent_handler.go`, `src-go/internal/service/im_action_execution.go`.
- **Consumer contracts**: `src-im-bridge/client/agentforge.go` and IM command/action formatting that currently lose queue or guardrail metadata.
- **Operator-facing APIs**: dispatch history, queue roster, manual spawn responses, and IM-visible dispatch summaries will gain richer, machine-readable truth without changing the overall topology.
