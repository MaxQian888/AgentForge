## Context

AgentForge 现在已经具备三类与本次 change 直接相关的真实资产：

- **平台控制面已存在**：Go 侧已经有 task、review、workflow 的 handler/service/store seam，前端也已经有 tasks / reviews / workflow operator workspace。也就是说，平台原生的“查任务、触发审查、跑工作流、查看运行历史/审批态”并不是未来概念，而是现有能力。
- **官方 built-in bundle 已存在**：`plugins/builtin-bundle.yaml`、`scripts/verify-built-in-plugin-bundle.js`、plugin panel readiness 体系已经能表达“哪些官方 built-ins 随仓库交付、是否可安装、为什么还不能运行”。
- **官方 starter 仍偏通用示例**：当前 ToolPlugin 主要是 `web-search` / `github-tool` / `db-query`，WorkflowPlugin 只有 `standard-dev-flow`。这些资产能证明插件系统可用，但还没有把 AgentForge 自己的核心平台特点——任务管理、Agent 编排、审查流水线、官方 workflow handoff——收敛成一套 repo-owned starter library。

这次 change 是跨插件资产、bundle 合同、验证脚本、文档基线和工作流运行约束的横切变更。重点不是“再做一个 generic MCP 示例”，而是把 repo 里已经存在的产品能力转成官方 starter catalog。

## Goals / Non-Goals

**Goals:**

- 为官方 built-in ToolPlugin 建立一组平台原生 starters，优先覆盖 task / review / workflow 这些 AgentForge 核心控制面。
- 为官方 built-in WorkflowPlugin 建立一组可直接代表核心交付链路的 starters，而不是只保留单一 demo。
- 扩展 built-in bundle metadata，让官方 starter 能表达 core-flow、依赖角色/服务、推荐 workspace/handoff 与 readiness guidance。
- 在不破坏现有 bundle/readiness/runtime 合同的前提下，把新 starters 接进现有验证和文档体系。

**Non-Goals:**

- 不在本次 change 内重做完整 visual workflow builder、marketplace 远程分发协议或新的插件运行时宿主。
- 不把所有 product surface 都包装成 ToolPlugin；只覆盖最能代表核心链路、且已有稳定 control-plane seam 的能力。
- 不引入新的 workflow process mode 或复杂 DAG 语义；优先复用当前稳定的顺序执行、review、approval 和已有 role 解析逻辑。
- 不改动 internal skill governance 这条已归档的 skill 资产治理线。

## Decisions

### 1. 内置工具采用“薄 MCP 适配层”，直接复用现有 Go control-plane seam

将新增一组 repo-owned ToolPlugin starters，例如 `task-control`、`review-control`、`workflow-control`。它们的职责不是直连数据库或绕过控制面，而是作为 **TS-hosted MCP adapter** 调用现有 Go API / service seam，输出适合 Agent 与 operator 使用的结构化结果。

这样做的原因：

- 平台真相已经在 Go handler/service/store；built-in tool 应复用现有授权、校验与 DTO，而不是复制业务逻辑。
- 这能让 ToolPlugin 更像“AgentForge 官方 MCP surface”，而不是仓库里另一套平行 API。
- 对验证也更友好：manifest、package validate、bundle metadata 与实际控制面 seam 可以一一对应。

备选方案：

- 让 built-in tools 直接读取数据库或文件系统。拒绝原因：会绕过现有控制面，破坏平台真相。
- 只继续保留 `web-search/github/db-query` 这类通用工具。拒绝原因：无法体现项目核心特点。

### 2. 工作流 starter library 先聚焦顺序型核心交付链路

本次 workflow starters 先以 **sequential-first** 为原则，保留现有 `standard-dev-flow`，并补至少两类平台原生 starters：

- `task-delivery-flow`：围绕 planner / coding / review 的任务交付链路。
- `review-escalation-flow`：围绕 deep review / approval pause / operator handoff 的审查升级链路。

这样做的原因：

- 当前 repo 已有 `planner-agent`、`coding-agent`、`code-reviewer` 等真实 role id，且 workflow step router 已支持 `agent`、`review`、`approval` 等动作。
- 顺序型 starter 可以最大化复用现有 runtime 和 step persistence，而不用把这次 change 扩大成 workflow engine 新语义开发。
- 先把“官方推荐起步路径”做实，比继续堆 process mode 更贴近用户请求。

备选方案：

