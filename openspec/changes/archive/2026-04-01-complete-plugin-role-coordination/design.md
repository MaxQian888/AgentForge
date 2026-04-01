## Context

AgentForge 现在已经分别有三条相关但没有真正收敛的链路：

- `WorkflowPlugin` 会在 `PluginService` 里校验 manifest 中的 `workflow.roles` 和 `steps[].role` 是否存在，但这个校验主要停留在注册/启用语义，插件详情页也只把 role id 当静态文本显示。
- role authoring 和 role execution profile 已经会把 `toolConfig.external` 与 `toolConfig.mcpServers` 投影成 runtime-facing `tools`，Bridge 也会按这些 id 过滤 active MCP plugins，但当前 role preview/sandbox、workspace、以及 spawn readiness 还没有 authoritative 地解释“这些 plugin/tool 依赖当前是否真的可用”。
- role API 仍可直接删除被 workflow/plugin 消费的 role；后果会被推迟到 workflow 执行或 runtime 侧失败时才暴露，前端也看不到反向影响。

仓库里已经有现成的可复用 seam：Go 侧的 role store、plugin registry/service、workflow execution runtime、role preview/sandbox，以及插件和角色各自的工作区。这个 change 的目标不是再造一套中心化平台，而是把这些现有 seam 用同一套 dependency truth 接起来。

## Goals / Non-Goals

**Goals:**

- 让 workflow -> role 和 role -> plugin/tool 这两类依赖都由同一套 Go-side 规则评估，并在 plugin detail、role workspace、preview/sandbox、enable/execute、delete/update 这些现有流程里复用。
- 让操作者在插件页看到 role 引用健康和跳转入口，在角色页看到 plugin/tool 依赖健康和下游 consumer，而不是只能等运行时报错。
- 为 destructive role 操作提供真实影响保护，避免 role 被删掉后已安装 workflow/plugin 才在执行阶段断链。
- 保持当前 repo-truthful 边界：roles 继续以 role store 为真相源，plugins 继续以 plugin registry/service 为真相源，Bridge 不额外读取 roles 或重新计算 dependency health。

**Non-Goals:**

- 不把 roles 目录并入 plugin registry，也不把 RolePlugin 改造成新的可执行 runtime。
- 不重做 marketplace install handoff；marketplace 已负责把 role/skill/plugin 物化到各自 consumer seam，本次只补 consumer 之间的联动与诊断。
- 不新增独立的“dependency center”页面或全局 dependency service 持久化表；优先复用现有 plugin 和 role DTO/详情面。
- 不尝试在本次里解决 skill compatibility 之外的通用 capability taxonomy 重构；已有 role-skill compatibility 规则继续保留，本次只追加 plugin/tool 依赖与 workflow role 引用联动。

## Decisions

### 1. 用共享的 Go-side dependency evaluator 计算双向关系，而不是让 plugin 和 role 各自拼装

实现会引入一组共享 evaluator/helper，由 Go 侧在当前 checkout 上按需计算两类事实：

- workflow/plugin 依赖的 role 引用：来自 `workflow.roles` 与 `workflow.steps[].role`
- role 依赖的 plugin/tool 引用：来自 `capabilities.toolConfig.external` 与 `capabilities.toolConfig.mcpServers[].name`

这组 evaluator 不持久化单独索引，而是基于现有 role store 和 plugin registry/service 即时生成 summary/diagnostics，供 plugin list/detail、role list/preview/sandbox、delete/update guard、workflow enable/execute 复用。

这样做的原因：

- 当前真相源已经明确存在，新增独立存储只会制造二次漂移。
- 依赖状态明显受当前 checkout 和当前 lifecycle state 影响，按需计算比缓存更真实。
- 双向关系必须使用同一套 blocking/warning 语义，否则 UI、preview、执行链路很容易再次分叉。

备选方案：

- 前端分别读取 plugin list 和 role list 后自己拼 dependency graph。拒绝原因：前端拿不到 enable/execute guard 的 authoritative 语义，也无法复用到 delete/runtime path。
- 为 dependency 另开独立数据库表或持久化索引。拒绝原因：超出本次 focused seam，并且会复制已有 registry/store 真相。

### 2. 复用现有 DTO 和详情面，不新增独立 dependency API

本次不会新增单独的 `/dependencies/*` API，而是扩展现有 contract：

- plugin records/detail 增加 role reference summary、dependency diagnostics、以及被 roles 消费的 reverse usage 摘要
- role list/get/preview/sandbox 响应增加 plugin dependency summary、downstream consumer summary、以及 destructive action impact 信息
- role delete/update、plugin enable/activate、workflow execute 继续走现有 handler/service，只是复用共享 evaluator 返回更明确的错误或 warning

这样做的原因：

- plugin-management-panel 和 role-management-panel 已经有稳定的数据入口，扩展现有 DTO 的演化成本最低。
- dependency 信息只有放在当前详情/authoring/动作语境里才有意义，单独开 API 反而会让前端自己做二次汇总。

