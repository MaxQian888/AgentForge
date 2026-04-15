## Why

AgentForge 的 scheduler control plane、automation rule engine、以及 canonical workflow runtime 现在都各自可用，但它们还没有形成“定时任务命中后可以稳定拉起正式 workflow run”的闭环。当前 `task.due_date_approaching` 之类的 scheduler-backed automation 只能执行轻量 action，而不能通过标准 workflow run surface 启动 `WorkflowPlugin` starter，这让“定时任务全流程”仍停在半连通状态。

现在补这条线，是因为仓库已经有成熟的 workflow runtime、starter library 和 scheduler/automation 接缝；继续让定时能力只做通知或字段修改，会迫使后续运维和业务流程继续手工兜底，也会让 scheduler page、automation log、workflow run history 各自只持有局部真相。

## What Changes

- 为 automation rule engine 增加 canonical `start_workflow` action，让 `task.due_date_approaching` 等 automation 事件可以通过现有 workflow runtime 启动正式 `WorkflowPluginRun`，而不是滥用 generic plugin invocation。
- 为 automation-triggered workflow starts 增加结构化 execution verdict，至少保留 `started` / `blocked` / `failed` 等 outcome、machine-readable reason、目标 workflow plugin id、以及 started run id，避免 automation log 只能留下模糊成功或失败文本。
- 在 scheduler-backed due-date detection 路径上汇总 downstream automation/workflow outcome，把 scheduler run summary 和 metrics 从“仅表示扫描过”提升为“能反映本轮评估中启动了多少 workflow、跳过了多少、失败了多少”的 truthful operator signal。
- 保持范围聚焦在 Go backend automation/scheduler/workflow seams、automation log truth 和现有 scheduler observability surface；本次不重做 visual workflow editor、不扩成任意自定义 cron 创作器，也不引入新的通用 job bus。

## Capabilities

### New Capabilities

### Modified Capabilities
- `automation-rule-engine`: automation actions 需要支持 canonical `start_workflow`，并为 automation-triggered workflow orchestration 返回结构化执行结果与日志真相。
- `background-scheduler-control-plane`: scheduler-backed automation jobs 需要在 run summary/metrics 中保留下游 automation/workflow outcome，而不只是报告扫描阈值或轮询完成。

## Impact

- **Automation control-plane**: `src-go/internal/service/automation_engine_service.go`, `src-go/internal/model/automation.go`, automation handler/service tests，以及任何校验 automation action contract 的 API / DTO surface。
- **Workflow runtime integration**: `src-go/internal/service/workflow_execution_service.go`, workflow run repositories/readers, and shared helpers used to start workflow runs from internal services without bypassing canonical persistence.
- **Scheduler-backed due-date path**: `src-go/internal/scheduler/builtin_handlers.go`, `src-go/internal/service/automation_engine_service.go`, and any scheduler metrics/summary shaping consumed by the existing scheduler API/UI.
- **Operator observability**: automation logs, scheduler run history payloads, and adjacent docs/tests that currently only describe lightweight automation actions or scan-only due-date summaries.
