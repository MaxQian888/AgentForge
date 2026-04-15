## Context

AgentForge 当前已经有几条彼此接近但还没闭合的 task/IM seam：

- `src-im-bridge/commands/task.go` 已经提供 `/task create|list|status|assign|decompose|move|delete`，并在部分响应里生成 task card 或 action reference。
- `src-go/internal/service/im_action_execution.go` 已有 shared IM action execution，但 task 侧目前主要覆盖 `assign-agent`、`decompose`、`save-as-doc`、`create-task`；task transition 这类关键 lifecycle action 还没有 canonical callback action contract。
- `src-go/internal/service/im_control_plane.go` 和 `QueueBoundProgress(...)` 已经能把绑定到 `taskId/runId/reviewId` 的 progress / terminal delivery 送回原 `replyTarget`，并保留 provider-aware metadata、fallback reason、以及 event gating。
- `src-go/internal/service/task_workflow_service.go` 与 `src-go/internal/handler/task_handler.go` 已能在任务流转后触发 workflow evaluation、progress activity 和 websocket event，但 task transition / workflow trigger 结果仍主要停留在 web activity、notification 或 ws consumer，没有稳定的 task-originated IM follow-up contract。
- Feishu callback / rich card lifecycle 已经在 `src-im-bridge/platform/feishu/*` 成型，说明 provider-native task card interaction 可以复用现有 callback readiness、delayed update、以及 fallback 逻辑，而不需要新增 transport。

这次 change 是 cross-cutting 的：它同时穿过 IM command/catalog、backend action execution、reply-target binding、task transition orchestration、以及 provider-aware card rendering。没有 design 的话，很容易把“task 能在 IM 里闭环”误做成新的 task-only transport、另一个 IM action 协议，或者把 task workspace UI 一起卷进来。

## Goals / Non-Goals

**Goals:**

- 让 `/task` command family 与 task card interaction 成为真实可执行的 task lifecycle surface，而不是只做浅层查询和零散按钮。
- 为 IM callback / card action 建立 canonical task lifecycle action contract，优先覆盖 task status transition，并让结果保持 truthful outcome。
- 让 IM-originated task transition 与其后续 task/workflow verdict 能通过现有 bound delivery seam 回到原 conversation/thread/callback context。
- 复用现有 `im-action-execution`、`im-control-plane`、Feishu callback lifecycle、typed rich delivery 等 seam，而不是再造一套 task-specific transport。
- 保持 scope focused：把问题收敛在 task/IM lifecycle 本身，不阻塞 task workspace、workflow engine、或全平台 operator console 的独立演进。

**Non-Goals:**

- 不重做 `ProjectTaskWorkspace`、task detail rail、或 project dashboard UI。
- 不把本次 change 扩成通用 IM automation platform，也不重写 `TaskWorkflowService` 成新的 workflow engine。
- 不新增第二套 callback / delivery transport；所有 task follow-up 继续复用现有 `/api/v1/im/action`、IM control-plane、compat notify/send、以及 typed outbound envelope。
- 不要求所有平台一次性具备同等 rich task card 能力；provider-native affordance 仍按各平台 readiness tier 和 callback readiness truthfully gating。

## Decisions

### Decision 1: 新 capability `task-im-lifecycle` 负责 task-specific contract，shared IM specs 继续只描述横切 seam

这次 change 的核心不是再扩一条 generic IM abstraction，而是定义“task 在 IM 中怎么被查看、推进、回执”。因此新增 `task-im-lifecycle` capability，用来承接 task-specific command response、task card affordance、reply-target binding 和 task/workflow follow-up。

已有 capability 继续保持原职能：

- `im-action-execution` 负责共享 callback/action 如何执行 canonical backend workflow
- `im-bridge-progress-streaming` 负责长流程回流与 bound follow-up delivery
- `im-operator-command-surface` 负责 slash command catalog / help / usage contract

**Alternative considered:** 只在现有 IM specs 里零散加 requirement，不新增 capability。  
**Rejected because:** task-specific lifecycle 会被拆散到多个 shared seam 中，后续很难判断“task IM 到底该做到什么”。

### Decision 2: 继续使用 `/api/v1/im/action` 作为唯一 callback-backed task action execution seam

task card / callback action 不直接调用 task handler，也不走 provider-specific side channel。所有 callback-backed task action 继续通过 `/api/v1/im/action` 进入 backend shared action executor，再由 executor 调用 task transition / dispatch / decomposition 等 canonical backend flow。

本次新增的 canonical task lifecycle action 以 `transition-task` 为 backend action 名，`/task move` 继续作为 operator command surface。这样可以让：

- 命令词与 backend action contract 解耦
- provider card / callback action 不需要复用 slash command 文本
- legacy action naming 可以在 backend 统一兼容，而不是让 provider payload 直接耦合 command spellings

**Alternative considered:** 让 task card action 直接拼 `/task move` 文本并交给 command parser。  
**Rejected because:** callback 与 slash command 是两种契约层次，把 provider action 回退成字符串命令只会继续固化解析漂移和 usage 不一致。

### Decision 3: IM-originated task transition 通过 task-scoped binding 复用现有 bound progress seam

