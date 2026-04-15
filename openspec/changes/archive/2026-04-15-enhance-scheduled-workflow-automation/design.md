## Context

AgentForge 现在已经具备三段互相独立但没有闭环的能力：

- scheduler control plane 会定时触发 `automation-due-date-detector` 这类 built-in job，并把 run history、summary、metrics 记录到 canonical scheduler surfaces。
- automation rule engine 会在 `task.due_date_approaching`、`task.status_changed`、`budget.threshold_reached` 等事件上执行 action，但当前 action library 只覆盖轻量副作用与 generic `invoke_plugin`。
- workflow runtime 已经能通过 `WorkflowExecutionService.Start(...)` 创建 canonical `WorkflowPluginRun`，并保留 per-step status、trigger payload、pause state 与 run history。

因此真正的缺口不是“缺 scheduler”或“缺 workflow runtime”，而是 scheduler-backed automation 无法通过 canonical seam 启动正式 workflow run。更糟的是，`automation-due-date-detector` 当前 scheduler summary 只会报告扫描阈值，看不到 downstream orchestration 到底是否启动、被阻塞还是失败。

这条 change 需要补的是一条 focused follow-up seam：让 scheduler 继续负责产生命中的 due-date automation event，让 automation engine 负责 rule evaluation，但把 workflow orchestration 交给现有 workflow runtime，并把结果回流到 automation log 与 scheduler run metrics。

## Goals / Non-Goals

**Goals:**

- 为 automation rule engine 增加 canonical `start_workflow` action，而不是复用 generic `invoke_plugin` 去曲线启动 workflow。
- 让 scheduler-backed due-date automations 可以通过现有 workflow runtime 启动正式 `WorkflowPluginRun`，并自动携带 automation source、rule identity、project/task context。
- 为 automation-triggered workflow starts 输出结构化 verdict，至少覆盖 `started` / `blocked` / `failed`、reason code、plugin id、run id。
- 让 `automation-due-date-detector` 的 scheduler run summary/metrics 能反映 downstream orchestration 结果，而不只是“扫描过多少小时阈值”。
- 保持 API/UI 改动最小但真实可用，至少让 automation rule authoring surface 能配置 `start_workflow`，并让 automation log consumer 看见结构化 detail。

**Non-Goals:**

- 不重做 workflow manifest trigger profile；那条 seam 继续留给现有 `enhance-task-triggered-workflow-orchestration` change。
- 不把 automation engine 扩成新的通用 orchestration platform，也不引入新的 job queue / worker bus。
- 不重写 scheduler page 或 workflow page，只在现有 log/store/payload 里补足 truth。
- 不为 workflow starts 新建一套独立 automation history 表；继续复用 `automation_logs` 与 `WorkflowPluginRun`。

## Decisions

### Decision 1: `start_workflow` 走 dedicated workflow runtime seam，而不是 generic `invoke_plugin`

`invoke_plugin` 当前面向 generic plugin invocation，而且仓库里的 `PluginService.Invoke(...)` 只接受 integration-style invocation contract；workflow starters 的 canonical start seam 已经是 `WorkflowExecutionService.Start(...)`。因此 `start_workflow` 必须通过一个专门的 workflow starter interface 接到现有 workflow runtime，而不能继续把 workflow orchestration 伪装成 generic plugin invoke。

这样做的好处是：

- 直接复用现有 `WorkflowPluginRun` 持久化、step execution 和 dependency validation。
- 避免为 workflow action 再造一条没有 run history 的“旁路成功”。
- 与当前 active change 的 task-trigger design 保持一致，后续可以共享 helper，而不是产生第三套启动语义。

备选方案是扩展 `invoke_plugin`，允许它也启动 workflow plugin。这个方案会模糊 integration plugin 与 workflow runtime 的边界，也会让 automation 侧继续绕开 canonical workflow run persistence，因此不采用。

### Decision 2: automation workflow start payload 采用 canonical base trigger + additive custom trigger

`start_workflow` 的 action config 以 `pluginId` 为必填项，并允许携带 additive `trigger` payload。执行时 backend 会先构造一份 canonical base trigger：

- `source = automation_rule`
- `eventType`
- `ruleId`
- `projectId`
- `taskId` 与必要 task summary（若事件自带 task context）
- 事件特有字段，例如 due date threshold / due_at

然后再把 action config 里的自定义 `trigger` 做 additive merge。这样 workflow runtime 拿到的 trigger payload 永远有稳定的 automation lineage，同时又保留调用方补充上下文的空间。

备选方案是直接把 rule config 原样塞进 workflow runtime。这样虽然省事，但 downstream step 无法稳定拿到 `ruleId`、`eventType` 或 task identity，也不利于 duplicate guard 和 operator debugging，因此不采用。

### Decision 3: duplicate guard 放在 automation start seam，按 `pluginId + event scope` 做 truthful block

automation-triggered workflow start 的第一版不引入新的全局索引，而是复用已有 `WorkflowRunStore.ListByPluginID(...)` 做 narrow duplicate filtering。默认 duplicate scope 定义为：

