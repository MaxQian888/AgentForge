## Context

AgentForge 当前仓库里至少存在三条不同的 internal skill seam：

- `skills/*` 是产品真相的一部分，会被 Go role catalog、runtime skill projection、marketplace built-in bundle 和 `pnpm skill:verify:builtins` 消费。
- `.agents/skills/*` 是 repo-local 开发技能集合，既有仓库自写技能，也有 `skills-lock.json` 记录来源的 upstream-imported skills；它们的 frontmatter、agent config 文件名、references/scripts/tests 形态并不统一。
- `.codex/skills/*`、`.claude/skills/*`、`.github/skills/*` 当前保存了同一类 OpenSpec workflow skills 的多份镜像，但仓库内没有 authoritative registry 或自动 sync/verify 机制来证明这些副本仍一致。

这意味着“SKILL.md 已经存在”并不等于“这个 skill 是规范、可维护、可验证的仓库资产”。当前唯一稳定的 skill 验证入口只覆盖 built-in runtime skills，对 repo 其他 internal skills 的 metadata completeness、mirror drift、upstream provenance 和 authoring expectations 都没有统一约束。

同时，外部平台约束已经在变化：GitHub Copilot 使用 `.github/copilot-instructions.md` 这一套 repo 指令面，Claude Code 也已经强调 slash commands / subagents 等显式项目资产。AgentForge 既然要长期同时维护 Codex / Claude / GitHub 等多种消费面，就更需要一个 repo-owned 的 canonical governance layer，而不是继续靠目录习惯和人工同步维持。

## Goals / Non-Goals

**Goals:**

- 给仓库内部维护的 skills 建立一份显式 registry，标明每个 skill family 的 canonical root、profile、provenance 与 mirror target。
- 用 profile-based 方式统一 skill 标准，而不是假设 `skills/*`、`.agents/skills/*`、`.codex/.claude/.github` 必须长得完全一样。
- 提供 shared verification + optional sync 入口，显式阻断缺失 frontmatter、坏掉的 agent yaml、mirror drift、lockfile/provenance 漂移等问题。
- 产出一份 maintainer-facing authoring guide，说明新增、更新、同步、验证 internal skills 的规范动作。

**Non-Goals:**

- 不改写当前 role runtime、marketplace、Go skill parser 只扫描 `skills/*` 的产品边界；`.agents/.codex/.claude/.github` 仍然是 repo tooling/workflow assets，而不是新的产品 skill source。
- 不在本次里把所有外部消费面迁移到新的目录约定，例如强制改成 `.claude/agents/*` 或发明新的 runtime loader。
- 不把所有 upstream-imported skills 强制改写成 repo-authored 形态；对来自 `skills-lock.json` 的同步资产允许保留 profile-aware 例外。

## Decisions

### 1. 引入 repo-level `internal-skills` registry，显式声明 canonical source 与 skill profile

本次设计将新增一份 repo-owned internal skill registry（路径可为仓库根目录独立清单，而不是继续隐含在某个 skill root 里）。每条记录至少声明：

- `id`
- `family`（如 `built-in-runtime`、`repo-assistant`、`workflow-mirror`）
- `canonicalRoot`
- `verificationProfile`
- `sourceType`（`repo-authored`、`upstream-sync`、`generated-mirror`）
- `mirrorTargets` / `lockKey` / `docsRef` 等可选元数据

这样做的原因：

- 当前 repo 里确实存在多类 skill consumer，单靠目录名无法表达“谁是 source of truth，谁只是 mirror”。
- `skills-lock.json` 已经说明部分 `.agents/skills` 来自 upstream source，但仓库没有地方把“这个 lock entry 对应哪个 skill package、允许哪些结构例外”写清楚。
- registry 能让验证、文档和后续同步脚本依赖同一份 authoritative inventory，而不是在多个脚本里重复硬编码路径。

备选方案：

- 继续靠目录命名和约定推断。拒绝原因：这正是当前漂移来源。
- 为每个 root 单独维护不同清单。拒绝原因：维护者仍然要跨文件推断 canonical ownership，无法真正降低复杂度。

### 2. 使用 profile-based skill contract，而不是强行统一成一套完全相同的包模板

本次不会要求所有 internal skills 完全同构，而是定义按 family/profile 生效的最小规范：

- `built-in-runtime`: 面向 `skills/*`，要求 marketplace / role/runtime 需要的完整元数据、可解析 `SKILL.md`、可用 agent interface 配置，以及与 `skills/builtin-bundle.yaml` 一致的 canonical package root。
- `repo-assistant`: 面向 `.agents/skills/*`，要求基础 frontmatter、清晰正文结构，以及在存在 agent config/references/scripts/tests 时满足约定布局；允许“纯知识包”与“带脚本/测试的技能包”两种受控子型。
- `workflow-mirror`: 面向 OpenSpec workflow skills，要求一个 canonical package 作为 source，其余消费面副本只能是显式声明的 mirror，不再作为独立 authoring truth。
- `upstream-sync`: 不是单独的消费 root，而是 `sourceType` 例外；允许像 `shadcn` 这类技能保留 upstream 带来的 `.yml` 或特殊布局，但必须在 registry + lockfile 中有明确 provenance。

这样做的原因：

- 真正需要标准化的是“规则和例外都显式”，不是把本来服务不同 consumer 的 skill 资产硬捏成同一模板。
- 如果把 upstream-imported skills 当 repo-authored 技能强行重写，会让后续同步成本过高，也会掩盖真实来源。

备选方案：

