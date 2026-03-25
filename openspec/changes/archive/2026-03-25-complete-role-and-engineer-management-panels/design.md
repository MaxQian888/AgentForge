## Context

AgentForge 当前在角色与工程师配置这两条产品线之间存在一个明显的“前后端断层”。

- 角色侧已经有 PRD 对齐后的 YAML schema、Go 侧统一 role store、`/api/v1/roles` CRUD，以及 `roleId -> execution profile` 的运行时投影能力，但前端 `app/(dashboard)/roles/page.tsx` 与 `components/roles/role-form-dialog.tsx` 仍只覆盖少量字段，无法完整编辑 `version`、`extends`、knowledge、tool config、permission mode、path 约束等核心内容。
- 团队成员侧已经在数据库、Go model 和创建/更新请求里预留了 `agent_config` 与 `skills`，`members` 表也明确区分 `human` / `agent`；但当前 `components/team/team-management.tsx` 的创建与编辑流程只处理 `name / type / role / email / isActive` 这类浅层字段，甚至更新链路没有把 `skills` 真正写回数据库，也没有把 `agentConfig` 返回给前端做结构化编辑。
- `docs/PRD.md` 与 `docs/part/PLUGIN_SYSTEM_DESIGN.md` 明确把“数字员工可定制”“角色插件快速创建数字员工”“人机混合团队管理”定义为产品能力，而不是未来愿景；同时 `docs/part/PLUGIN_RESEARCH_ROLES.md` 还给出了角色模板、可视化编辑器、测试沙盒、版本管理、团队共享等 UX 方向。

因此，这个 change 的目标不是重新设计 Role YAML 或新增底层 runtime，而是把已经存在的角色/成员合同变成一套真实可操作的前端管理体验，并补上最小必要的 API/DTO 对齐。

## Goals / Non-Goals

**Goals:**

- 把角色页从“最小 CRUD 表单”升级为结构化角色工作台，覆盖 metadata、identity、system prompt、capabilities、knowledge、security、inheritance 的主要编辑面。
- 让角色创建支持模板起步、继承起步、版本字段维护，以及保存前的执行约束摘要预览。
- 把 Team Management 里的 Agent 成员创建/编辑升级为类型感知的工程师画像编辑流，覆盖 skills、role binding、agent profile 结构化字段与配置就绪度展示。
- 对齐 member API、DTO 与前端 store，使前端能够真实读取、编辑并回写 Agent profile，而不是继续停留在字符串占位。
- 保持人类成员与 Agent 成员共用一个 roster 入口，但让 Agent 特有配置不再被压扁成一行 `role` 文本。

**Non-Goals:**

- 不在本次中实现 Git 级角色版本历史、回滚分支或团队共享/fork 工作流。
- 不新增角色执行投影后端、Role YAML 解析规则或 Bridge runtime 协议。
- 不重做 Team 启动流水线，也不在本次里承诺完整的 provider catalog；已有和进行中的 provider/runtime 变更可以被本 change 消费，但不是本 change 的主交付。
- 不把成员管理改造成独立的 HR 系统；范围仍聚焦于项目级团队成员配置与运行前管理。

## Decisions

### 1. 角色页改为“工作台式编辑体验”，不继续堆大号 modal

选择把角色页从当前 `RoleFormDialog` 的小表单升级为更适合长表单的工作台结构：角色库列表 + 结构化编辑区 + 执行约束/预览摘要。

原因：

- 角色字段已经超出小型 modal 的可维护范围，继续往现有对话框里塞字段只会让操作和测试都迅速失控。
- 用户需要同时理解“当前角色是什么”“继承了谁”“保存后运行时大致会怎么执行”，这些信息更适合放在同一工作台里联动展示。
- 工作台更容易加入模板起步、复制、继承、版本提示和摘要对比，而不是在提交前靠用户自己脑补最终效果。

备选方案：

- 继续扩展现有 `RoleFormDialog`。缺点是信息密度过高、预览空间不足、后续字段继续增长时几乎必然退化成难用的大表单。

### 2. 在前端引入 `RoleDraft` / `AgentProfileDraft` 归一化层，而不是让 UI 直接操作原始 API 形状

选择在页面/组件层维护适配编辑体验的 draft 类型：

- `RoleDraft`：把 tags、paths、knowledge sources、tool lists 等字段统一成适合输入控件的结构。
- `AgentProfileDraft`：把 `agent_config` 从原始 JSON 字符串解析为结构化对象，并和 `skills`、`roleId`、活跃状态等页面字段一起管理。

原因：

- 当前 Go/TS 合同与前端输入控件的理想形状并不一致，直接双向绑定 API DTO 会让组件充满兼容判断。
- `agent_config` 目前在后端仍是字符串/JSONB 入口，前端需要一个安全的解析、默认值和回写边界，避免 UI 直接操作原始 JSON 文本。
- draft 层可以稳定承载校验、脏状态、模板预填、差异摘要等交互，而不污染 store 的服务端同步模型。

备选方案：

- 直接在 store 里保存 API 原始对象并让组件自行转换。缺点是每个组件都要重复处理兼容逻辑，难以保持一致。

### 3. 角色预览先在前端生成“执行摘要”，不新增专门的 preview API

选择先基于当前角色草稿在前端生成一个执行摘要，聚焦：

- metadata / version / extends
- role / goal / system prompt 摘要
- allowed tools、max turns、budget、permission mode
- review requirement、allowed/denied paths 等治理信号