- 有 task context 时：`pluginId + ruleId + taskId + eventType`
- 无 task context 时：`pluginId + ruleId + projectId + eventType`

如果已有相同 scope 的 active run（`pending` / `running` / `paused`），则本次 action 不创建第二个 run，而是返回 `blocked` verdict，并把 duplicate reason 写入 automation log detail。这样 scheduler/automation 能保持幂等 truth，而不会因为周期 job 重复命中导致 workflow run 噪音爆炸。

备选方案是不做 guard，把“重复 run 是否允许”留给 workflow plugin 自己处理。这个方案会让 due-date detector 等周期任务在短窗口内反复起同一 starter，不符合“定时任务全流程完整”的操作预期，因此不采用。

### Decision 4: scheduler summary 通过 due-date evaluation summary 反映 downstream orchestration 结果

当前 `AutomationDueDateChecker` 只返回 `error`，导致 `automation-due-date-detector` 的 scheduler run summary 只能记录阈值窗口。为了让 scheduler surface 真正反映全流程，本次会让 due-date evaluation 返回结构化 summary，例如：

- `evaluatedTasks`
- `matchedRules`
- `workflowStarts`
- `workflowBlocked`
- `workflowFailed`

`automation-due-date-detector` 再把这些字段写进 scheduler run metrics，并生成包含 downstream orchestration truth 的摘要文本。这样 operator 打开 scheduler page 就能直接分辨“本轮只是扫描过”还是“本轮确实拉起了 workflow / 被 duplicate guard 挡住 / 因配置错误失败”。

备选方案是保持 scheduler summary 不变，只让 operator 去 automation log 或 workflow run page 自己拼接。这个方案会让 scheduler control plane 继续只有局部真相，不符合这条 change 的目标，因此不采用。

### Decision 5: automation logs 保留现有顶层状态，但 detail 必须追加 action-level verdict

`automation_logs` 目前只有 `success` / `failed` / `skipped` 顶层状态。为了避免扩大数据库表结构，本次不新增状态列，而是在 `detail` JSON 中加入 action-level verdict 列表，至少包含：

- `type`
- `outcome`
- `reason`
- `pluginId`
- `runId`

顶层 rule status 的折叠规则保持简单：

- 至少一个 action hard-fail：`failed`
- 全部未执行或被前置条件跳过：`skipped`
- 其余情况：`success`

这允许 `start_workflow` 在 duplicate block 场景下保留 machine-readable truth，而不必为了一个 action outcome 就重做整个 automation log schema。

## Risks / Trade-offs

- [Risk] `start_workflow` action 与现有 active change 的 task-trigger `start_workflow` 出现 helper 重复 → Mitigation: 设计上强制复用 canonical workflow runtime seam，并在实现阶段优先抽 shared helper，而不是在 automation engine 内写第二套 workflow start 逻辑。
- [Risk] additive trigger merge 让 rule authors 覆盖 canonical lineage 字段 → Mitigation: canonical base trigger 字段在 merge 后仍以 backend authority 为准，调用方只能补充未保留字段。
- [Risk] duplicate filtering 只基于 `ListByPluginID` 可能在高并发场景下不够强 → Mitigation: 第一版先提供 truthful best-effort guard 和 focused tests；若后续需要强一致索引，再单开 change。
- [Risk] scheduler summary 过度依赖 automation engine 新返回值 → Mitigation: 保持 summary object additive，遇到老实现或部分失败时仍能退回 scan-only fields，不阻断 job completion。
- [Risk] automation UI 现在很轻，`start_workflow` 配置可能先只能暴露最小字段 → Mitigation: 本次至少保证 action type 可选、`pluginId` 可填、日志 detail 可见；复杂 authoring wizard 留待后续 change。

## Migration Plan

1. 先扩展 OpenSpec specs，锁定 `start_workflow` action、structured verdict 和 scheduler downstream metrics contract。
2. 在 Go backend 引入 dedicated automation workflow starter seam，并把 `automation-due-date-detector` 的 evaluation path 改成返回结构化 summary。
3. 更新 automation handler/store/rule editor 的最小 authoring contract，让新 action 可配置并在日志 consumer 中可见。
4. 通过 focused Go and frontend/store tests 验证 canonical workflow start、duplicate block 和 scheduler metrics truth。
5. 回滚策略：如果 workflow-start automation 行为不稳定，可以先撤掉 `start_workflow` action 的 authoring/validation exposure，并把 due-date summary 回退为 scan-only；现有 scheduler、automation 其他 action 和 workflow runtime 不需要整体回滚。

## Open Questions

- 第一版 `start_workflow` 是否只允许已安装启用的 workflow plugin，还是允许 built-in but not installed 的 starter id 直接启动？当前倾向前者，保持与现有 runtime truth 一致。
- 对没有 task context 的 automation event（例如 `budget.threshold_reached`），第一版是否同样开放 workflow start？当前倾向支持，但 duplicate scope 改按 `projectId + ruleId + eventType`。
- automation UI 是否在本次就需要专门的 workflow picker，还是先允许 `pluginId` 文本输入即可？当前倾向先做 truthful text input，不把范围扩成新的 selector surface。