备选方案：

- 增加一个统一 dependency endpoint，再由两个面板分别拉取。拒绝原因：会让现有 surface 多一跳数据拼装，而且 delete/enable/execute 仍然必须在各自 service 重做同一判断。

### 3. 区分三类 severity：catalog-level cue、readiness blocker、destructive guard

本次统一 severity，但不把所有依赖问题都提升为“禁止保存”：

- catalog/detail cue：在角色库、角色工作区、插件详情页展示 resolved/missing/degraded/consumer count 等状态
- readiness blocker：role preview/sandbox/spawn、plugin enable/activate、workflow execute 遇到当前 checkout 下不可执行的依赖时阻断
- destructive guard：删除 role 时，如果仍被已安装 workflow/plugin 能力消费，则直接拒绝并返回 consumer 清单

具体语义：

- role 引用的 plugin/tool 依赖在 authoring/list/detail 中可见，但不因为当前 checkout 缺依赖就阻止 role 保存；否则 role 无法先于环境准备存在。
- 同样的缺依赖进入 preview/sandbox/spawn 时会成为 blocking readiness，因为此时系统正在承诺“当前 checkout 可执行”。
- workflow plugin 的 role 引用在 manifest 注册后仍需在启用和执行时重新校验；这样 role 被改删后不会继续显示为“健康可用”。

备选方案：

- 只在执行时兜底，不在 authoring/detail 中暴露。拒绝原因：这会继续让用户在最晚阶段才发现断链。
- 保存 role 时也一律阻断所有缺依赖。拒绝原因：会把环境可用性和配置 authoring 绑死，反而削弱 role 作为 declarative asset 的价值。

### 4. 反向 consumer 视图以现有页面深链接呈现，不新增平行管理面

插件页和角色页都只展示与当前对象直接相关的 consumer/dependency 摘要，并提供现有页面深链接：

- plugin detail 可跳到 `/roles` 打开相关 role，workflow plugin 还可直接显示 step-level role bindings
- role workspace/context rail 可列出引用当前 role 的 workflow/plugin，以及当前 role 使用的 plugin/tool 依赖
- marketplace provenance 继续留在 marketplace/item detail，不在本次里扩成新的 cross-surface 入口

这样做的原因：

- 当前 repo 已经有 roles/plugins 两个稳定工作区，新增第三个 dependency workspace 只会拉宽范围。
- 用户此类需求更需要“在当前正在看的对象旁边看到另一侧影响”，而不是离开当前任务上下文再切页调查。

备选方案：

- 新建单独 dependency dashboard。拒绝原因：超出 focused change，且和现有 detail/context rail 重复。

## Risks / Trade-offs

- [共享 evaluator 需要同时读取 role store 和 plugin registry，容易在多个 handler/service 中被重复调用] → Mitigation: 把关系计算收敛到少数 helper，并在 list/detail 场景返回轻量 summary，在 preview/delete/enable 场景才拉更完整明细。
- [扩展 plugin/role DTO 可能让前端改动面变大] → Mitigation: 新字段全部以可选摘要形式追加，已有页面先按 presence 渐进消费。
- [role 对 plugin/tool 的引用并不总是等于“当前必须 active”] → Mitigation: 明确区分 authoring cue 与 execution readiness；只在 preview/sandbox/spawn/execute 阶段把当前 checkout 不可用依赖升级为 blocker。
- [role delete 阻断可能影响已有操作流] → Mitigation: 返回明确的 dependent plugin/workflow 清单和 remediation 文案，让操作者先处理引用方，而不是给出模糊 500/400。

## Migration Plan

1. 先在 Go 侧加入共享 dependency evaluation，并把 workflow role drift 与 role plugin/tool dependency 以可选摘要挂到现有 DTO。
2. 更新 plugin panel 与 role workspace/context rail，消费新摘要并加入深链接、状态标签、影响提示。
3. 在 role preview/sandbox/spawn、plugin enable/activate、workflow execute、role delete 路径接入同一 evaluator，把 runtime/drift 问题收敛成稳定错误语义。
4. 补齐前后端 targeted tests，优先覆盖 role delete guard、workflow stale role drift、role tool/plugin readiness、以及两边详情面的摘要展示。
5. 如需回退，先让前端忽略新摘要字段，再移除动作路径上的 strict guard，最后再回退 evaluator 本身，保证兼容旧 DTO。

## Open Questions

- role update 是否需要在“会影响下游 workflow/plugin”时返回 machine-readable warning metadata，还是先只在 preview/sandbox 与 delete 场景提供影响信息；本 change 默认先覆盖 delete hard guard 和 preview/detail warnings。
- 对 role 引用的 plugin/tool 依赖，readiness blocker 是否只要求“已安装”，还是要求“已 active / ready”；当前倾向按运行所需能力区分，MCP/tool runtime 依赖默认要求 active or ready-to-activate。
