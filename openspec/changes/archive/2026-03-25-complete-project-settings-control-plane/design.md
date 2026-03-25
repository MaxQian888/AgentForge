## Context

AgentForge 当前已经有一个可用但明显偏薄的项目设置入口：`app/(dashboard)/settings/page.tsx` 可以修改项目名称、描述、仓库信息和 coding-agent 默认值，并通过 `app/(dashboard)/settings/page.test.tsx` 覆盖最基础的 runtime catalog 保存流程。但 repo 真相同时也很明确：

- 前端 settings 页面没有承接 PRD 里已经明确写出的项目级治理配置，例如任务/Sprint/项目预算阈值、告警阈值、审查升级与人工审批策略。
- `lib/stores/project-store.ts` 和 `src-go/internal/model/project.go` 现在都只把 `settings` 视为 `codingAgent` 一段配置，没有为更完整的项目级 settings document 预留稳定结构。
- `docs/PRD.md` 明确写了“项目配置要求人工审批 | 在项目 settings 中可配置”，并把预算上限、告警阈值、项目级治理视为正式产品能力，不是可选备注。
- `docs/part/AGENT_ORCHESTRATION.md` 又进一步把 `default_task_budget_usd`、`default_sprint_budget_usd`、`default_project_monthly_budget_usd`、`alert_threshold_*`、`max_active`、`warm_pool_size`、`max_queue_size` 等治理参数写成真实配置面。

因此，这次设计不是单纯给 settings 页面加几个输入框，而是要把“项目级治理配置”收敛成一个可显示、可编辑、可保存、可被现有 review/runtime 逻辑消费的统一控制面。

## Goals / Non-Goals

**Goals:**
- 把项目 settings 从“基础信息 + coding-agent 默认值”升级为结构化的项目级 settings control plane。
- 让前后端共同支持一个向后兼容的项目设置文档，覆盖基础信息、仓库、coding-agent、预算/告警、审查策略和必要的通知偏好。
- 让 settings 页在展示层面具备完整 operator summary，而不是让用户只能逐卡片猜测当前项目到底启用了哪些治理规则。
- 保持现有 `coding-agent-provider-management` 的 runtime catalog、resolved default 和 diagnostics 语义不变。
- 让项目级 review policy 真正进入 `deep-review-pipeline` 的路由决策，而不是继续停留在文档承诺。

**Non-Goals:**
- 不在本次 change 中重做 workflow 配置页、agent monitor 页或 cost overview 页；这次只补齐项目级 settings 控制面及其必要后端接线。
- 不把项目设置扩成组织级/团队级全局策略中心；所有新配置都仍以 project 为边界。
- 不在这次 change 中引入新的独立配置服务或额外数据库表；优先复用现有 `projects.settings` JSON 存储。
- 不在这次 change 中做完整的 IM 平台矩阵化通知中心；只处理与项目治理直接相关的通知偏好/摘要。

## Decisions

### Decision: Expand `projects.settings` into one backward-compatible structured settings document

项目设置继续存放在现有 `projects.settings` JSON 字段里，但从仅有 `codingAgent` 扩展为结构化文档。建议的逻辑分层是：

- `codingAgent`: 保留现有 runtime/provider/model 选择。
- `governance.budget`: 默认任务预算、默认 Sprint 预算、默认项目月度预算，以及任务/Sprint/项目告警阈值。
- `governance.review`: 是否强制人工审批、人工审批触发阈值（如大变更行数）、以及 deep review 升级相关 project policy。
- `governance.notifications`: 与预算/审查升级直接相关的通知偏好和摘要开关。

这样可以避免引入新表或破坏现有 `PUT /api/v1/projects/:id` 更新路径，同时允许 legacy 项目在读取时自动补齐默认值。

备选方案一是把每个治理模块拆成独立表或独立 API；这会让当前 repo 的项目设置保存流程割裂，也会扩大 apply 范围。备选方案二是前端本地拼接配置、后端仍只存 `codingAgent`；这会让“可配置”继续停留在 UI 幻象，因此不采用。

### Decision: Return one server-authored settings summary alongside editable settings

为了满足“信息显示完整”，settings 页面不应由前端自行推导当前 runtime readiness、预算姿态或审查路由摘要。后端在返回项目 DTO 时，应同时返回一个 `settingsSummary` 或等价派生摘要，至少覆盖：

- 当前 resolved coding-agent 选择及其可用性/阻塞诊断。
- 当前预算治理基线（task/sprint/project）与阈值摘要。
- 当前 review policy 是否会让 approve 结果进入人工审批。
- 当前配置里存在的 fallback/defaulted 项。

这样 settings 页面可以渲染 summary rail 或摘要卡片，并与实际后端语义保持一致。

备选方案一是纯前端从 DTO 和 catalog 自己推导全部摘要；这会把业务规则复制到页面中，未来极易和后端 drift。备选方案二是拆多个读取接口；这会增加页面装配复杂度和一致性风险，因此不采用。

