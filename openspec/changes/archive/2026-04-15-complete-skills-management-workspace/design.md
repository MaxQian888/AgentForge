## Context

AgentForge 的 Skills 能力已经沿着多条 seam 分别落地:

- `src-go/internal/role/skill_catalog.go`、`skill_runtime.go`、`role_handler.go` 负责 `skills/*` 的 role authoring 与 runtime 消费。
- `src-go/internal/handler/marketplace_handler.go`、`marketplace_built_in_skill_bundle.go` 与 `/marketplace` 页面负责 built-in/marketplace skill 的浏览、preview 与安装 handoff。
- `internal-skills.yaml`、`skills-lock.json`、`skills/builtin-bundle.yaml`、`scripts/internal-skill-governance.js`、`scripts/verify-built-in-skill-bundle.js` 负责 repo-managed skills 的治理与验证。
- 侧边栏与 App Router 当前没有独立的 `/skills` 工作台; `components/layout/sidebar.tsx` 的 configuration group 只有 `/roles`、`/plugins`、`/marketplace`、`/settings`、`/im`、`/docs`。

因此当前缺的不是单点 parser 或单个按钮, 而是一条把“治理真相”和“产品使用面”连接起来的 operator workflow: 维护者仍然需要在多个页面、多个 YAML 文件和多个 CLI 命令之间来回切换, 才能搞清一个 skill 当前是否被 bundle 收录、是否有 lock provenance、是否 mirror drift、是否能被角色引用、是否已在 marketplace 中可浏览。

另一个约束是 runtime 不能倒退成“后端调用 Node 脚本来补 API”。现有治理验证主要实现于 Node 脚本, 但 AgentForge 的 Go backend/Tauri/desktop 链路不能把 Node 进程当成稳定服务依赖, 所以新的 Skills 管理 API 必须以 Go-native 聚合层为主, Node 脚本继续保留为 CLI verifier。

## Goals / Non-Goals

**Goals:**

- 提供独立的 `/skills` 工作台, 统一呈现 `built-in-runtime`、`repo-assistant`、`workflow-mirror` 三类 skill 的 inventory、family、provenance、health 与 downstream consumer state。
- 提供 Go-native skills management API, 直接消费 `internal-skills.yaml`、`skills-lock.json`、`skills/builtin-bundle.yaml` 和 skill package 文件系统真相。
- 让管理工作台可以执行真实且有限的动作: verification、workflow mirror sync、以及跨到 `/marketplace`、`/roles`、关联 docs 的 handoff。
- 将结构化 skill preview 扩展到所有受治理 skill families, 保持 Markdown/YAML/agent config/detail 视图一致。
- 为治理聚合、action endpoint 和前端 workspace 补 focused tests, 避免页面只靠乐观本地推断。

**Non-Goals:**

- 不在本次中提供 SKILL.md、agent yaml、registry yaml 的浏览器内编辑器或可视化 authoring 表单。
- 不把 upstream-synced skill 的“远端拉取/更新 lock hash”自动化成 UI 按钮; `upstream-sync` 仍按文档和现有维护流程更新。
- 不把 `/skills` 变成远端 marketplace 的替代页; marketplace 继续负责 publish/install/review/feature 这类市场生命周期。
- 不替换现有 `pnpm skill:verify:*` / `pnpm skill:sync:mirrors` 命令; 它们继续存在, 但其底层 contract 要与新 API 共用同一份真相。

## Decisions

### 1. 新增独立的 `/skills` 工作台, 不把治理能力继续塞进 `/roles` 或 `/marketplace`

`/roles` 关注“如何把 skill 用到角色里”, `/marketplace` 关注“如何浏览与消费 skill 资产”, 而这次缺的是“如何管理仓库里受治理的 skill 本身”。因此新增独立的 `/skills` 页面, 并把它放进现有 sidebar 的 configuration group, 与 `/roles`、`/plugins`、`/marketplace` 并列。

UI 结构沿用仓库已有 workspace 模式:

- 左侧 inventory/filter rail: 按 family、status、sourceType、downstream consumer state 分组筛选。
- 中间 list/summary pane: 显示 skill 卡片或表格, 聚合 canonical root、docsRef、bundle/lock/mirror 状态和支持的动作。
- 右侧 detail/action pane: 展示 Markdown/YAML preview、governance diagnostics、mirror targets、consumer surfaces 与操作按钮。

这样做的原因:

