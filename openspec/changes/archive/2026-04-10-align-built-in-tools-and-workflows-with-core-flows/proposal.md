## Why

AgentForge 的产品差异已经明确落在“IM → 任务 → 编码 → 审查 → 通知”的开发管理闭环，以及 Planner/Coder/Reviewer、多层审查、MCP 工具化控制面这些核心链路上。但当前官方 built-in plugin bundle 里的 ToolPlugin 仍主要是 `web-search` / `github-tool` / `db-query` 这类通用工具，WorkflowPlugin 也只有一个 `standard-dev-flow` 顺序 starter，无法代表仓库已经具备的任务、审查、工作流与调度控制面能力。

现在补这条线，是因为仓库里的 task/review/workflow handler、service、workspace 与 built-in bundle/readiness/verification 体系都已经存在；如果继续只交付通用示例型 built-ins，插件系统、工作流面板和官方 starter catalog 会长期偏离 AgentForge 自己的核心产品特点，也会让后续 role、marketplace 和 operator 体验缺少统一的“官方推荐起步路径”。

## What Changes

- 引入一组 repo-owned 的官方 built-in ToolPlugin starters，把现有 Go/TS control-plane seam 封装成平台原生 MCP 工具，优先覆盖任务协作、审查控制、工作流运行/审批与项目上下文查询这些 AgentForge 核心动作。
- 引入一组 repo-owned 的官方 built-in WorkflowPlugin starters，不再只保留单一的 `standard-dev-flow` 示例，而是补齐能代表核心交付链路的任务交付 / 编码审查 / 审查升级等工作流模板。
- 扩展 built-in bundle contract，让 ToolPlugin / WorkflowPlugin 官方条目声明 core-flow 分类、依赖的 role/service seam、推荐 workspace/handoff、以及 readiness 与 docs 元数据，避免官方 starter 只是一组孤立 manifest。
- 调整 built-in workflow runtime contract，使仓库维护的官方 workflow starters 可以被当作“平台内置工作流库”而不是“单个演示 starter”，并对 role/service 依赖、触发方式与执行边界给出更明确约束。
- 更新验证与文档基线，让 built-in bundle、starter manifests、脚本校验和插件系统文档共同反映 AgentForge 的核心产品闭环，而不是继续停留在通用 MCP / workflow 示例层。

## Capabilities

### New Capabilities
- `built-in-control-tools`: 定义 AgentForge 官方内置 ToolPlugin starters，覆盖任务、审查、工作流与项目上下文等平台原生控制面 MCP 工具合同。
- `built-in-workflow-starters`: 定义 AgentForge 官方内置 WorkflowPlugin starter library，覆盖任务交付、编码→审查交接、审查升级等核心交付链路模板。

### Modified Capabilities
- `built-in-plugin-bundle`: 官方 built-in bundle 需要从“列出 manifest”升级为“表达核心流程归属、依赖关系、readiness 与推荐入口”的 curated starter catalog。
- `workflow-plugin-runtime`: 官方 built-in workflow 支持需要从单一 starter 扩展为平台内置 starter library，并对可执行触发、role/service 依赖与运行边界给出更严格合同。

## Impact

- Affected built-in assets: `plugins/builtin-bundle.yaml`, new or updated manifests under `plugins/tools/**` and `plugins/workflows/**`, plus any companion package/runtime files required by those built-ins.
- Affected control-plane seams: existing task/review/workflow/project-context APIs and services in `src-go/internal/**`, corresponding TS Bridge / MCP adapter seams, and any operator-facing starter metadata consumed by plugin/workflow surfaces.
- Affected verification tooling: `scripts/verify-built-in-plugin-bundle.js` and adjacent starter validation / readiness checks.
- Affected docs: `docs/PRD.md`, `docs/part/PLUGIN_SYSTEM_DESIGN.md`, `docs/part/AGENT_ORCHESTRATION.md`, `docs/part/REVIEW_PIPELINE_DESIGN.md`, plus any maintainer guidance for official built-in starters.
