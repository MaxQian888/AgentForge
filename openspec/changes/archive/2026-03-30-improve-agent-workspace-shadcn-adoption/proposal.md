## Why

`/agents` 相关体验在最近几轮 change 后已经具备真实的监控、调度和详情能力，但当前前端仍混用了 shadcn/ui 组件与手写结构：顶部视图切换还是按钮拼装，bridge health / empty / loading / cost progress 仍有多处自定义标记，`components/agent` 与 `components/agents` 之间的视觉和状态表达也不一致。现在补一条 focused change，可以在不改动后端契约的前提下，把 agent workspace 收敛到同一套可复用 UI 语义，并把明确缺失的 shadcn 组件通过 `pnpm` 流程补齐。

## What Changes

- 逐个审视真实 agent UI 入口与组件链路：`app/(dashboard)/agent/page.tsx`、`app/(dashboard)/agents/page.tsx`、`components/agent/*`、`components/agents/*`。
- 为 agent workspace 建立统一的 UI 复用基线，优先采用已安装的 shadcn/ui 组件，而不是继续保留手写 tabs、alert-like banner、empty/loading 块和 progress bar。
- 将 agents 页面监控/调度切换、bridge health 提示、空态/加载态、队列与概览卡片、agent sidebar item、agent detail 统计块、agent card 等界面收敛到一致的组件组合。
- 在审计确认存在明确缺口时，通过 `pnpm dlx shadcn@latest add ...` 安装缺失的官方组件，并限制在 agent workspace 真正会使用到的范围内。
- 保留 `OutputStream` 的终端语义与信息密度，不为“全量替换”牺牲日志阅读体验，但统一其外层容器、空态和关联状态表达。
- 为涉及的 agent UI 组件补齐或更新针对性的测试，确保组件替换不改变现有交互合同。

## Capabilities

### New Capabilities
- `agent-workspace-ui`: 定义 agent operator workspace 的前端组成约束，包括监控/调度视图切换、bridge 健康提示、空态/加载态、成本/状态进度表达，以及列表卡片/详情面板/输出流之间的一致 UI 语义。

### Modified Capabilities

## Impact

- Frontend routes: `app/(dashboard)/agent/page.tsx`, `app/(dashboard)/agents/page.tsx`
- Frontend components: `components/agent/*`, `components/agents/*`
- Existing UI primitives: `components/ui/*`
- Dependencies: 如审计确认缺口，新增的 shadcn/ui 组件需统一通过 `pnpm dlx shadcn@latest add ...` 安装，优先考虑 `@shadcn/progress`、`@shadcn/alert`、`@shadcn/empty`
- Tests: 相关 agent UI 组件测试与页面测试需要同步更新
- No backend/API changes: 仅调整前端 workspace 组成与组件复用方式，不改变 Go / Bridge 接口合同
