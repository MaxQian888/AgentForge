## 1. Audit And Dependency Baseline

- [x] 1.1 审计 `app/(dashboard)/agent/page.tsx`、`app/(dashboard)/agents/page.tsx`、`components/agent/*`、`components/agents/*`，为每个手写 tabs / alert / empty / loading / progress 结构建立“复用现有 shadcn 组件 or 新增缺失组件”的映射清单。
- [x] 1.2 仅在审计确认存在真实缺口时，通过 `pnpm dlx shadcn@latest add ...` 安装 agent workspace 实际会用到的官方组件，并逐文件复核生成的 `components/ui/*` 实现。

## 2. Agent Workspace Shell And State Framing

- [x] 2.1 重构 `components/agents/agent-workspace.tsx`，用正式的 workspace navigation 组件承接 monitor / dispatch 切换，同时保持 URL 驱动的 agent 选择、sidebar 开合与 `/agent?id=...` → `/agents?agent=...` 行为不变。
- [x] 2.2 重构 `components/agents/agent-workspace-overview.tsx`，将 bridge health、pool diagnostics、dispatch 概览中的手写状态块收敛到统一的 alert / empty / progress / card 语义。
- [x] 2.3 重构 `components/agents/agent-workspace-sidebar.tsx`，让 agent roster 的加载态、空态、搜索无结果态具备明确且一致的 workspace framing。

## 3. Agent Summary And Detail Surfaces

- [x] 3.1 重构 `components/agent/agent-card.tsx` 与 `components/agents/agent-sidebar-item.tsx`，统一 status badge、runtime 摘要与 budget progress 的视觉表达。
- [x] 3.2 重构 `components/agents/agent-workspace-detail.tsx`，统一 header、统计卡片、pool snapshot、dispatch context 的状态与进度呈现方式，并保持现有交互动作不变。
- [x] 3.3 更新 `components/agent/output-stream.tsx`，保留 console 风格、自动滚动和等宽输出，同时引入清晰的 waiting / empty state 与更一致的外层容器语义。

## 4. Verification

- [x] 4.1 更新或新增与 agent workspace 相关的组件/页面测试，覆盖视图切换、degraded/loading/empty state、status/progress affordance 与 output waiting state。
- [x] 4.2 运行针对性的前端验证，至少覆盖本次变更涉及的测试与 lint 路径，并记录未覆盖的风险边界。
