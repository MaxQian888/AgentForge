## Why

AgentForge 现在已经有多条与 Skills 相关的成熟能力线: `role-skill-catalog` 负责角色引用, `role-skill-runtime` 负责运行时投影, `/marketplace` 负责 built-in 与远端 skill 的浏览/安装, `internal-skills.yaml` 与 `skill:verify:*` 则负责内部治理。但这些能力仍然分散在角色工作台、市场页面、YAML 注册表和本地脚本里, 仓库里没有一条真正的 Skills 管理主线来回答“当前有哪些受治理 skill、它们属于哪一类、状态是否健康、该去哪里继续操作”。

这导致维护者在补全或排查 Skills 时仍然要手动切换 `internal-skills.yaml`、`skills-lock.json`、`skills/builtin-bundle.yaml`、`pnpm skill:verify:*`、`pnpm skill:sync:mirrors`、`/marketplace` 和角色编辑器, 才能拼出完整真相。既然 AgentForge 已经把 Skills 当成 runtime、authoring、marketplace、workflow 都会消费的一等资产, 现在需要补一个 repo-truthful 的 Skills 管理工作台, 把治理、预览、校验、同步和下游 handoff 真正闭环。

## What Changes

- 新增独立的 `Skills` 管理工作台, 提供受治理 internal skills 的统一列表、筛选、分组、详情与状态面板, 覆盖 `built-in-runtime`、`repo-assistant`、`workflow-mirror` 三类 skill。
- 新增 Go 后端 skills-management API, 基于 `internal-skills.yaml`、`skills-lock.json`、`skills/builtin-bundle.yaml`、现有 skill package parser 和 bundle/verification seam 暴露统一的 inventory、detail、verification、mirror-sync 与 downstream readiness 数据。
- 在管理工作台中提供面向维护者的真实动作: 运行 internal/built-in verification、同步 workflow mirrors、查看 provenance/lock/bundle 对齐状态、识别 drift/blocker, 并跳转到 `/marketplace`、角色工作台或关联文档继续处理。
- 扩展 skill package preview contract, 让 `repo-assistant` 与 `workflow-mirror` skill 也能像 built-in/marketplace skill 一样展示 `SKILL.md` Markdown、frontmatter YAML、agent config YAML 和依赖/工具摘要。
- 为 Skills 管理链路补齐 focused tests, 覆盖 registry 聚合、family/provenance 状态、verification/mirror-sync 动作、preview detail 和 UI handoff 行为。

## Capabilities

### New Capabilities
- `skills-management-workspace`: 统一的 Skills 管理工作台与配套管理 API, 让维护者可以查看、校验、同步并追踪受治理 skill 的真实状态与下游消费面。

### Modified Capabilities
- `internal-skill-governance`: 从 CLI-only 的治理/验证规则扩展为可被 operator workspace 消费的 inventory、health、verification 与 mirror-sync 契约。
- `skill-package-preview`: 将结构化 skill preview 从 built-in/marketplace item detail 扩展到所有受治理 internal skill families, 保持 detail 渲染和诊断模型一致。

## Impact

- 前端会新增 `app/(dashboard)/skills` 及相关 `components/skills/*`、store、导航入口与 handoff 逻辑, 并复用现有 Markdown/YAML preview 组件而不是再造一套字符串渲染。
- 后端会新增或扩展 `src-go/internal/handler/*`、`src-go/internal/server/routes.go` 以及 skills governance 聚合逻辑, 把 registry、bundle、lockfile、preview 与 verification 结果收敛成稳定 API。
- 现有治理真相源 `internal-skills.yaml`、`skills-lock.json`、`skills/builtin-bundle.yaml` 继续保留为 source of truth, 但不再只通过脚本间接可见。
- 需要补 Go 单测与前端 Jest 覆盖, 验证 skill inventory/detail、verification/mirror-sync 返回、preview detail、页面状态与跨工作台 handoff。
