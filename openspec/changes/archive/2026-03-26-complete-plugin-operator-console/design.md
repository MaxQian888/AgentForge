## Context

AgentForge 当前的插件控制面已经不是后端空壳。Go 侧已经提供了完整的 operator-facing 插件路由，包括 catalog search/install、deactivate/update、event audit、MCP refresh and interaction、workflow run inspection，以及 `/internal/plugins/runtime-state` 驱动的运行态同步；插件记录里也已经保留了 trust、approval、release、runtime metadata 和 latest interaction 等字段。相比之下，前端 `app/(dashboard)/plugins/page.tsx` 与 `lib/stores/plugin-store.ts` 仍主要停留在已安装列表、built-in discover、marketplace browse、local install 和少数 lifecycle 动作上。

当前最大的 repo-truth mismatch 有两个：

- `GET /api/v1/plugins/discover` 在服务层会走 `DiscoverBuiltIns -> Install(...)`，使 built-in discovery 隐式写入注册表，而前端又把它当成“可发现但未安装”的来源分区来渲染。
- 插件页没有消费现有的 catalog install、audit events、MCP diagnostics、workflow run history、trust/release metadata 和 deactivate/update contracts，导致 operator console 缺乏对真实运行态和来源风险的可观测性。

这个 change 需要在不重开插件基础设施大重构的前提下，把插件页补成一个 repo-truthful 的 operator console：前端补齐控制面，后端只修 discovery/install 语义上最小但关键的合同漂移。

## Goals / Non-Goals

**Goals:**

- 让插件页按真实来源区分 installed、built-in discoverable、catalog/marketplace entries，并保持 install 语义 truthful。
- 暴露现有插件控制面中已经存在但前端未接入的 operator 能力，包括 deactivate/update、trust/release、audit events、MCP diagnostics 和 workflow runs。
- 通过渐进式详情/诊断面板承载高密度运行信息，避免把主列表继续堆成不可读的大卡片墙。
- 把本次实现控制在插件 operator console seam，不重复打开角色管理、工作流可视化总面板或插件运行时基础设施 change。

**Non-Goals:**

- 不实现公开远程 marketplace、artifact 下载器、自动签名链路或新的信任根体系。
- 不新增新的插件 lifecycle 状态、数据库表或 TS/Go runtime contract。
- 不把前端改成直接访问 TS Bridge；仍然保持 Go control plane 为唯一 operator-facing API。
- 不把 workflow plugin 运行态扩展成新的编排器 UI，只展示已有 run contract 的 operator view。

## Decisions

### 1. 用 read-only catalog/discovery 替换当前“discover 即 install”的 built-in 语义

前端的 built-in 和 catalog 分区必须建立在 read-only discover/search 结果上，而不是继续依赖 `DiscoverBuiltIns()` 这种会隐式注册插件的接口。实现上应把 built-in availability 优先建立在 `/api/v1/plugins/catalog` 或等价的 read-only catalog feed 上，并让显式 install action 成为唯一创建 installed record 的入口。

这样做的原因是：

- 当前插件页已经把 built-ins 视为“可安装候选”，但后端 discovery 会直接写 registry，source-channel 语义自相矛盾。
- `plugin-catalog-feed` 已经是更贴近产品语义的 contract；修正后可以让 built-in、catalog entry、installed state 三者保持一致。
- 这是一个小而关键的 backend correction，能显著降低前端额外补偿逻辑。

备选方案：

- 继续保留 `/plugins/discover`，前端只做 client-side 去重和文案修饰。缺点是 installed state 仍会被一次 browse 操作污染，无法形成 truthful contract。
- 彻底删除 built-in 分区，仅保留 marketplace。缺点是会隐藏当前 repo 真实存在的 builtin plugin seam。

### 2. 保持 Go 为唯一 operator-facing 插件控制面

所有新增 operator flows 仍通过 Go API 暴露，前端不会绕过 Go 直接访问 TS Bridge 或 runtime internals。插件页只扩展 store/actions 去调用现有的 `/api/v1/plugins/*`、`/plugins/catalog*`、`/plugins/:id/events`、`/plugins/:id/mcp/*` 和 `/plugins/:id/workflow-runs` 路由。

这样做的原因是：

- Go 已经是插件 registry、trust state、runtime sync 和 audit event 的 authoritative seam。
- 直接让前端访问 TS Bridge 会绕开 trust/lifecycle gating，并再次制造第二份 source of truth。
- 当前 change 的价值是把 operator surface 接上，而不是重新设计跨宿主协议。

备选方案：

- 前端直连 TS Bridge 以减少 API 层包装。缺点是会破坏当前 control-plane 分层，也无法统一 ToolPlugin 与 Go-hosted Workflow/Integration 的 operator view。

### 3. 用“主列表 + 渐进式详情诊断面板”承载高密度运行信息

插件页继续保留列表筛选与快速动作，但把高密度信息下沉到选中插件详情区，并在详情区按 section 或 tab 展示：overview、trust/release、runtime diagnostics、events、workflow runs。事件、workflow runs 和 MCP refresh/call/read/get 等重负载能力采用按需获取，而不是页面初始加载时全量请求。