原因：

- 预览需求主要是帮助操作者在保存前看清配置，而不是运行完整的 Go 侧 inheritance 解析链。
- 现有角色 API 已经返回足够多的字段支持大部分摘要场景，不需要为了预览再加一个新接口。
- 前端摘要可以先解决“看不懂自己配了什么”的高频问题；真正的执行投影仍由 Go 作为 source of truth。

备选方案：

- 新增后端 preview endpoint。优点是能拿到更权威的投影；缺点是本次会把范围扩大到新的 API 设计、测试和兼容成本。

### 4. Agent 成员继续挂在 `team-management` 能力下，但 UI 明确拆出 Agent 专属编辑区

选择保留统一 roster 和统一 Team 页面，不再拆出新的 “Engineer Center” 路由；同时在创建/编辑时根据成员类型切换字段分区：

- Human：保留简单资料编辑
- Agent：增加技能、角色绑定、Agent profile、配置摘要和就绪度提示

原因：

- PRD 强调的是“碳基 + 硅基统一管理”，拆成两个完全独立的入口会削弱这一点。
- 当前 roster、任务分配推荐、Agent activity 跳转都已经围绕成员模型构建，沿现有入口扩展是最贴近仓库真相的路径。
- 类型感知分区能避免 human flow 被 Agent 字段拖累，也避免 Agent 配置被隐藏到难以发现的二级页面。

备选方案：

- 为 Agent 成员新开独立页面。缺点是会割裂统一成员模型，也会增加导航与状态同步复杂度。

### 5. 成员 API 只做最小必要扩展：显式返回并接受 Agent profile，不重做存储模型

选择保持 `members.agent_config` 作为后端持久化入口，但补齐以下合同：

- `MemberDTO` 返回 `agentConfig`（或结构化投影后的等价字段）给前端
- `UpdateMember` 真正持久化 `skills` 与 `agentConfig`
- 前端只编辑“支持字段”的结构化视图；如未来还要支持更多 runtime/provider 配置，可在同一 profile 对象内渐进扩展

原因：

- 目前最大的断点是“后端有入口，前端没法用”，而不是存储模型本身不够用。
- 不做数据库迁移即可打通前后端闭环，适合本次前端补全 change。
- 这种最小扩展与进行中的 provider/runtime change 可以兼容，不必等待另一个 change 完成后再做 UI。

备选方案：

- 新增独立 `agent_profiles` 表或专门 API。缺点是把一个 UI 补全 change 扩成数据建模改造，超出当前必要范围。

### 6. 与 runtime/provider 方向保持“可接入、但不被阻塞”的边界

选择在 Agent profile 设计上为 `runtime/provider/model`、预算等字段预留入口，但不要求本次必须把整套 provider catalog 一并交付。若相关 capability 已在别的 change 中推进，本 change 直接消费；若暂未落地，本 change 允许先以受支持字段和 readiness 提示交付。

原因：

- 当前用户任务聚焦在“角色和工程师自定义/编辑部分”，不是完整 runtime 产品线整合。
- 仓库里已经存在单独的 provider/runtime 方向 change，强行并入本 change 会让范围失真。
- 预留兼容位可以减少后续返工，又不影响这次 proposal/apply 的聚焦性。

备选方案：

- 等 provider/runtime change 全完成后再做成员画像编辑。缺点是团队成员配置继续长期缺位。

## Risks / Trade-offs

- [角色编辑字段很多，页面可能显得重] → 用分区、摘要卡片、模板起步和预览 rail 降低首次编辑负担。
- [前端预览不一定完全等同于 Go 最终 execution profile] → 明确其语义是“保存前摘要”，权威执行投影仍以后端为准。
- [`agent_config` 当前没有稳定公开 schema] → 以受支持字段的 typed draft + 容错解析为主，未知字段保留但不强制编辑。
- [与进行中的 provider/runtime 变更存在边界接触] → 在 spec 和 tasks 里明确本 change 不负责重做 provider catalog，只消费可用合同并对缺失状态给出提示。
- [成员 DTO 扩展后会触达 dashboard/team 归一化逻辑] → 通过 focused tests 锁住 human/agent roster、create/edit、summary 展示与回写行为。

## Migration Plan

1. 先补齐 member DTO / store / normalization，使前端能够真实读取与回写 Agent profile 和 skills。
2. 重构 Team 管理表单为类型感知的创建/编辑流，并增加 Agent 成员摘要与就绪度展示。
3. 将角色页从小表单升级为工作台式编辑流，加入模板/继承起步与执行摘要预览。
4. 为关键流程补齐 focused tests，覆盖 role draft、agent member profile draft、create/update 提交、summary cues 与回归场景。

回滚策略：

- 本次不涉及数据库迁移；若需要回退，可直接回退前端组件、store 与 DTO 扩展代码。
- 若 member DTO 扩展影响到其他消费方，可暂时保留后端返回兼容字段并回退前端使用路径。

## Open Questions

- Agent profile 在本 repo 当前阶段是否直接暴露 `runtime/provider/model` 选择，还是先只交付 `roleId + budget/notes/skills` 这类更稳的字段集合？
- 角色预览是否需要在第二阶段加入“继承后最终生效值”的服务端解析能力，还是先维持前端摘要 + 原始字段可见即可？
- Team roster 的“就绪度”最低判定规则应以哪些字段为准：绑定角色即可，还是还要包含运行时默认项与预算限制？
