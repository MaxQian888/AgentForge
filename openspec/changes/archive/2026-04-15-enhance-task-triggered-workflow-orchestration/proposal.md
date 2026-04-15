## Why

AgentForge 现在已经有三块可以单独跑通的后端能力：project-level task transition triggers、official built-in workflow starters、以及可持久化的 workflow runtime。但这三块还没有在“任务状态变化驱动正式编排”这件事上接成一条 canonical control-plane。当前 `TaskWorkflowService` 只支持 `notify` / `auto_assign_agent` / `auto_transition` 这类轻量动作，前端 trigger action vocabulary 又和后端存在漂移，而 `task-delivery-flow`、`review-escalation-flow` 这样的成功 starter 仍然只能手动启动，导致“后端任务编排”仍停留在零散自动化而不是正式 workflow orchestration。

现在补这条线，是因为仓库已经具备可复用的成功案例和稳定 seam：official starter library、workflow runtime、dispatch truth、team/template workflow 启动器都在位。如果继续让 task workflow 只做 fire-and-forget 小动作，后续 task page、IM、scheduler、starter catalog 和 operator workflow 都会继续各自拼自己的 orchestration 入口，无法形成统一的后端真相。

## What Changes

- 扩展 project task workflow trigger control-plane，让任务状态迁移可以启动 canonical workflow run，而不是只支持通知、自动分配和自动状态流转。
- 对齐 trigger action vocabulary 与 config contract，消除当前 workflow config panel/store 与 Go backend trigger engine 之间的动作名漂移，并明确哪些 action 属于 dispatch、哪些属于 workflow starter activation。
- 为 task-triggered workflow execution 引入结构化 outcome 和 lineage，至少保留 trigger source、matched rule、started/skipped/blocked/failed 状态、关联 workflow run / starter id、以及重复触发或依赖缺失时的 machine-readable reason。
- 让 official built-in workflow starters 可以声明并兑现真实可执行的 task-driven trigger profiles，优先覆盖 `task-delivery-flow`，并对 `review-escalation-flow` 这类 starter 保持 truthful availability 而不是暗示已支持。
- 保持范围聚焦在 Go backend task orchestration control-plane、workflow runtime contract、starter manifest truth 和紧耦合 consumer contract；本次不重写 visual workflow editor，也不重做整个 workflow engine。

## Capabilities

### New Capabilities
- `task-triggered-workflow-automation`: 定义 project-level task transition triggers 如何启动 canonical workflow runs、保留结构化 trigger outcome、并在重复触发或依赖不满足时返回真实的 orchestration verdict。

### Modified Capabilities
- `workflow-plugin-runtime`: built-in workflow starters 的 task-driven trigger profiles 需要通过 canonical task workflow control-plane 变成真实可执行入口，并在 unsupported / duplicate / dependency-missing 场景下返回稳定的 dependency or availability truth。
- `built-in-workflow-starters`: 官方 starter library 需要声明哪些 starter 支持 task-driven activation、需要什么 task context、以及哪些 starter 仍然只支持 manual trigger，避免 starter catalog 和真实 control-plane 能力漂移。

## Impact

- **Go task workflow control-plane**: `src-go/internal/service/task_workflow_service.go`, `src-go/internal/model/workflow.go`, `src-go/internal/handler/task_handler.go`, 以及相关 tests / realtime event payloads。
- **Workflow runtime and starter execution**: `src-go/internal/service/workflow_execution_service.go`, `src-go/internal/service/workflow_step_router.go`, workflow/plugin start surfaces, and any shared helper needed to prevent duplicate task-triggered runs.
- **Starter manifests and bundle truth**: `plugins/workflows/task-delivery-flow/manifest.yaml`, `plugins/workflows/review-escalation-flow/manifest.yaml`, plus any metadata or validation path that currently only treats them as manual starters.
- **Consumer contracts**: workflow config DTOs, task/workflow API docs, and workflow config consumers such as `lib/stores/workflow-store.ts` / `components/workflow/workflow-config-panel.tsx` that currently expose drifted trigger action names.
