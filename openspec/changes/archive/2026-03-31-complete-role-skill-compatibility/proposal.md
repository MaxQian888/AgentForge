## Why

AgentForge 现在已经具备 repo-local skill catalog、结构化 role skills authoring，以及 runtime skill projection，但这条链路还没有把 role 和 skill 的兼容性真正定义完整。`SKILL.md` frontmatter 中的 `requires` 与 `tools` 已经会进入 runtime bundle，可 role workspace、preview/sandbox 和 spawn readiness 仍主要停留在“这个 skill 能不能找到”，没有 authoritative 地解释“这个 role 的 allowed tools、继承后的 skill tree、以及 skill 依赖闭包是否真的兼容可执行”。

这导致当前角色和技能的结合仍然存在产品真相缺口: 操作者可以成功保存或预览一个挂了 skills 的 role，却看不清隐藏依赖、技能声明的工具需求、以及哪些组合会在真正执行时变成不完整配置。现在需要把这条 seam 单独补齐，让 role + skill 成为可解释、可校验、可阻断不兼容组合的完整能力，而不是只把 skills 当作可保存的引用列表。

## What Changes

- 扩展 role-skill catalog 和 authoring 相关 metadata，让 catalog entries 暴露 skill 的 direct dependency paths、声明工具需求，以及足够的 compatibility context，而不再只显示 path、label 和静态部件计数。
- 在 Go role execution profile、preview/sandbox readiness、以及 agent/workflow 启动前的 role projection 中增加 role-skill compatibility evaluation，先把当前 role tools 归一化到与 skill frontmatter 可比较的能力标识，再校验 auto-load 技能及其依赖闭包是否需要当前 role 未覆盖的工具，并区分 blocking 与 warning 级别诊断。
- 更新 role workspace、role library 和 review/context rail，使操作者能在 authoring 流里看见 direct skills、transitive loaded skills、声明工具需求、兼容性状态，以及哪些问题只是手动 on-demand inventory warning，哪些会阻断实际执行。
- 保持手动 skill path 和现有 runtime bundle 投影模型不变，但补充兼容性 guidance、样例 role/skill 对齐、以及 focused tests，覆盖 tool mismatch、dependency-closure visibility、preview/sandbox compatibility feedback 和 spawn-time blocking 行为。

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `role-skill-catalog`: catalog entries need to expose role-authoring metadata for dependency and declared-tool compatibility, not just discovery labels and source roots.
- `role-skill-runtime`: runtime projection must evaluate whether auto-load skills and their dependency closure are compatible with the role's effective tool grants, and surface blocking versus warning diagnostics accordingly.
- `role-management-panel`: the role workspace and library must show skill dependency, tool-demand, and compatibility cues so operators can understand the effective role-skill combination before saving.
- `role-authoring-sandbox`: preview and sandbox feedback must explain role-skill compatibility state, including transitive skill loading, declared tool requirements, and which mismatches will block execution.

## Impact

- Affected backend seams: `src-go/internal/role/*`, `src-go/internal/model/role.go`, `src-go/internal/handler/role_handler.go`, `src-go/internal/service/agent_service.go`, and workflow/runtime paths that currently rely on execution-profile skill diagnostics.
- Affected frontend role surfaces: `lib/stores/role-store.ts`, `lib/roles/role-management.ts`, `components/roles/*`, and `app/(dashboard)/roles/*` where skill metadata, review context, and role library summaries are rendered.
- Affected assets and docs: `skills/**/SKILL.md`, sample roles under `roles/`, `docs/role-authoring-guide.md`, and `docs/role-yaml.md` so documented role-skill semantics match compatibility behavior.
- Affected verification: focused Go, frontend, and possibly bridge/runtime contract tests for skill dependency visibility, tool compatibility diagnostics, preview/sandbox feedback, and spawn-time blocking rules.
