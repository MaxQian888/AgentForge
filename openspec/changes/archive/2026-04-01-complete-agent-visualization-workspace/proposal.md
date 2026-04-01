## Why

`/agents` 现在已经有真实的 operator workspace，但主要仍是侧栏、指标卡片和详情流，缺少一个能把任务、队列、Agent、运行时与派发状态串成一眼可读关系图的视角。AgentForge 本身强调从任务分解到 Agent 执行与交付的完整链路，现有数据也已经足够支撑前端推导式可视化，所以现在需要把 Agent 可视化补到真正完整，而不是继续停留在表格和列表层面。

## What Changes

- 扩展现有 `/agents` 工作区，让 operator 在同一 workspace 内查看基于 React Flow 的流程图式 Agent 可视化，而不是跳到平行新页面。
- 基于现有 `Agent`、agent pool queue、dispatch stats、dispatch history、bridge health 数据推导图节点与边，展示任务派发、运行时选择、排队/阻塞与执行中的关系。
- 让图形化视角与当前 sidebar 选中、URL 驱动的 agent detail、monitor/dispatch 状态保持同步，确保它是现有工作区的一部分而不是只读 demo。
- 为图视角补齐明确的 loading、empty、degraded 与无匹配状态，并定义 operator 可读的节点摘要、状态强调和基础交互。
- 引入 `reactflow` 依赖并补齐针对 `/agents` workspace 与图数据映射的前端测试。

## Capabilities

### New Capabilities
- `agent-execution-visualization`: 定义基于 React Flow 的 Agent 执行与派发关系图，覆盖节点/边映射、状态表达、同步选择与空态/降级态。

### Modified Capabilities
- `agent-workspace-ui`: `/agents` 共享工作区需要把图形化可视化纳入统一 shell，并保持与现有 monitor、dispatch、detail 导航一致。

## Impact

- Frontend routes: `app/(dashboard)/agents/page.tsx`, `app/(dashboard)/agent/page.tsx`
- Frontend components: `components/agents/*`
- Frontend state/selectors: `lib/stores/agent-store.ts` 及围绕 agent graph 的前端推导 helper
- Localization and tests: `messages/en/*`, `messages/zh-CN/*`, `app/(dashboard)/agents/page.test.tsx`, `components/agents/*.test.tsx`
- Dependencies: 新增 `reactflow` 或其当前包名 `@xyflow/react`
- No backend/API contract changes: 仅消费现有 agent/pool/dispatch/bridge 数据，不要求新增 Go 或 Bridge 接口