- 把治理与消费分开, 能避免 `/marketplace` 同时承担“远端市场”和“内部治理控制台”两种语义。
- 当前 sidebar 已有相同级别的 operator pages, 新增 `/skills` 的信息架构成本低。
- 任务工作台、agent 工作台已经证明这种 list/detail/action layout 在当前设计系统里可复用。

备选方案:

- 继续扩展 `/marketplace`。拒绝原因: 那个页面的主 contract 是 item browse/install/moderation, 不是 internal registry governance。
- 把治理动作塞进 `/roles`。拒绝原因: 角色作者不一定需要管理 mirror/lock/bundle, 语义会混乱。

### 2. 新增 Go-native skills management service/API, 不从后端 shell out 到 Node verifier

新增一层 Go-native governed skill 聚合服务, 负责:

- 读取 `internal-skills.yaml`、`skills-lock.json`、`skills/builtin-bundle.yaml`
- 扫描 skill canonical roots 与 mirror targets
- 生成统一 inventory/detail DTO
- 执行 verification 与 workflow mirror sync
- 汇总 downstream surfaces, 比如 role catalog availability、marketplace built-in membership、docsRef、mirrorTargets

推荐 API 形态:

- `GET /api/v1/skills`: 返回受治理 skills 列表, 支持 `family`、`status`、`sourceType` 等过滤
- `GET /api/v1/skills/:id`: 返回单个 skill 的 detail、preview、diagnostics、actions、handoff targets
- `POST /api/v1/skills/verify`: 运行 internal/built-in verification, 返回按 family/skill 聚合的结果
- `POST /api/v1/skills/sync-mirrors`: 只对 `workflow-mirror` skills 执行同步, 返回 updated targets 与 remaining drift

这样做的原因:

- Go backend 是稳定的 operator/runtime entrypoint; Node 脚本适合作为 maintainer CLI, 不适合作为运行时服务依赖。
- 现有验证脚本已经定义了 family/sourceType/exception/bundle/lock 的 contract, 这些规则可以在 Go 侧按同样真相重建, 而不是在 API 层再走一次 subprocess。
- 把 API 设计成 inventory/detail/action 三类 endpoint, 更符合当前 dashboard store 模式, 也更容易测试和缓存。

备选方案:

- 由 Go handler 调 `node scripts/internal-skill-governance.js --json`。拒绝原因: 运行时依赖 Node, 且对 desktop/backend-only 场景不稳。
- 只提供只读 inventory, verification 继续纯 CLI。拒绝原因: 这样 `/skills` 仍然不是完整管理面。

### 3. 将 skill package preview 泛化为“任意 canonical root 下的受治理 skill preview”

当前 `src-go/internal/role/skill_preview.go` 与 `skill_package.go` 假设 canonical root 是 `skills/*`, 适合 built-in runtime skills, 但不够表达 `.agents/skills/*` 与 `.codex/skills/*`。本次将 preview loader 泛化为 family-aware 的 package preview seam:

- 输入改为 registry entry 或 canonical root + source root, 而不是只接受 `skills/<id>`
- 输出仍保持统一 preview DTO: Markdown body、frontmatter YAML、agent config YAML、requires、tools、availableParts、reference/script/asset counts
- built-in runtime、repo-assistant、workflow-mirror 统一复用这套 preview 结构
- `src-marketplace` 保持 artifact-backed preview shape 与之同构, 不改变它的远端职责

这样做的原因:

- `/skills` detail 不能只对 built-ins rich preview, 对 repo-assistant/workflow-mirror 又退回纯元数据摘要, 那会造成新的“半成品 surface”。
- 当前 preview DTO 已经足够好, 真正缺的是 root/source 的泛化, 而不是再发明第三套 preview model。

备选方案:

- 只给 built-in runtime skills 显示 preview。拒绝原因: 管理工作台会对另外两类 skill 丢失最重要的上下文。
- 为每个 family 各做一套 detail renderer。拒绝原因: 维护成本高, 且违背“SKILL.md package 是统一资产”的真相。

### 4. 动作模型保持“真实且有限”: verify、sync、handoff, 其余能力显式 blocked

管理工作台只暴露当前仓库已经有真实支撑的动作:

- verify internal skills
- verify built-in runtime subset
- sync workflow mirrors
- open downstream consumer surfaces: `/roles`, `/marketplace`, docsRef, canonical root reference

对没有现成稳定 contract 的动作, 页面必须给出 blocked/not-supported state, 例如:

- `upstream-sync` skill 的 remote refresh
- in-browser 编辑 registry/lock/bundle/skill files
- repo-assistant 或 workflow-mirror skill 的 marketplace install

这样做的原因:

- 用户明确要求“功能完整, 不要省略或者简化”, 真正完整不等于把不存在的后端 contract 硬做成假按钮, 而是把“支持什么/不支持什么/下一步去哪”说清楚。
- 当前已有 specs 很强调 truthful state, blocked reason 与 downstream handoff; 这个 workspace 应沿用同一原则。

备选方案:

- 把未来可能有的动作也先做按钮。拒绝原因: 会制造伪能力。
- 所有动作都隐藏, 只显示读状态。拒绝原因: 不符合 management workspace 目标。

### 5. Verification 与 mirror sync 的结果模型采用“快照 + per-skill diagnostics”, 不只回布尔值

`/skills` 工作台需要的不只是 `ok: true|false`, 而是能解释:

- 哪个 family/skill 出问题
- 问题是 bundle drift、lock missing、mirror drift、missing agent config, 还是 preview parse failure
- action 执行后哪些 target 被更新, 哪些 remain unchanged

因此 action response 与 inventory/detail DTO 都应包含 per-skill diagnostics, 推荐维度包括:

- `status`: healthy / warning / blocked / drifted
- `issues[]`: code、message、targetPath、family、sourceType
- `supportedActions[]` 与 `blockedActions[]`
- `consumerSurfaces[]`: role-skill-catalog / marketplace / docs / workflow-mirror targets

这样做的原因:

- 现有脚本已经按 skill 维度报告 failure, UI 不应把它压扁成一个总开关。
- 前端可以稳定渲染 list badge、detail diagnostics 和 action aftermath, 不需要靠 stderr 文本解析。

备选方案:

- 只返回 CLI 风格字符串日志。拒绝原因: 不适合作为稳定 UI contract。

## Risks / Trade-offs

- [Go 侧重建治理规则可能与现有 Node verifier 漂移] → Mitigation: 保持同一份 registry/bundle/lock truth, 并为 Go action responses 与 Node verifier fixtures 建立对照测试。
- [将 preview 泛化到 `.agents/skills` 与 `.codex/skills` 可能暴露更多 malformed package 边界] → Mitigation: detail 与 inventory 都使用显式 preview-unavailable/diagnostic state, 不把预览失败误报成 package 不存在。
- [新增 `/skills` 页面会带来导航、i18n 与 responsive 布局工作量] → Mitigation: 复用现有 dashboard shell、workspace patterns 与 sidebar i18n key 结构, 不重新发明页面骨架。
- [workflow mirror sync 是文件系统写操作, 在不同运行宿主下要避免误操作] → Mitigation: 只允许针对 registry-declared workflow-mirror targets 执行, 并在响应中返回实际变更的 target 列表与未改动原因。
- [工作台同时展示 built-in/runtime、repo-assistant、workflow-mirror 三类 skill, 信息密度高] → Mitigation: 默认按 family/status 分层, 详情面板承担重信息, 列表面只放决策所需摘要。

## Migration Plan

1. 先新增 Go-native governed skill 聚合层与最小 API contract, 让 inventory/detail/verify/sync 都有稳定响应。
2. 把现有 preview loader 从 `skills/*` root 假设中解耦, 让三类 internal skill families 都能生成统一 preview。
3. 新增 `/skills` 页面、store、detail pane 与 sidebar 入口, 先接 inventory/detail/handoff。
4. 再接 verify/sync action, 明确 success/blocked/drift diagnostics, 并补 action regression tests。
5. 最后补 cross-surface handoff 与 docs/i18n, 确保 `/skills`、`/marketplace`、`/roles` 之间的跳转与状态一致。

回滚策略:

- 若 `/skills` UI 本身有问题, 可以先隐藏导航入口, 保留 Go API 与聚合测试, 不影响现有 `/marketplace`、`/roles` 和 CLI verifier。
- 若 verify/sync action 的写路径不稳定, 可以先保留只读 inventory/detail 与 blocked action state, 暂时关闭写操作 endpoint, 而不回退整个工作台。

## Open Questions

- canonical root 打开/定位动作是否需要在 desktop 模式下接 Tauri 文件系统能力, 还是第一阶段只显示路径与 docsRef; 本次更倾向于先保持路径可见和 handoff 明确, 不把文件管理器集成绑进首版范围。
- `GET /api/v1/skills` 是否需要支持“只返回 drifted/blocked 项”这类服务端过滤, 还是由前端先基于 inventory 本地筛选; 如果 inventory 规模继续保持几十项量级, 首版可先本地筛选, 只保留 family/sourceType 这类高价值 query 参数。
