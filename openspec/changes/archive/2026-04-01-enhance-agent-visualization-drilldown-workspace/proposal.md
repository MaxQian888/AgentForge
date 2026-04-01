## Why

`/agents` 的 visualization 视角已经能把 task、dispatch、agent、runtime 关系画出来，但它现在仍然更像一张当前快照图：只有 agent 节点能进入详情，task/dispatch/runtime 节点停留在摘要卡片层，active view 也不会落进 URL。对于 operator 来说，这意味着图上发现了阻塞、排队或 runtime 降级后，仍要跳去别的 panel 重新找上下文，图视角还没有成为真正可操作的工作面。

## What Changes

- 让 `/agents` workspace 把当前 view 和 visualization focus 纳入 URL 驱动状态，这样 operator 可以直接 deep-link 到 visualization 模式和某个图节点上下文。
- 扩展 visualization，使 task、dispatch、runtime 节点都能进入同一 workspace 内的 drilldown surface，而不是只停留在静态摘要。
- 复用现有 dispatch history、queue admission context、runtime diagnostics 数据，为选中的图节点提供 operator 可读的原因、状态和关联关系说明。
- 保持 agent 节点沿用现有 `?agent=` detail 流，避免新图视角分叉出第二套 agent detail 信息架构。
- 为 visualization drilldown 补齐 focused/cleared/no-data 状态与对应测试，确保 view 切换和 URL 恢复不会破坏现有 monitor/dispatch 行为。

## Capabilities

### New Capabilities
- None.

### Modified Capabilities
- `agent-execution-visualization`: 图视角的 requirement 将从只读关系图扩展到支持 task/dispatch/runtime 节点钻取、上下文说明和可恢复的 focused state。
- `agent-workspace-ui`: `/agents` workspace shell 的 requirement 将扩展到支持 URL-driven view/focus state，并在 visualization drilldown 与现有 agent detail 之间保持一致导航。

## Impact

- Frontend routes and workspace state: `app/(dashboard)/agents/page.tsx`, `components/agents/agent-workspace.tsx`
- Visualization surfaces: `components/agents/agent-visualization-canvas.tsx`, `components/agents/agent-visualization-model.ts`, new or adjacent visualization detail components under `components/agents/`
- Shared data seams: `lib/stores/agent-store.ts` for dispatch history reuse in visualization drilldown
- Localization and tests: `messages/en/*`, `messages/zh-CN/*`, `components/agents/*.test.tsx`
- No backend/API contract changes: 仅复用现有 agent、queue、dispatch history、runtime catalog 与 bridge diagnostics 数据
