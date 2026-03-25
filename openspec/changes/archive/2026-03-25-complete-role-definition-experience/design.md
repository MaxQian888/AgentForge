## Context

AgentForge 当前已经具备一条基础的角色定义链路：Go 侧 `src-go/internal/role/*` 可以解析和保存 YAML 角色，`/api/v1/roles` 能做基础 CRUD，前端 `components/roles/role-workspace.tsx` 也已经从简单弹窗进化成结构化工作台。但这条链路仍然明显落后于 `docs/PRD.md`、`docs/part/PLUGIN_SYSTEM_DESIGN.md` 与 `docs/part/PLUGIN_RESEARCH_ROLES.md` 所定义的角色 authoring 蓝图。

当前真实缺口分成三层：

- 合同层：Go/TS 当前角色模型只覆盖了高级 schema 的一部分。像 `identity.personality/language/response_style`、`capabilities.packages/toolConfig/maxConcurrency/customSettings`、`knowledge.shared/private/memory`、`security.profile/permissions/outputFilters/resourceLimits`、`collaboration`、`triggers`、`overrides` 等字段，还没有形成完整、稳定、可 round-trip 的产品合同。
- authoring 层：现有角色工作台主要覆盖 metadata、基础 prompt、skills、knowledge、security，缺少高级字段编辑、字段说明、原始 YAML 预览、继承后有效值可见性，以及保存前的 authoritative preview。
- 验证层：当前只有前端执行摘要，没有一条后端权威的 preview/sandbox 流。作者无法在保存前验证 advanced role draft 的解析结果、继承合并、触发器/协作配置是否生效，也无法在受控范围内做一次角色行为 dry-run。

同时，这次设计也有几个约束：

- 角色的 source of truth 仍然是 `roles/<role-id>/role.yaml`，不能引入第二套权威存储。
- 角色 preview/sandbox 不能走“前端自己猜”，必须复用 Go 的解析、归一化和 execution-profile 投影逻辑。
- 仓库已经存在轻量文本生成入口 `src-bridge/src/server.ts` 的 `/bridge/generate`，可以用来做非持久化的角色 prompt probe，不必直接启动完整 coding-agent 运行时。

## Goals / Non-Goals

**Goals:**

- 将角色合同扩展到 PRD 已定义但当前未完整闭环的高级字段，并保证 YAML、API、前端 draft、preview、sample role 之间一致。
- 提供完整的角色 authoring workspace，覆盖高级字段编辑、字段说明、继承/override 可见性、原始 YAML 预览和有效执行摘要。
- 提供服务端权威的 preview/sandbox 能力，让作者能在保存或发布前验证 effective manifest、execution profile、运行前 readiness 与受控测试输出。
- 更新角色相关文档、样例角色和 focused tests，使“如何定义角色”在仓库中自洽，而不是继续依赖漂移的设计文档。

**Non-Goals:**

- 不在本次中建设公开 Role Marketplace、团队共享/fork 产品流或角色评分体系。
- 不在本次中提供 Git 级角色版本历史、分支管理或回滚 UI；本次只保证 `metadata.version` 与 authoring guidance 闭环。
- 不在本次中启动真实可写文件系统的 coding-agent sandbox；角色探针以非持久化 preview 与轻量 prompt probe 为主。
- 不重做 Team Management 或 Agent launch 全流程；本次只在需要时消费现有 runtime catalog/readiness seams。

## Decisions

### 1. 用“完整 typed role contract + 少量受控动态字段”补齐高级 schema，而不是继续扩张松散 map

角色定义将继续以 Go 为主导，但这次会把 PRD 中已经稳定的高级结构提升为显式类型，而不是继续让前后端各自持有零散字段或 `map[string]any` 占位。优先结构化的部分包括：

- `metadata.icon`
- `identity.persona/goals/constraints/personality/language/response_style`
- `capabilities.packages/tools.built_in/tools.external/tools.mcp_servers/max_concurrency/custom_settings`
- `knowledge.shared/private/memory`
- `security.profile/permissions/output_filters/resource_limits`
- `collaboration`
- `triggers`