这次不再为 task transition 新建 delivery store 或新 event bus。IM 发起的 task lifecycle action 在 backend 接受后，继续通过现有 `BindAction` / `ReplyTarget` / `QueueBoundProgress(...)` 这条链路保留 task-scoped binding。

关键点是把 task lifecycle follow-up 明确分为两层：

- **Immediate result**: 任务流转动作本身是否接受、是否更新状态、是否被 block
- **Asynchronous follow-up**: 由该流转触发的 task-triggered workflow / progress / terminal verdict

前者由 `im-action-execution` 直接返回；后者通过 bound progress / terminal delivery 回到原 conversation。这样既不让 task transition API 被同步长链路拖住，也不让 IM 侧因为只拿到初始 ack 而丢失后续 orchestration truth。

**Alternative considered:** task transition 一律同步等待 workflow trigger 结束后再返回 IM 完成结果。  
**Rejected because:** 会扩大 task handler 的失败面和延迟，也会与现有异步 `TaskWorkflowService` 路径冲突。

### Decision 4: task follow-up 只在“有绑定上下文或显式 routing target”时回 IM，不能伪造目标

task transition 或 workflow outcome 不能因为“系统里有某个平台在线”就任意发到别处。只有在以下任一条件成立时才允许回 IM：

- 当前动作来自 IM，且保留了 `ReplyTarget`
- 当前 task 已有 live task-scoped binding，可被 `taskId` 解析
- 有显式 project/channel routing rule，并且这是一个非-bound compatibility notify/send 场景

否则结果必须 truthfully 表现为 web-only、blocked delivery、或 no-bound-target，而不是偷发到默认群。

**Alternative considered:** 没有 binding 时自动回退到某个项目默认 IM channel。  
**Rejected because:** 这会破坏已有 `im-control-plane` 的 source-of-truth，用户也无法推断结果会发到哪里。

### Decision 5: provider-aware task card affordance 由 provider 层决定是否可交互，不由 shared layer 强行承诺

task summary / task detail 响应可以生成 typed structured payload，但“有没有 callback-backed按钮、能不能原地更新、是否只给 manual guidance”必须由 active provider profile 和 runtime readiness 决定。

这意味着：

- card-capable provider 可以渲染 task summary card 和 follow-up buttons
- Feishu 只有在 callback-ready 时才展示 callback-backed task actions
- 非 callback-ready 或 text-first provider 仍需输出任务标识、状态和可执行的 manual commands

shared layer 只表达“想做什么 action”；最终是否能安全落地为 native button/card，由 provider 负责选择并附带 fallback reason。

**Alternative considered:** 统一要求所有 provider 都输出同样的 task action card。  
**Rejected because:** 当前各平台 callback / mutable update 能力差异真实存在，统一假设只会制造虚假 affordance。

## Risks / Trade-offs

- **[Risk] `transition-task` 与现有 `/task move`、旧 action spellings 可能继续漂移** → **Mitigation:** command surface 文档只保留 `/task move` 作为 CLI canonical name，backend action contract 则统一到 `transition-task` 并允许兼容 alias。
- **[Risk] task transition 的 immediate result 与后续 workflow verdict 容易被用户误认为一回事** → **Mitigation:** 在 action result / terminal follow-up 中明确区分“状态已更新”与“后续 workflow started|blocked|failed”。
- **[Risk] 绑定 taskId 的 IM follow-up 可能在没有 live bridge 时失败** → **Mitigation:** 继续复用 control-plane blocked delivery / pending delivery truth，不允许静默换目标。
- **[Risk] provider-native task card 实现容易外溢成跨平台重构** → **Mitigation:** 共享层只扩 typed task interaction intent；具体 button/card 构造限定在 provider-aware rendering seam。
- **[Risk] 正在进行的 task-triggered workflow orchestration change 也会触碰相邻代码** → **Mitigation:** 本 change 只消费它的 outward outcome seam，不抢占其 starter/runtime 设计主线；必要时以 adapter/metadata 对齐而不是改写其 change scope。

## Migration Plan

1. 先补 OpenSpec delta specs，锁定 task IM lifecycle、task transition action contract、以及 bound follow-up 语义。
2. 在 `src-go` 中扩展 shared IM action executor 与 binding flow，补 `transition-task` canonical action 和 reply-target-aware result shaping。
3. 在 `src-im-bridge` 中对齐 `/task` command catalog、task card action reference、以及 provider-aware task affordance gating。
4. 把 task transition / workflow verdict 接到现有 bound progress / terminal delivery seam，并补 structured/native fallback metadata。
5. 用 scoped verification 跑 IM command / callback / task transition / bound follow-up / provider card tests；如果 follow-up routing 不稳定，可保留 command-side action result，同时先把 richer task follow-up 降到 manual guidance，而不回退整个 action execution 改动。

## Open Questions

- `transition-task` 是否要公开为唯一 callback action 名，还是保留 `move-task` 作为 legacy alias？当前倾向：callback/action contract 统一 `transition-task`，兼容读取 `move-task`。
- 哪些 task 状态应被渲染为 primary button，哪些只保留为 manual command？当前倾向：只暴露当前状态下最常见且低风险的下一步流转，不把所有状态全塞成按钮。
- destructive action（如 `delete-task`）是否应该进入 interactive card，还是继续只保留 slash command？当前倾向：本次先保留 slash command-only，避免 provider callback 里的误操作面过大。
