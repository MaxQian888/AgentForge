## Context

AgentForge 当前的 Agent operator surface 已经收敛到 `app/(dashboard)/agents/page.tsx` 驱动的 `AgentWorkspace`。这个 workspace 现在依赖同一份 `useAgentStore` 数据源消费 `agents`、`pool`、`runtimeCatalog`、`bridgeHealth`、`dispatchStats`，并通过 URL `?agent=` 进入 `AgentWorkspaceDetail`。也就是说，现有 `/agents` 已经具备真实的运行时数据和统一详情流，但主要表达方式仍是 sidebar、summary cards、queue table 和 detail panel。

与此同时，仓库里唯一接近“图”的实现是 `components/workflow/workflow-config-panel.tsx` 中的静态网格式 `WorkflowGraph`，它没有 pan/zoom、节点交互、关系高亮，也没有复用到 Agent workspace。仓库当前也尚未安装 `reactflow/@xyflow/react`。这次 change 需要在不扩展 Go/Bridge API 的前提下，把已有 agent/pool/dispatch 数据推导成前端可消费的流程图视角，并保持和当前 `/agents` shell、query param、detail surface 一致。

## Goals / Non-Goals

**Goals:**
- 在现有 `/agents` workspace 内增加一个一等公民的 Agent 可视化视角，而不是新增平行页面。
- 使用 React Flow 把当前可见 scope 中的任务、排队/阻塞派发、Agent、运行时关系表达为可浏览的流程图。
- 让图视角与当前 URL 驱动的 agent 选择、sidebar scope、detail surface 保持同步。
- 为图视角补齐 loading、empty、degraded、no-match 状态，并保留 operator 级状态提示。
- 把图数据推导和节点渲染拆成可测试的前端 seam，避免把复杂映射逻辑塞回页面组件。

**Non-Goals:**
- 不新增或修改 Go Orchestrator、TS Bridge、WebSocket、REST API contract。
- 不把 `/agents` 改造成可拖拽编辑器；这次图视角是读操作优先，不承担 workflow builder 职责。
- 不引入持久化布局、多人协作标注、时间轴回放等超出当前 store 数据面的能力。
- 不替换现有 `AgentWorkspaceDetail` 的输出流、操作按钮或 dispatch context 信息架构。

## Decisions

### D1: 把可视化作为 `/agents` shell 的第三个一等视角

`AgentWorkspace` 现有 monitor / dispatch 共享一套 sidebar、toolbar、detail framing。新的流程图视角将作为同一 workspace navigation 下的第三个视角加入，而不是嵌到 monitor 概览卡片里，也不是独立路由。这样可以直接继承现有 shell、URL 选择逻辑与 mobile/desktop 布局，保证“图视角”是现有 Agent operator surface 的补全。

**Alternative considered**: 把图直接塞进 monitor 概览顶部。  
**Rejected because**: 会让 monitor 面板继续膨胀，也无法把图视角提升为明确的 operator 工作模式。

### D2: 图数据完全由当前 `/agents` 已加载的数据推导，不新增页面级 API fan-out

图视角只消费 `agents`、`pool.queue`、`runtimeCatalog`、`bridgeHealth`、`dispatchStats` 与当前 URL/member scope。不会为了全局图再为每个 task 补拉 `dispatchHistoryByTask`，也不会要求后端新增“graph API”。数据推导会放到独立 helper 中，把 page/store snapshot 映射为 React Flow 的 nodes/edges/legend 数据，便于单测和后续替换。

这意味着图主要表达“当前 roster + queue + runtime availability”的实时快照，而不是完整历史回放。详情页仍然保留 `dispatchHistoryByTask` 的细节职责。

**Alternative considered**: 在 graph view 中对所有 task 追加历史请求，拼出更完整派发时间线。  
**Rejected because**: 会带来额外页面 fan-out、性能噪音和新的 loading/error surface，超出这次 focused completeness 的边界。

### D3: 采用确定性的分栏布局，而不是再引入自动布局引擎