- 所有 skill 一律要求 `SKILL.md + agents/openai.yaml + references/`。拒绝原因：会把现有合法 skill family 误判为不合规。
- 完全放弃例外机制。拒绝原因：`skills-lock.json` 已经证明仓库存在受控 upstream sync 的现实需求。

### 3. 以 `.codex/skills/*` 作为当前 OpenSpec workflow skill 的 canonical source，`.claude/.github` 作为镜像目标

对于 `openspec-*` 这一类 workflow skills，本次推荐把 `.codex/skills/*` 定义为仓库当前 canonical source，并将 `.claude/skills/*`、`.github/skills/*` 视为 mirror targets，通过脚本或 shared generator 同步。

这样做的原因：

- 当前实际 Codex 会直接消费 `.codex/skills/*`，仓库里的 OpenSpec skill authoring 也已经围绕这套文件在运行。
- 直接新增一个中立新根目录会引入迁移和消费方调整，收益不如先把“哪份是 source、哪份是 mirror”说清楚。
- 一旦 registry 明确了 source/mirror 关系，后续即使要迁到中立根目录，也有稳定迁移起点。

备选方案：

- 新增完全中立的 `internal-skills/` 根目录再生成三方副本。暂不采用：会扩大这次 change 的迁移面。
- 允许 `.codex/.claude/.github` 三方都手工维护。拒绝原因：这正是当前 drift 难以证明的根源。

### 4. 增加 shared verification command，并让 built-in verification 成为其子集而不是唯一 skill gate

本次将新增 repo-level verification 入口（例如 `pnpm skill:verify:internal`），按 registry/profile 扫描 internal skills，并校验：

- registry 中的 canonical roots 是否存在且可读；
- frontmatter 与 agent config 是否满足对应 profile；
- `sourceType=upstream-sync` 的 skill 是否有对应 `skills-lock.json` provenance；
- `sourceType=generated-mirror` / `mirrorTargets` 是否与 canonical source 保持同步；
- 当前 `pnpm skill:verify:builtins` 是否仍能基于 shared validator 覆盖 `skills/*` 的 built-in 子集。

这样做的原因：

- 现在的 `pnpm skill:verify:builtins` 只覆盖 marketplace 前置条件，无法说明 repo 其他 skill assets 是否仍规范。
- 用 shared validator 可以避免 `skills/*`、`.agents/*`、workflow mirrors 各写一套不一致的检测逻辑。

备选方案：

- 继续只保留 built-in validator。拒绝原因：无法覆盖用户这次真正想要的“项目内部 skills 标准化”。
- 把验证完全塞进 CI 脚本，不提供本地命令。拒绝原因：维护者缺少快速反馈回路。

### 5. 产出 maintainer-facing internal skill authoring guide，明确新增/更新/同步流程

仓库需要一份专门的 authoring guide，至少回答：

- 新 skill 先选哪个 family/profile；
- 何时必须写 `requires` / `tools` / agent interface；
- 何时需要 references/scripts/tests；
- upstream-sync skill 如何更新 lockfile 与验证 hash；
- workflow mirror 如何同步和验证；
- 哪些目录是产品 skill source，哪些只是 repo workflow assets。

这样做的原因：

- 仅靠脚本报错不足以回答“为什么这里要这样组织”。
- 当前 README、role docs 和各 skill 本身都散落着局部真相，但没有 maintainer 视角的一页式规范。

## Risks / Trade-offs

- [profile 设计过细，反而增加维护复杂度] → Mitigation: 控制在少量 family/profile，优先覆盖当前真实 skill 类型，避免抽象过度。
- [upstream-imported skills 与 repo-authored 技能标准冲突] → Mitigation: 用 `sourceType=upstream-sync` + lockfile provenance 做显式例外，而不是静默放任或强制改写。
- [mirror sync 脚本会把人为定制内容覆盖掉] → Mitigation: 只对 registry 标记为 generated-mirror 的目标执行同步，并在文档中声明这些路径不再手工编辑。
- [shared validator 与现有 built-in validator 逻辑重复] → Mitigation: 让 `skill:verify:builtins` 复用 shared validator 的 built-in profile，而不是长期双写规则。
- [标准化扩大到产品 runtime 行为] → Mitigation: 明确把产品 loader 边界留在 `skills/*`，其他目录只做 repo tooling / workflow governance。

## Migration Plan

1. 新增 internal skill registry，并先录入当前已知 family：`skills/*` built-ins、`.agents/skills/*`、`.codex/.claude/.github` OpenSpec workflow mirrors。
2. 定义 profile-aware validator，先覆盖读取、frontmatter、agent yaml、mirror drift 与 lockfile provenance。
3. 把 `pnpm skill:verify:builtins` 接到 shared validator 的 built-in 子集，新增 repo-level internal skill verification command。
4. 补 internal skill authoring guide，并把当前技能资产回填到对应 profile 规范。
5. 如需 mirror generation/sync，再将 OpenSpec workflow skills 改为 canonical-source + generated-target 的维护模式。

回滚策略：

- 若 shared validator 初期误报过多，可先以 warning/report 模式落地 registry 和 docs，再逐步把 profile 规则提升为 hard fail。
- 若 mirror auto-sync 引入过高摩擦，可先保留 drift detection，再延后自动写回。

## Open Questions

- `.claude/commands/opsx/*.md` 是否也应并入同一 canonical workflow source，还是先只治理 `skills/*` 镜像？
- 对来自 upstream 的 skill（如 `shadcn`）是否要在第一阶段就统一 `.yml -> .yaml`，还是把文件名差异保留为受控例外？
