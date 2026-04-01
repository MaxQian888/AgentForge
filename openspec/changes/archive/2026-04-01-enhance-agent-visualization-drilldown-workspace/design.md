## Context

`complete-agent-visualization-workspace` 已经把 `/agents` 补成三视图 workspace：monitor、visualization、dispatch 共享同一套 sidebar、toolbar 和 agent detail framing。当前 visualization 通过 `buildAgentVisualizationModel()` 把 `agents`、`pool.queue`、`runtimeCatalog` 与 `bridgeHealth` 映射成 React Flow graph，并且只有 agent 节点会走现有 `?agent=` 详情流。

这带来了两个真实缺口。第一，view state 仍然是组件内本地 state，reload 或 share link 后无法直接回到 visualization。第二，task、dispatch、runtime 节点虽然已经在 graph 上可见，但 operator 发现阻塞原因、排队上下文或 runtime 诊断后，仍需要离开 graph 去别的面板重新找信息。仓库中其实已经有可复用的数据 seam，例如 `dispatchHistoryByTask`、`fetchDispatchHistory(taskId)`、runtime catalog diagnostics，以及现有 workspace detail shell，因此这次设计应当优先做前端内聚与导航一致性，而不是再开新的 API 或第二套详情体系。

## Goals / Non-Goals

**Goals:**
- 让 `/agents` workspace 支持 URL-driven 的 active view 与 visualization focus，使 graph 视角可以被 reload 和 deep-link 恢复。
- 让 visualization 内的 task、dispatch、runtime 节点具备 operator drilldown，而不是仅停留在只读摘要。
- 复用现有 dispatch history、queue admission context、runtime diagnostics 和 agent workspace shell，而不是新建并行数据面。
- 保持 agent 节点继续走现有 `?agent=` detail 流，避免 graph-only agent detail 分叉。
- 为 visualization focus、URL hydration、dispatch history fetch 与 clear-state 行为提供可测试 seam。

**Non-Goals:**
- 不新增 Go / Bridge API、WebSocket contract 或新的 graph-specific backend endpoint。
- 不实现 timeline replay、历史回放、持久化节点布局或拖拽编辑。
- 不把 visualization drilldown 扩展成通用工作流 builder 或替换现有 Dispatch tab。
- 不重写 monitor / dispatch 主布局，只在现有 `/agents` workspace 中补 focus-aware surface。

## Decisions

### D1: 用 query param 扩展 `/agents` workspace 状态，而不是新增 route

workspace 将把 active view 表达为 `view=monitor|visualization|dispatch`，并为 visualization 的非 agent focus 增加专用 query key（例如 `vizNode=<kind>:<id>`）。这样 operator 可以直接分享或恢复 `/agents?view=visualization&vizNode=dispatch:queue-123`，而不需要新增平行 route，也不会打破现有 `/agents?agent=<id>` 深链。

**Alternative considered**: 为 visualization drilldown 建新的 `/agents/visualization` 或 `/agents/graph/[node]` 路由。  
**Rejected because**: 这会拆散当前统一 workspace shell，并引入第二套路由语义与加载逻辑。

### D2: 非 agent 节点 drilldown 保持“graph + context rail”同屏，而不是替换整个主内容区

当选中 task、dispatch 或 runtime 节点时，workspace 保持 visualization canvas 可见，并在同一工作区中渲染 focus-aware context rail 或 detail panel。operator 可以一边看关系图，一边看该节点的 dispatch history、queue reason、runtime diagnostics 和关联对象摘要。只有 agent 节点继续切回现有 detail view，因为 agent 已经有完整的 output/controls surface。

**Alternative considered**: 所有节点都像 agent 一样切换成 full-page detail。  
**Rejected because**: task/dispatch/runtime 的核心价值是为 graph 提供上下文，不值得为它们复制一套完整详情页切换体验。

### D3: 扩展 graph model，输出稳定的 focus metadata，而不在 canvas 内现拼详情数据

`buildAgentVisualizationModel()` 除了 nodes、edges、summary，还将输出按 node id 索引的 focus metadata，至少覆盖 task、dispatch、runtime 节点的可读详情来源。这样 canvas、detail rail 和测试都使用同一份结构化映射，而不是让 UI 组件靠 node label 反推 taskId 或 runtime tuple。