仍然允许保留一定动态性的地方只限于确实需要表达式或开放 key/value 的子字段，例如 trigger condition 文本或 capability custom settings。

这样做的原因是：

- 这些字段已经在 PRD 与设计文档中稳定存在，不再属于“未来也许会变”的探索项。
- typed contract 才能让 YAML round-trip、API 校验、前端 draft、preview 输出和 sample role 测试共享同一真相。
- 继续堆 `map[string]any` 会让 authoring UI、validation、文档和测试都越来越脆弱。

备选方案：

- 继续沿用当前“基础 typed + 高级 map”模式。优点是初期改动小；缺点是 role definition 永远无法形成真正可编辑、可验证的产品面。

### 2. Preview 和 sandbox 走 Go 权威接口，而不是前端本地推导

这次不会再把 preview 建在前端摘要推断上。新的 preview/sandbox 会由 Go 暴露显式接口，内部复用同一套 role parse/normalize/inheritance/execution-profile 流程。前端 role workspace 只负责发送 draft 与展示结果。

建议新增两类接口：

- `POST /api/v1/roles/preview`: 接收 persisted role id 或临时 draft，返回 normalized manifest、effective manifest、execution profile、validation issues、inheritance/override 摘要
- `POST /api/v1/roles/sandbox`: 在 preview 通过后执行受控 probe，返回运行前 readiness、最终 prompt 摘要、选定 runtime/provider/model、以及非持久化测试结果

这样做的原因是：

- 继承、stricter security merge、advanced field normalization 都属于 Go 真相，前端本地复制一遍逻辑会再次漂移。
- preview 结果未来也可能被 CLI、自动化或其他产品面复用，不该只活在 React 组件里。
- role authoring 的关键问题是“最终生效的是什么”，不是“表单当前输入了什么”。

备选方案：

- 保持前端 preview，只新增更多字符串 summary。缺点是高级字段、override 与 trigger merge 几乎无法被可靠解释。

### 3. 角色 sandbox 用 Bridge `generate` 做轻量 probe，不直接启动完整 coding-agent run

角色测试沙盒需要真实、但不能过重。这里选择复用现有 TS Bridge `POST /bridge/generate` 作为 lightweight probe：Go 在完成 role preview 后，把 resolved role prompt 与受控 test input 组装成一次文本生成请求，附带 runtime/provider/model 与 readiness diagnostics。这个 probe 只返回样例输出和错误/诊断，不创建任务、不创建 worktree、不写 `agent_runs`。

这样做的原因是：

- 当前仓库已经有轻量生成入口，可以测试角色 prompt/语气/约束是否大体合理。
- 对 authoring 来说，probe 需要验证“这个角色会怎么说/怎么总结”，而不是立即执行完整编码任务。
- 避免把角色定义补全 change 扩大成新的 agent lifecycle / sandbox orchestration 变更。

备选方案：

- 直接调用完整 `/agents/spawn` 或 `/bridge/execute`。优点是更贴近生产；缺点是成本更高、验证更慢、环境要求更重，而且容易引入工作区副作用。
- 完全不调用模型，只做静态 preview。优点是稳定；缺点是无法兑现“角色测试沙盒”的核心价值。

### 4. 角色工作台升级为“三栏 authoring console”，把说明、YAML、sandbox 都放进同一流

当前 `RoleWorkspace` 已经是工作台式布局，但仍偏“表单 + 摘要”。新设计会把它升级为更完整的 authoring console：

- 左侧：role library / templates / sample roles
- 中间：结构化 role draft 分区编辑
- 右侧：字段说明、effective summary、YAML preview、sandbox 入口与结果

高级字段优先使用专门编辑控件，而不是一个大 JSON 文本框。说明内容来自 repo 内文档提炼后的 field guide，而不是临时写死的提示语。YAML preview 始终从当前 draft 或 preview 响应生成，帮助操作者理解最终落盘内容。

这样做的原因是：