图是一个强约束的 operator pipeline，而不是任意 DAG 编辑器。节点会按固定泳道布局：`task -> queued/blocked dispatch -> agent -> runtime`。同一类节点在纵向堆叠，运行时节点按 runtime/provider/model tuple 去重，减少重复。这样可以只新增 React Flow 一个核心依赖，不再额外引入 dagre/ELK，避免布局抖动、客户端复杂度和测试不稳定。

**Alternative considered**: 引入 dagre 或 ELK 做自动布局。  
**Rejected because**: 当前图结构并不复杂，自动布局收益有限，额外复杂度和 bundle 成本不值得。

### D4: 节点交互以“同步现有 detail 流程”为核心，而不是发明第二套 drill-down

Agent 节点点击后会走现有 `?agent=` 选择流程，进入同一份 `AgentWorkspaceDetail`。Task、queue、runtime 节点只承担摘要和关系高亮职责，不引入新的持久化 side panel 或替代 detail 页的交互系统。这样可以确保 operator 无论从 sidebar 还是图视角进入，都看到同一套暂停、恢复、终止、输出流和 dispatch context。

**Alternative considered**: 为所有节点都做独立详情面板。  
**Rejected because**: 会把一次可视化补全扩展成新一轮信息架构设计，偏离“完善现有 surface”的范围。

### D5: 图视角的状态语义沿用现有 workspace contract

如果 `/agents` 正在初次加载且没有任何 roster/queue 数据，图区域显示 loading skeleton；如果当前 scope 下没有任何 agent 或 queue entry，则显示显式 empty/no-match；如果 bridge health degraded，则在图视角中显示降级提示，并在有数据时继续渲染最后的快照图，而不是直接空白。状态组件和文案沿用现有 agents/workflow 风格，避免出现新的状态语言体系。

**Alternative considered**: graph 只在“有数据时渲染”，其他情况直接隐藏。  
**Rejected because**: 这会把 operator 最需要判断的问题重新变成“为什么这里空了”。

### D6: React Flow 的基础样式通过现有全局样式面接入，并限制自定义范围

React Flow 自带的基础样式需要进入 Next.js app 样式面。实现上会把必要的基础样式接入现有全局样式入口，再在 agent graph 组件内追加少量面向节点状态和卡片语义的 class，不为这次变更建立新的视觉系统。这样可以降低对 static export、desktop shell 与测试快照的意外影响。

**Alternative considered**: 大量重写 React Flow 默认视觉。  
**Rejected because**: 这会把 change 从“功能完整性 + operator 可读性”拉向高成本视觉重构。

### D7: 验证聚焦在图映射和 workspace 行为，不扩大到整仓库图形回归

验证层会覆盖两类 seam：
- 图数据映射 helper：给定 `agents`、`pool.queue`、`runtimeCatalog` 等输入时，输出稳定的 nodes/edges/summary。
- Workspace 集成：新视角可切换、空态/降级态可见、agent 节点点击进入现有 detail 流。

不会为这次 change 引入新的 e2e harness，也不会把 `reactflow` 内部 DOM 细节做成大快照。

**Alternative considered**: 只做页面人工验证或大面积 snapshot。  
**Rejected because**: 前者难以防回归，后者会让第三方库细节主导测试噪音。

## Risks / Trade-offs

- [Risk] Agent 数量和 queue entry 较多时，图可能变得密集。 → Mitigation: 运行时节点去重、固定泳道、React Flow 自带 pan/zoom、仅展示当前 workspace scope。
- [Risk] React Flow 引入后可能影响 Next.js 样式或静态导出边界。 → Mitigation: 将基础样式接入现有全局样式面，并把 graph 组件保持在现有 client workspace 内。
- [Risk] 图节点如果承载过多信息，会和 sidebar/detail 职责重叠。 → Mitigation: 节点只放 operator 级摘要，完整操作和日志仍回到现有 detail surface。
- [Trade-off] 这次不会提供历史回放级 dispatch timeline。 → Accept，因为当前页面数据面更适合实时快照图而不是全量时序图。