**Alternative considered**: 在 canvas 点击事件里基于 node id 字符串临时解析详情。  
**Rejected because**: 会把展示层和数据推导耦合在一起，也不利于 URL hydration 与单测。

### D4: task / dispatch drilldown 复用现有 dispatch history seam，runtime drilldown 复用 runtime catalog 诊断

当 focus 到 task 或 dispatch 节点时，workspace 使用 node metadata 提供的 task id 调用现有 `fetchDispatchHistory(taskId)`，并复用 `DispatchHistoryPanel` 或相邻展示模式呈现 dispatch attempt timeline。dispatch 节点的 panel 额外显示 admission outcome、queue reason、priority、budget 等已在 queue entry 中存在的上下文。runtime 节点则直接消费 `runtimeCatalog` 中的 availability、diagnostics、supportedFeatures，并补充当前 graph 中关联的 agent / dispatch 计数。

**Alternative considered**: 新增 graph 专用聚合 endpoint，一次返回所有 detail 内容。  
**Rejected because**: 现有数据已经足够覆盖这次范围；新 API 只会扩大 change 面和验证成本。

### D5: `agent` selection 对 graph focus 具有更高优先级，但双方都共存于同一 query contract

若 URL 中存在 `agent=<id>`，workspace 继续渲染现有 `AgentWorkspaceDetail`，并忽略 `vizNode`。当 operator 从 visualization 点击 agent 节点时，行为保持不变，同时清除 `vizNode`。当 operator 清除 agent detail 返回 workspace 时，如果 URL 中保留 `view=visualization`，则回到 visualization；若存在可恢复的 `vizNode`，则恢复该 focus。这样可以保持现有 deep-link 兼容，同时让 graph drilldown 真正可恢复。

**Alternative considered**: 完全禁止 `agent` 与 `vizNode` 共存。  
**Rejected because**: 用户可能直接从 share link 或浏览器前进后退进入组合状态，workspace 需要有明确的优先级而不是脆弱失败。

### D6: 验证集中在 URL hydration、focus drilldown 和共享 seam 复用

测试将覆盖三类行为：一是 workspace 根据 query param 恢复 view / focus / agent detail；二是 graph node selection 更新 URL 并显示对应 drilldown；三是 task / dispatch focus 触发 dispatch history 复用而不破坏现有 monitor / dispatch tab。不会为这次 change 引入 e2e harness，也不会对 React Flow DOM 做大快照。

**Alternative considered**: 只做人工点击验证。  
**Rejected because**: query-state 与 focus-priority 容易回归，没有自动化很难维持稳定。

## Risks / Trade-offs

- Query contract 变复杂 → 通过限定键名与优先级（`agent` > `vizNode`）并对未知值回退到安全默认值来降低歧义。
- visualization model 承担更多 detail metadata → 通过把 focus payload 控制在 UI 需要的最小字段集合内，避免把整个 store snapshot 都塞进 graph model。
- dispatch history 可能按 task 懒加载，首次 focus 有短暂 loading → 在 detail rail 中提供显式 loading/empty 状态，不阻塞 graph 本身渲染。
- graph 与 context rail 同屏后，移动端空间更紧张 → 沿用当前 workspace 响应式模式，在窄屏下用 sheet/drawer 展示 drilldown，而不是压缩 graph 到不可读。

## Migration Plan

1. 先扩展 workspace query-state 解析与写回逻辑，使旧链接在没有 `view` / `vizNode` 时继续落到当前默认行为。
2. 扩展 visualization model 与 node selection contract，保证新 detail rail 可以从稳定 metadata 驱动。
3. 接入 task / dispatch / runtime drilldown UI，并把 dispatch history 复用接到对应 focus path。
4. 补齐翻译与测试后，再用 targeted frontend verification 验证 `/agents` 的 tab、detail、graph 互不回退。

回滚策略很简单：这是纯前端 workspace 变更，若需要回退，只需移除新增 query-state 和 drilldown 组件，旧的 monitor / visualization / dispatch 行为仍可独立存在。

## Open Questions

- None at proposal time. 当前范围不依赖新的后端 contract，剩余选择都属于 apply 阶段的具体组件拆分问题。
