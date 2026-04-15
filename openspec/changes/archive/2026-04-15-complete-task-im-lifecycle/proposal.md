## Why

AgentForge 现在已经有可用的 task workspace、IM Bridge 命令入口、shared IM action execution，以及可复用的 reply-target / control-plane delivery seam，但“任务功能在 IM 里闭环”仍然没有完成。当前 IM 侧虽然能执行 `/task create|assign|decompose|move` 等动作，任务状态变化、task-triggered workflow 结果、以及 task-centric 卡片交互却还没有稳定地回到原会话，导致任务与 IM 仍像两块并列能力，而不是一条完整协作链路。

现在补这条线是合适的，因为仓库已经具备稳定基础：`src-im-bridge` 已有 task command 与 provider-native card 能力，Go backend 已有 `im-action-execution`、`im-bridge-progress-streaming`、`im-control-plane`、以及正在推进的 task-triggered workflow outcome contract。如果继续让 task IM surface 只停留在“能发几个命令”，后续 task orchestration、workflow starter、Feishu callback card、以及 operator help/catalog 仍会继续漂移。

## What Changes

- 完善 task-oriented IM command surface，让 `/task` 在现有 create/list/status/assign/decompose/move 基础上具备更完整、truthful 的任务协作入口，并让帮助/usage/catalog 与真实能力保持一致。
- 扩展 backend IM action execution，使 task card 或 callback action 不只支持 assign/decompose/save-as-doc/create-task，还能覆盖关键的 task lifecycle action，并返回结构化 task/workflow outcome 与 reply-target lineage。
- 让 IM 发起的 task transition、task-triggered workflow、以及相关异步结果能够通过现有 IM control-plane 回到原 conversation / thread / callback context，而不是只留在 web notification 或 websocket activity 中。
- 为 task card / structured delivery 补齐 provider-aware task interaction contract，优先复用现有 Feishu callback / native card lifecycle 和 canonical typed delivery，而不是新增平行 transport。
- 保持范围聚焦在 task/IM control-plane、command catalog、reply-target binding、以及 task lifecycle follow-up；本次不重做 project task workspace，不扩成全平台 operator console 大重构，也不重写整个 workflow engine。

## Capabilities

### New Capabilities
- `task-im-lifecycle`: 定义任务功能在 IM 中的 canonical lifecycle，包括 task commands、task card actions、reply-target binding、以及 task/workflow result 回到原会话的 contract。

### Modified Capabilities
- `im-action-execution`: 共享 IM action execution 需要支持更完整的 task lifecycle action，并返回带 task/workflow lineage 的 canonical outcome。
- `im-bridge-progress-streaming`: 绑定到 IM 的异步 follow-up 不再只覆盖 agent/decompose progress；task transition 与 task-triggered workflow result 也需要通过同一 reply-target-aware delivery contract 回流。
- `im-operator-command-surface`: `/task` command family 的 canonical catalog、usage、aliases、以及 help discoverability 需要与真实 task lifecycle 能力对齐。

## Impact

- **IM Bridge command and interaction surface**: `src-im-bridge/commands/task.go`, task help/catalog wiring, action references, and provider-aware task card rendering/dispatch.
- **Backend IM action and binding flow**: `src-go/internal/service/im_action_execution.go`, `src-go/internal/handler/im_control_handler.go`, `src-go/internal/service/im_control_plane.go`, and related task/reply-target binding helpers.
- **Task transition and workflow follow-up path**: `src-go/internal/handler/task_handler.go`, `src-go/internal/service/task_workflow_service.go`, task progress / bound progress delivery seams, and any workflow outcome payloads that need IM-facing lineage.
- **Docs and consumer contracts**: OpenSpec task/IM specs, operator help/catalog docs, and any task/workflow activity consumers that currently assume task-triggered outcomes are web-only.
