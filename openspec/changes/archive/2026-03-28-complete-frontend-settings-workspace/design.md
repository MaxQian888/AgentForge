## Context

`app/(dashboard)/settings/page.tsx` 现在已经承载 General、Repository、Coding Agent、Budget、Review Policy、Webhook、Custom Fields、Forms、Automations 等多个设置区块，但它本质上仍是一个直接拼接本地 `useState` 的大表单。当前实现可以提交设置数据，却缺少完整的前端工作区语义：

- 没有统一 draft baseline，用户改动后看不到 dirty 状态，也没有 discard/reset。
- `handleSave()` 只做一次提交并显示短暂成功提示，没有 pending 态、失败态和字段级错误反馈。
- 预算、review policy、runtime/provider、webhook 等输入没有形成前端可见的约束与错误展示。
- diagnostics 区块主要重复 runtime availability badge，没有把当前草稿、持久化值、默认回退值和阻塞原因组织成真正的 operator summary。
- 现有 `app/(dashboard)/settings/page.test.tsx` 只覆盖 runtime 保存 happy path，无法保护 legacy fallback、invalid input、discard/reset 和 save error 这些关键行为。

仓库里已经归档了 `project-settings-control-plane` capability，因此这次不再重开数据模型或后端控制面，只补齐前端 settings workspace 的交互闭环，并保持现有 `/settings` 路由与项目更新接口。

## Goals / Non-Goals

**Goals:**

- 为项目设置页建立明确的 draft lifecycle：初始值、dirty state、save pending、save success、save failure、discard/reset。
- 在前端把关键输入约束和服务器返回的失败信息变成可操作的 UI 反馈，而不是提交后静默失败。
- 将 operator diagnostics 改成基于当前设置状态的真实摘要，能区分当前草稿值、持久化值、legacy fallback 和 runtime blocking diagnostics。
- 用 focused tests 锁住 settings workspace 的关键行为，避免后续继续退化成“只有保存 payload 测试”。

**Non-Goals:**

- 不新增 settings 子路由、wizard、modal 流程或新的全局表单框架。
- 不在本次 change 中重做 custom fields、forms、automations 的内部功能实现；这些模块仍作为 settings workspace 的嵌入区块存在。
- 不修改现有项目设置的持久化结构，也不新增数据库迁移。
- 不把设置页扩成组织级设置中心或通知中心。

## Decisions

### Decision: Keep `/settings` as one page but introduce an explicit draft model

设置页继续保留单页工作区和统一保存入口，但前端状态从“散落的 `useState` 集合”提升为带 baseline 的 draft model。页面需要至少维护三类派生信息：

- `draft`: 当前可编辑值。
- `persistedSnapshot`: 最近一次成功加载或保存后的服务端真值。
- `dirty/validation/submitState`: 基于两者比较得出的编辑状态、错误状态和提交状态。

这样可以在不改 URL 和不引入新 store 的前提下实现 discard/reset、unsaved indication 和失败后保留草稿。替代方案是把整页迁到全局 Zustand store；这会放大 settings 局部状态的耦合面，不利于 focused change，因此不采用。

### Decision: Validation is layered as local guards plus server-error projection

前端先对明显可判定的约束做同步校验，例如：

- 预算数值必须是非负数，告警阈值必须在支持范围内。
- 当 webhook 处于启用态时，必填 URL 和至少一个事件。
- runtime/provider 组合必须来自当前 runtime catalog 的兼容集合。

提交后如果后端仍返回验证失败，`lib/stores/project-store.ts` 需要把错误继续抛给页面，由页面映射到 form-level 或 field-level feedback，而不是吞掉异常。替代方案是仅保留后端校验；这无法满足“提交前可见”和“失败后可定位”的 settings 工作区要求，因此不采用。

### Decision: Operator diagnostics becomes a derived summary over the current draft

diagnostics 区块不再只是重复 runtime 卡片，而是由当前 draft 与 persisted snapshot 共同派生：

- coding-agent readiness：显示当前 runtime/provider/model 是否可运行，以及阻塞诊断。
- governance posture：显示预算阈值、自动停止、review escalation、人工审批等当前姿态。
- fallback/default indicators：对 legacy project 注入的默认值或当前未保存草稿给出明确文案。
- integration readiness：对 webhook 的启用状态、事件订阅完整度与缺失项给出摘要。

这样 diagnostics 才能成为“保存前可判断、保存后可复核”的可信表面。替代方案是继续保留静态 badge 列表；信息量不足，不采用。

### Decision: Tests focus on workspace behaviors instead of only payload snapshots

设置页测试需要覆盖真实交互闭环，包括：

- legacy/fallback 项目打开后的默认值与默认态说明；
- 编辑后 dirty state 与 discard/reset；
- invalid input 阻止保存并显示错误；
- 服务端 save failure 保留草稿并显示失败提示；
- successful save 更新 persisted snapshot 并清除 dirty state；
- diagnostics 会随当前 draft 改变而更新。

这比只断言 `updateProject` payload 更贴合本次 change 的风险点，也更能保护 settings 工作区体验。

## Risks / Trade-offs

- [大表单继续集中在单文件可能增加维护成本] → 通过提取 draft/validation/summary helper 或局部 section 组件降低页面复杂度，但不在这次 change 中强行拆成全新架构。
- [前端与后端验证规则可能出现重复或漂移] → 只在前端实现明显且稳定的输入约束，把最终真值仍交给后端，并保留服务器错误投影路径。
- [嵌入式 child editors 也可能产生“未保存”认知混淆] → 明确本次 dirty/reset 仅针对 project settings 主表单；custom fields/forms/automations 仍沿用各自保存模型，并在文案或布局上区分。
- [Diagnostics 基于 draft 派生后可能和 persisted 值同时存在] → 在摘要里显式标明 `draft` / `saved` / `defaulted` 状态，避免用户误判当前是否已落库。

## Migration Plan

1. 先为 settings 页面整理 draft schema、validation helper 和 diagnostics summary helper，并补 store 的错误透传能力。
2. 在不改 API 契约的前提下重构 `app/(dashboard)/settings/page.tsx`，补齐 dirty/reset、pending/error/success 反馈和 diagnostics。
3. 补 focused tests，覆盖 invalid/save-failure/discard 等关键场景。
4. 以 settings 页面相关测试作为主验证面，必要时补一次 scoped lint/typecheck。

回滚策略：

- 如果新的前端工作区行为引入回归，可以回退到当前单页表单实现，因为本次不涉及数据迁移和接口变更。
- 如果错误投影导致页面无法保存，可临时保留 store 抛错路径并降级页面侧字段映射，不会影响后端已有 settings 持久化。

## Open Questions

- 后端当前返回的验证错误是否已经稳定到可直接做字段映射；如果没有，需要在 apply 阶段先做最小兼容映射策略。
- diagnostics 中“legacy fallback”是显示为 badge、inline note 还是 summary row，更适合当前页面的信息密度，需要在实现时结合现有 UI 组件选择最轻量方案。