### Decision: Keep the current `/settings` route but turn it into a sectioned operator workspace

不新增 settings 子路由或 modal 流程，而是在现有 `app/(dashboard)/settings/page.tsx` 上扩展为分区式工作区：

- 顶部保留项目级标题与保存状态。
- 主体按 `General`、`Repository`、`Coding Agent`、`Budget & Alerts`、`Review Policy`、`Notifications` 分段。
- 右侧或页底提供统一 summary / diagnostics 区域。
- 使用统一 dirty state、field validation 和 save/reset 体验，而不是每个卡片各自保存。

这样可以最大化复用当前页面与测试入口，也更贴合 repo 现有 dashboard 信息架构。

备选方案一是改成 tab/sub-route 结构；这更重，也会引入额外 URL 状态管理。备选方案二是继续维持独立卡片各自保存；这会让配置一致性和用户心理模型都变差，因此不采用。

### Decision: Project review policy becomes an input to review routing, not a UI-only hint

项目设置里的 review policy 必须被 `deep-review-pipeline` 消费。具体来说：

- deep review 完成后的后续路由，需要读取项目 settings 中的人工审批开关和阈值。
- 当项目配置要求人工审批时，即使 Layer 2 推荐为 `approve`，也应进入 pending approval，而不是直接自动完成。
- 当项目未开启该策略时，现有自动通过路径保持不变。

这让 settings 页的“审查策略”成为真实产品行为，而不是不会生效的说明文本。

备选方案一是先只做持久化，不接入 review pipeline；这会继续制造“可以配置但不生效”的产品断层。备选方案二是把审查策略直接写死在 review service；这和本次补 settings 控制面的目标相冲突，因此不采用。

### Decision: Unified save keeps coding-agent semantics stable during broader settings updates

现有 `coding-agent-provider-management` 已经定义了 runtime catalog、resolved defaults 和 diagnostics 语义。新的 unified settings save 必须保证：

- 用户保存预算或 review 配置时，不会意外覆盖现有 coding-agent 选择。
- legacy 项目即使没有治理配置，也仍能收到同样的 coding-agent fallback default。
- settings 页面仍然从服务端 catalog 渲染 runtime/provider/model，而不是退回本地常量。

备选方案是把 coding-agent 部分继续独立提交；这会让 settings 页面重新裂成两套保存路径，不利于长期维护，因此不采用。

## Risks / Trade-offs

- [Settings schema grows quickly and becomes hard to reason about] → 通过明确的 nested DTO 分区、服务端默认填充和 summary DTO 控制复杂度，而不是把所有字段平铺在一个大对象里。
- [Legacy projects only store `codingAgent`, causing blank states or save regressions] → 读取时始终补齐 documented defaults，保存时做 merge 而不是全量覆盖原始 JSON。
- [Review policy is visible in UI but not honored consistently downstream] → 在同一 change 中修改 `deep-review-pipeline` spec，并把 project policy 读取接到 review routing 边界上。
- [Operator summary drifts from real runtime/governance state] → 摘要由后端 author，而不是页面本地拼装；前端只做展示。
- [The settings page becomes too dense] → 用 sectioned layout + summary rail 控制阅读负担，避免把所有字段堆成单列长表单。

## Migration Plan

1. 扩展 Go 侧 `ProjectStoredSettings` / `ProjectSettingsDTO`，为治理配置与摘要建立向后兼容结构。
2. 更新项目读取/保存逻辑：legacy JSON 读取时注入默认值，保存时做分区 merge。
3. 在项目 API 响应中补充 settings summary，并保持 coding-agent catalog 的现有输出格式。
4. 扩展 `lib/stores/project-store.ts` 与 `app/(dashboard)/settings/page.tsx`，把页面升级为统一 settings workspace，并补对应测试。
5. 将 project review policy 接入 deep review follow-up routing，让人工审批/升级策略真正生效。
6. 通过 focused tests 验证：legacy project fallback、unified save、coding-agent defaults 不回退、review policy 路由生效。

回滚策略：
- 如果 unified settings save 产生兼容性问题，可以在后端暂时忽略新增治理字段，只继续消费 `codingAgent`，而不破坏已有项目读取。
- 因为仍复用 `projects.settings` JSON 字段，本次不需要不可逆数据库迁移；回滚主要是代码路径回退，而不是数据结构回滚。

## Open Questions

- 第一版 notifications 是否只覆盖 in-product / IM 两类治理通知偏好，还是需要细到具体 channel/platform？
- `max_active` / `warm_pool_size` / `max_queue_size` 这类 agent-pool 参数是否应直接进入项目 settings 第一版，还是先只在 summary 中显示文档默认值并把可编辑范围收敛到预算与审查策略？
- 人工审批阈值第一版是否只做布尔开关 + 大变更行数阈值，还是也要纳入“高风险文件类型”这类更复杂规则？