- 直接新增 hierarchical / wave 官方 starters。暂不采用：虽然 runtime 已有部分实现，但当前 repo 更需要稳定、可讲清楚的核心 starter library。
- 用新 starter 替换 `standard-dev-flow` 原 id。暂不采用：会增加现有文档/资产迁移成本。

### 3. Bundle metadata 增加“核心流程归属 + 依赖引用 + 推荐入口”而不是只列 manifest

`plugins/builtin-bundle.yaml` 将从“官方条目清单”升级为“官方 starter catalog”，每个 ToolPlugin / WorkflowPlugin 条目除了现有 docsRef、verificationProfile、readiness 外，还应声明：

- `coreFlows`：例如 `task-delivery`、`review-automation`、`workflow-ops`
- `dependencyRefs`：依赖的 role ids、service/API seam、或必须存在的 runtime 约束
- `workspaceRefs` / `handoffRefs`：推荐从哪个 workspace 进入，或安装后应该跳转到哪个 operator surface
- `starterTier` / `starterFamily`：区分平台原生 core starter 与 generic helper

这样做的原因：

- AgentForge 的官方 built-ins 不只是“可安装”，还要告诉 operator 它属于哪段核心链路、依赖什么、下一步去哪用。
- 这些 metadata 能直接被验证脚本、plugin panel 和文档消费，不需要在多个地方重复 hardcode。

备选方案：

- 只在文档写推荐用法，不进 bundle metadata。拒绝原因：UI / 验证无法拿到结构化真相。

### 4. 验证脚本继续以 bundle 为真相源，并增加 starter-specific contract checks

`scripts/verify-built-in-plugin-bundle.js` 将继续作为官方 built-in plugin gate，但需要扩展检查：

- core-flow / dependencyRefs / workspaceRefs 等新增 metadata 是否完整
- 官方 core starters 是否都存在 manifest、docsRef、verificationProfile 与 readiness contract
- Workflow starters 的 role 引用、trigger mode 与 bundle metadata 是否一致
- ToolPlugin starters 的 package validate / MCP entrypoint 是否与声明的 control-plane scope 对齐

这样做的原因：

- 当前仓库已经有 bundle-level verification 习惯，继续沿这个 seam 增量扩展成本最低。
- 验证必须阻止“文档说是官方 starter，bundle/manifest 却对不上”的漂移。

备选方案：

- 另起一条完全独立的 starter verifier。暂不采用：会制造新的真相源。

## Risks / Trade-offs

- **[Risk] 新 built-in tools 范围过大，变成一套新的通用 API SDK** → **Mitigation:** starter 只覆盖 task / review / workflow 这些已有稳定 control-plane seam，并保持薄适配层。
- **[Risk] workflow starters 设计过于理想化，超出当前 runtime 边界** → **Mitigation:** 第一阶段只承诺 sequential + 现有 step action，避免引入新执行语义。
- **[Risk] bundle metadata 字段增加后，panel / docs / scripts 消费不一致** → **Mitigation:** 明确 bundle 为单一真相源，脚本校验先于 UI 适配。
- **[Risk] `standard-dev-flow` 与新 starters 职责重叠** → **Mitigation:** 保留其“最小 sequential demo”定位，并通过 starterFamily/coreFlows 与文档区分用途。

## Migration Plan

1. 先扩 proposal/spec contract，确认官方 starter library 的 capability 和 bundle metadata 形状。
2. 在实现阶段新增 ToolPlugin / WorkflowPlugin manifests，与现有 `standard-dev-flow` 一起组成官方 starter library。
3. 扩展 `plugins/builtin-bundle.yaml` 和 `scripts/verify-built-in-plugin-bundle.js`，让新 metadata 成为 hard gate。
4. 更新 `docs/PRD.md` 与 `docs/part/*.md`，把官方 starter catalog 对齐到产品核心链路描述。
5. 回滚策略：若新增 starter 实现不稳定，可保留 metadata contract 与 docs 变更，但先临时移除对应 bundle entry，使其不再被视为官方 built-in。现有通用 built-ins 与 `standard-dev-flow` 保持可用。

## Open Questions

- `review-control` 是否在第一阶段就暴露 approve / reject / request-changes 这类写操作，还是先只做 trigger + inspect？
- `standard-dev-flow` 是否保留为长期 public starter，还是后续作为 `task-delivery-flow` 的 minimal profile 保留？
- plugin panel 是否要在本次 change 内直接展示 `coreFlows` / `starterFamily`，还是先只完成 bundle + docs + verification，再由后续 UI change 消费？
