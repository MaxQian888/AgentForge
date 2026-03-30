## Context

AgentForge 当前真实的 agent UI 入口是 `app/(dashboard)/agent/page.tsx` 与 `app/(dashboard)/agents/page.tsx`，对应的主要实现集中在 `components/agent/*` 与 `components/agents/*`。其中 `/agent` 只是向 `/agents?agent=...` 的重定向壳，真正的 operator workspace 在 `AgentWorkspace`、`AgentWorkspaceSidebar`、`AgentWorkspaceOverview`、`AgentWorkspaceDetail`、`AgentCard`、`OutputStream` 这一组组件上。

仓库已经初始化 shadcn/ui，项目上下文确认使用 Next.js 16、Tailwind v4、`new-york` 风格、`radix` base，且已经安装 `button`、`card`、`badge`、`tabs`、`table`、`sheet`、`tooltip`、`scroll-area`、`separator`、`skeleton` 等组件。当前缺口不在“是否能用 shadcn”，而在 agent workspace 仍保留了多处手写 UI 结构：

- `AgentWorkspace` 用按钮模拟 monitor / dispatch tab。
- `AgentWorkspaceOverview` 与 `AgentWorkspaceDetail` 仍有手写 health/info 条和 progress bar。
- `AgentSidebarItem` 与 `AgentCard` 也各自维护一套成本进度和状态表达。
- `OutputStream` 已经正确使用 `ScrollArea`，但空态和外层状态表达仍较原始。

另外，shadcn registry 已确认 `@shadcn/progress`、`@shadcn/alert`、`@shadcn/empty` 可直接使用；`@shadcn/field` 虽然可用，但当前 agent workspace 没有明确的复杂表单场景，不应为“尽可能使用”而引入未被消费的组件。

## Goals / Non-Goals

**Goals:**
- 逐个组件审视 agent workspace，并优先复用仓库中已存在的 shadcn/ui 组件。
- 在存在明确缺口时，仅新增 agent workspace 真正需要的 shadcn 组件，并统一走 `pnpm dlx shadcn@latest add ...`。
- 统一 agents 页面、sidebar、overview、detail、card、output stream 的状态表达方式，让 tabs、alert、empty、loading、progress 具有一致语义。
- 在不改动 store、API、路由合同的前提下，提升 operator-facing agent UI 的一致性与可维护性。

**Non-Goals:**
- 不修改 Go / Bridge / Zustand 的数据结构与接口语义。
- 不重做整个 dashboard shell 或 role / task / project 工作区。
- 不为了形式统一而移除 `OutputStream` 的终端风格表达。
- 不安装 audit 之外用不到的 shadcn 组件。

## Decisions

### D1: 范围严格限定在真实 agent workspace 路径

本次 change 只覆盖 `app/(dashboard)/agent/page.tsx`、`app/(dashboard)/agents/page.tsx`、`components/agent/*`、`components/agents/*`。虽然用户口头提到了 `app/agent`，但仓库里不存在该目录；若把不存在的路径写入 design，会在 apply 阶段制造歧义，并与已归档的 `enhance-agent-role-completeness` 发生不必要重叠。

**Alternative considered**: 把 role / spawn / dispatch 相关所有 agent 周边页面一起纳入。  
**Rejected because**: 范围会重新膨胀成 product-completeness 变更，偏离“逐个组件检查并替换为 shadcn”的 focused seam。

### D2: 先复用已安装组件，再补最小缺口

优先复用仓库已经存在的 `Tabs`、`Card`、`Badge`、`Table`、`Button`、`Sheet`、`Tooltip`、`ScrollArea`、`Separator`、`Skeleton` 等组件。只有当现有组件无法自然表达目标语义时，才新增缺失组件。

明确的首选补充组件为：
- `@shadcn/progress`：替换 `AgentCard`、`AgentSidebarItem`、`AgentWorkspaceDetail` 中手写预算进度条。
- `@shadcn/alert`：承接 bridge degraded / diagnostics warning 之类的 operator-facing 状态提示。
- `@shadcn/empty`：承接 agents 空态、过滤后无匹配、尚无输出等显式空状态。

`@shadcn/field` 暂不作为默认安装项，除非组件审计发现 agent workspace 内存在真正需要 field composition 的表单面。

**Alternative considered**: 直接继续用手写 `div` / `span` 包装，保持现状。  
**Rejected because**: 现状已经出现同一概念在多个组件里重复实现，后续维护只会继续分叉。

### D3: OutputStream 保持 console 语义，只统一其外层框架

`OutputStream` 是日志/终端流，不应该被改造成普通内容卡片。设计上保留其深色、等宽、按时间滚动和自动滚动到底部的语义；统一的是其容器边界、空态、与 detail 页其他区块的衔接方式，而不是把日志内容本身做成常规业务面板。

**Alternative considered**: 把输出流完全改造成通用 Card + prose 内容区。  
**Rejected because**: 会降低日志扫描效率，也不符合 agent runtime 输出的实际使用方式。

### D4: Tabs / Alert / Empty / Progress 的引入不改变现有数据合同

这次替换只改变 UI 组成，不改变 URL 参数、store shape、translation key contract 或 API 调用时机。`/agent?id=...` 仍重定向到 `/agents?agent=...`；sidebar 选择、dispatch 统计、bridge health、output stream 的数据来源保持不变。

**Alternative considered**: 顺手重构 store 或统一数据映射。  
**Rejected because**: 这会把 UI adoption 变成状态层重构，增加不必要风险。

### D5: 测试按组件合同更新，而不是用宽泛快照覆盖

验证重点放在组件级行为：tabs 切换、degraded/empty/loading 状态是否可见、progress 与 status 是否一致、output 空态与滚动行为是否保持。避免用大而泛的快照把 shadcn 源码细节一起锁死。

**Alternative considered**: 只做人工页面检查或补全量 snapshot。  
**Rejected because**: 前者回归保护不足，后者会让 shadcn 源码变动造成无意义噪音。

## Risks / Trade-offs

- [Risk] 组件审计容易滑向大范围视觉重构。 → Mitigation: 严格限制在现有 agent workspace 文件与现有用户请求，不顺手扩到 role/task/project 页面。
- [Risk] 新增 shadcn 组件会把生成文件写入 `components/ui`，若不审阅容易带入不符合仓库风格的细节。 → Mitigation: 安装后逐文件复核生成物，只保留被 agent workspace 实际消费的组件。
- [Risk] 统一 progress / empty / alert 后，现有测试和文案可能出现小范围破坏。 → Mitigation: 保持现有翻译 key 与 store 数据流，测试围绕可见行为更新。
- [Trade-off] `OutputStream` 不会被完全“组件化”。 → Accept，因为日志可读性比视觉绝对统一更重要。