- 用户要求“包括各种说明的”，这意味着 authoring 不是只有输入控件，还要有理解和校验辅助。
- 高级 schema 很多，继续把所有字段塞进一条滚动表单会让角色 authoring 迅速失控。
- YAML preview 和 sandbox 放在同一上下文里，才能让作者在“编辑 -> 看 effective result -> 试跑 -> 再修”之间形成闭环。

备选方案：

- 增加 raw YAML tab 让用户自己维护。优点是实现简单；缺点是回到手写 YAML，违背“产品内可完整定义角色”的目标。

### 5. 文档、样例角色和测试一起升级，避免 schema 与说明再次漂移

这次不会只改类型和页面。所有变化都要同步到：

- `docs/role-yaml.md`
- `docs/PRD.md` / `docs/part/PLUGIN_SYSTEM_DESIGN.md` 的当前能力表述
- 1-2 个 canonical sample roles
- Go role parser/store/preview/sandbox tests
- React role workspace / preview / sandbox tests

这样做的原因是：

- 角色定义是“文档驱动 + YAML 驱动 + UI 驱动”的三方系统，只改其中一边很快就会失真。
- 当前仓库已经发生过 schema 比 docs 新、UI 比 schema 旧的漂移，这次要在 change 内把它收敛掉。

备选方案：

- 先只改代码，文档后补。缺点是 user-facing guidance 又会继续滞后。

## Risks / Trade-offs

- [高级 schema 一次扩展过多，改动面会较广] → 先以 PRD 中已稳定、当前代码已有局部基础的字段为主，保留表达式型子字段的轻量动态性。
- [sandbox 依赖 runtime/provider/model readiness，环境不满足时用户可能无法 probe] → 将 readiness 诊断作为 sandbox 的一等输出，允许 preview 成功但 probe 被明确阻塞。
- [preview 和 save 都走 normalize 流后，旧 sample role 可能暴露兼容问题] → 先补 canonical sample 与 focused parser/store tests，再更新 authoring UI。
- [YAML preview 与 persisted YAML 可能因字段排序或默认值补全不同产生认知差异] → 明确区分 draft YAML、effective manifest 和 persisted canonical YAML，并在 UI 中标注语义。
- [PRD 中 marketplace/team-sharing/version-history 等愿景仍未完全交付] → 在 docs 和 spec 中明确这次只补 authoring completeness，不声称已完成生态化能力。

## Migration Plan

1. 扩展 Go 侧 `RoleManifest`、parser、store、handler 与 validation，使高级字段能被读取、保存、列出和规范化。
2. 新增 role preview/sandbox 服务与 API，先打通 normalized manifest / execution profile preview，再接入 lightweight Bridge probe。
3. 扩展前端 `RoleManifest`、`RoleDraft` 和 role workspace，加入高级字段编辑、说明、YAML preview 与 sandbox 面板。
4. 更新 sample roles 与角色文档，让仓库样例覆盖高级 schema 的 happy path。
5. 补齐 focused tests，并在 rollout 中以 role parser/store/API/workspace/sandbox 为最小验证面。

回滚策略：

- Preview/sandbox 作为新增接口与 UI 面板引入，不改变现有 `/api/v1/roles` 的基础 happy path；若 sandbox 不稳定，可单独下线 probe，保留 preview 与 authoring。
- 高级字段以向后兼容方式扩展 current manifest；旧角色若未声明新字段，仍应保持可读取与可执行。
- 若部分 advanced field round-trip 出现问题，可临时保留只读展示，不阻塞旧字段编辑和已有角色执行。

## Open Questions

- `knowledge.shared/private/memory` 的最小 MVP 是否全部结构化落地，还是先对外暴露字段并在 execution profile 中继续保持非执行态保留？
- `triggers` 的条件表达式是否先保持字符串约定，还是同时引入更严格的 schema/DSL 校验？
- sandbox probe 的默认 runtime/provider/model 应优先取项目 catalog 默认值、角色草稿字段，还是让操作者显式选择一次？