这样做的原因是：

- 当前主列表已经接近信息密度上限，如果继续在卡片层加入 trust、release、events、latest interaction、workflow runs，会快速失控。
- 选中态详情区已经存在，是最自然的渐进式扩展点。
- 按需加载可以避免为每次打开插件页都触发多条诊断请求。

备选方案：

- 为 diagnostics、events、workflow runs 分拆子路由。缺点是 operator 需要频繁跳转页面，且当前插件控制台场景更偏“面板工作流”而不是“多页工作流”。
- 将所有信息都铺在卡片上。缺点是移动性和可扫描性都会恶化。

### 4. install 与 lifecycle 动作按 source/runtime/trust 显式分流

插件页中的动作区需要明确分成几类：

- Local install: 本地路径/manifest 安装。
- Catalog install: 基于 catalog entry 的显式安装。
- Runtime lifecycle: enable/disable/activate/deactivate/restart/health。
- Update: 仅在插件存在真实 source/release metadata 时展示或允许。

同时，UI 必须把“不可执行”“未信任”“browse-only”“缺少更新源”这些限制显式解释出来，而不是只把按钮隐藏掉。

这样做的原因是：

- 当前后端 contracts 已经区分 source type、trust state 和 runtime host，但前端动作层仍过于扁平。
- operator console 的核心价值之一就是把“为什么不能做”解释清楚。
- 这能让后续远程 marketplace 或 approval gate 到来时自然衔接，而不需要推翻动作模型。

备选方案：

- 只补按钮，不补原因文案。缺点是 operator 仍然无法理解为什么某些动作不存在或失败。

### 5. diagnostics 只消费现有真实字段和接口，不额外发明新后端聚合层

本次诊断面板优先复用已有契约：

- trust/release: 直接取 `PluginRecord.source.*`
- MCP summary: 直接取 `runtime_metadata.mcp`
- audit history: `GET /api/v1/plugins/:id/events`
- workflow runs: `GET /api/v1/plugins/:id/workflow-runs`
- operator-triggered MCP operations: existing refresh/tool/resource/prompt routes

这样做的原因是：

- 当前后端能力已经足够支撑 operator 视图，缺的是消费层。
- 新增聚合 API 会扩大 scope，并重复已有 registry/runtime/audit data model。
- 先消费真实字段，有助于反向暴露 contract drift，而不是把 drift 包在一层新 DTO 里。

备选方案：

- 新增 `/plugins/:id/dashboard` 聚合接口。缺点是把当前 focused UI seam 扩成新的 backend aggregation change。

## Risks / Trade-offs

- [修正 discovery 语义会影响现有 built-in flow] -> 保持 install endpoint 不变，只把 browse/discover 改成 read-only，并补 Go handler/service tests 锁定新语义。
- [插件详情面板变重] -> 采用按需加载 events/workflow runs/MCP interactions，只在选中插件后请求重数据。
- [不同 plugin kind 的诊断面不一致] -> 以 kind-aware empty state 处理，不为没有 MCP 或 workflow contract 的插件伪造统一表格。
- [现有 `/plugins/marketplace` 与 `/plugins/catalog` 语义有重叠] -> 本次先在 UI 层明确 builtin/catalog/installable/browse-only 的呈现规则，并把长期远程 marketplace 留在 open question。
- [dirty worktree 下容易误把 broader plugin backlog 带入此 change] -> 任务和验证只围绕插件 operator console、discovery semantics 与已有 control-plane API 消费，不扩到插件 runtime foundation。

## Migration Plan

1. 先修正 Go 侧 built-in/catalog discovery 语义，并补 focused service/handler coverage，确保 browse 不再隐式 install。
2. 扩展 `plugin-store`，新增 catalog install、deactivate/update、events、workflow runs、MCP diagnostics 相关 actions 和选中态诊断数据。
3. 重构插件页与相关组件，完成来源分区、安装入口、详情诊断 sections 和动作 gating 文案。
4. 追加 focused frontend tests 与 targeted Go tests，最后用 `openspec status --change ... --json` 确认 artifacts apply-ready。

回滚策略：

- discovery 语义如有回归，可先回退到旧 discover route 行为，同时保留新增 UI 结构；这只会让 builtin panel 回到旧的不 truthful 状态，不涉及数据迁移。
- 前端 operator console 是增量 UI，更适合按文件回退，不需要数据库恢复步骤。

## Open Questions

- `GET /api/v1/plugins/marketplace` 在短期内是否继续承担 builtin+installed merged feed，还是应逐步退让给更明确的 catalog feed？本次先按现有 contracts 做 truthful 区分，不强制统一接口。
- workflow run 视图是否只展示最近 N 条，还是需要最小分页能力？本次倾向先做最近记录视图，避免打开新的 workflow operator surface。
- update action 是否需要前端显式“available update”比较逻辑，还是仅在 `source.release.availableVersion` 存在时展示即可？本次默认以后者为准。
