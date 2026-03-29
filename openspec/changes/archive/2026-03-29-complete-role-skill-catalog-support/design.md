## Context

AgentForge 当前的角色 skills 链路已经完成了第一阶段闭环：Go 和前端都能 round-trip `capabilities.skills`，角色工作台也能编辑 `{ path, auto_load }` 行，角色卡片和摘要能显示技能数量与 auto-load/on-demand 拆分。这说明现在的缺口不再是 schema 或基础 CRUD，而是 authoring 完整性。

当前 repo 真相与文档设计之间仍然有三处明显脱节：

- `docs/PRD.md`、`docs/part/PLUGIN_SYSTEM_DESIGN.md` 和 `docs/part/PLUGIN_RESEARCH_ROLES.md` 把 Skill-Tree 描述成真实知识层，示例 role 也直接引用 `skills/react`、`skills/testing`、`skills/css-animation` 等 path，但当前 checkout 中并没有对应的 repo-local skill tree 或 catalog。
- `components/roles/role-workspace-editor.tsx` 与相关状态层只提供自由输入 path 的 skill rows。作者可以保存 skill path，但无法在工作台里知道某个 skill 是否真实存在、来自哪个 source、是否只是模板/继承带来的保留引用。
- 之前的 `complete-role-skills-management` 变更明确把“全局 skill 目录服务或 marketplace catalog”排除在外，这是合理的第一阶段边界；但现在如果继续停在纯文本 path authoring，角色 skills 仍然不像文档中定义的“一等知识层”，而更像未经解释的字符串配置。

因此，这次设计要解决的是“authoritative catalog + source-aware authoring + preview/review feedback”问题，同时保持现有 role manifest、preview/sandbox contract 和 runtime projection 边界不失真。

## Goals / Non-Goals

**Goals:**

- 为角色 authoring 建立一个 repo-truthful 的 skill catalog，至少覆盖文档和内置 role 示例引用的 canonical repo-local skills。
- 让角色工作台支持“从 catalog 发现并选择 skill”与“继续手动输入 path”两种路径，并把各自的解析状态解释清楚。
- 让 review、preview 或 sandbox 结果对 role skills 提供来源、解析状态和 unresolved 提示，避免作者只能靠猜测理解当前 skill tree。
- 让 built-in/sample roles 与 sample skill fixtures 保持一致，消除“role.yaml 引用了并不存在的 skills”这种长期漂移。
- 保持现有结构化 skill rows、继承语义、顺序保留和 non-blocking 手动引用能力，不把这次 change 扩大成 runtime auto-load 或 skill marketplace。

**Non-Goals:**

- 不把 role skills 直接注入当前 Bridge execution profile，也不为 auto-load skills 新增 runtime 自动加载逻辑。
- 不扫描用户机器上的全局 Codex skill 安装目录，也不把 repo authoring 绑定到当前操作者本机环境。
- 不把成员页 `skills` 标签系统与 role `capabilities.skills` 合并成统一能力图谱。
- 不引入完整的 marketplace/registry 产品面；这次只解决 repo-local authoring completeness 与 sample truth。

## Decisions

### 1. 以 repo-local `skills/` 作为 role authoring 的 canonical catalog root，并允许手动 path 作为软回退

角色 authoring catalog 只对 repo 内明确约定的 skill roots 负责，首选 canonical root 为 `skills/`。catalog 通过扫描 `skills/**/SKILL.md` 生成可选 skill inventory，path 以 role.yaml 当前使用的相对形式暴露，例如 `skills/react`。

原因：

- 文档和示例 role 已经把 `skills/<name>` 当成 canonical path 形式，补齐 repo-local `skills/` 能最直接地消除文档与仓库事实漂移。
- 只扫描 repo-local roots 能避免把 dashboard authoring 绑定到某个操作者机器上的全局 skill 安装状态，保持 Web/CI/多人协作的一致性。
- 手动 path 仍然要保留，因为 role 可能暂时引用未来才会同步进仓库的 skills，或引用其他后续 source。

备选方案：

- 扫描 `.codex/skills`、用户 home 或远端 registry。缺点是环境耦合重、结果不稳定，也会让前端 authoring 结果依赖当前运行节点。
- 完全不做 catalog，只继续自由输入 path。缺点是这正是当前缺口，无法称为“完整 support”。

### 2. Skill catalog 使用“可发现 + 软解析”模型，而不是保存前硬性存在校验

catalog 会为每条 role skill reference 计算 authoring-level resolution state，例如 `resolved`, `unresolved-manual`, `inherited-unresolved`, `template-derived` 等，并尽量附带 title/description/source metadata。保存时仍然只对空 path、重复 path 和明显格式问题做 hard block；skill 是否已在 catalog 中解析只作为 warning/context，不作为 hard blocker。

原因：

- 这与前一阶段 change 的边界一致：角色 skills 仍是声明式知识引用，不应因为当前仓库尚未同步某个 skill 文件就完全阻断 authoring。
- warning 比 hard block 更适合当前阶段，也更能兼容模板复制、父角色继承和分步迁移。
- preview/review 中展示 unresolved 状态已经能显著降低误配，而不需要把 authoring 流变得过于脆弱。

备选方案：

- 保存前强制 catalog 命中。缺点是会把 repo 演进、跨分支协作和草稿阶段的配置全部卡死。
- 完全不区分 resolved 与 unresolved。缺点是继续让 skill path 只是无解释字符串。

### 3. 角色工作台采用 hybrid skill rows：每行既能选 catalog skill，也能切回手动 path

当前 skill rows 结构继续保留，但每一行不再只是裸输入框。作者应能搜索 catalog skill 并回填 path，也能直接输入一个手动 path。行内要显示解析状态、来源和 auto-load 说明；当 path 不在 catalog 中时，该行保持可保存，但必须明确显示它是 unresolved manual reference。

原因：

- 这样不会推翻已经落地的 role draft/state 结构，只是在同一 model 上增强 authoring 能力。
- 兼顾 discoverability 和 flexibility，符合“完整 support”而不引入过度约束。
- 与响应式 role workspace 现状兼容，适合在现有 Capabilities section 内逐步增强。

备选方案：

- 改成单独的 catalog picker，不允许手动 path。缺点是破坏现有灵活性，并对未来扩展 source 不友好。
- 保持旧输入框，另加旁路 catalog 页面。缺点是作者仍要在多个上下文之间来回跳转。

### 4. Preview / sandbox 不负责解析 skill 内容本身，但要返回 skill-resolution context

当前 preview / sandbox 已是 authoritative authoring helper。本次不新增“读取 skill 内容并拼接 prompt”的运行时语义，而是在 preview/result 层返回或派生足够的 skill-resolution context，帮助前端解释：

- 当前 role skills 哪些是 catalog-resolved；
- 哪些来源于模板或继承；
- 哪些仅作为 canonical YAML 引用保留但尚未在 repo-local catalog 中解析；
- unresolved skills 不属于当前 execution profile hard blocker，而是 authoring warning。

原因：

- 这延续了 `role-authoring-sandbox` 现有职责：解释 authoring 结果，而不是改 runtime contract。
- preview/review 已经承载 provenance 与 save-impact 反馈，把 skill resolution 放在同一处最符合当前架构。

备选方案：

- 把 skill resolution 只放在前端本地推断。缺点是容易与后端 skill root 规则漂移。
- 把 unresolved skill 当 readiness blocker。缺点是会误导作者，以为当前 runtime 已经消费 role skills。

### 5. 用 canonical sample skills 修复 built-in role 与文档示例漂移

仓库需要至少补上与内置/示例 role 一致的一组 sample skills，例如 `skills/react`、`skills/typescript`、`skills/css-animation`、`skills/testing`，并让 catalog/tests 基于这些 fixtures 工作。sample skill package 以最小可读 `SKILL.md` 为主，强调这是 repo-owned authoring fixture，而非完整 marketplace 资产。

原因：

- 没有 sample skills，catalog 只能是空的，role.yaml 仍继续引用不存在路径，proposal 目标无法成立。
- 这能让 docs、fixtures、catalog、tests 共享一套最小真相。

备选方案：

- 只做 catalog 代码，不补 sample skills。缺点是产品面仍然演示不出真实路径。
- 把所有 docs 示例改成“假路径”。缺点是降低产品信度，不解决 authoring 可发现性。

## Risks / Trade-offs

- [Role skills 被误解为当前 runtime 已自动加载] → 在工作台、preview 和文档里明确区分“catalog resolved authoring reference”与“current execution profile projection”。
- [repo-local sample skills 被误当成正式 marketplace 资产] → 使用最小 fixture 结构并在 docs 中标明它们是 built-in/sample authoring assets。
- [catalog 与手动 path 两套入口让 UI 复杂度上升] → 复用现有 skill rows，只在单行内增强搜索/选择/状态展示，不再引入第二个独立工作流。
- [未来 skill roots 扩展导致 catalog contract 再变化] → 在返回结构中预留 `source`/`root`/`resolutionState` 这类稳定元数据，避免只编码单一路径假设。

## Migration Plan

1. 引入 repo-local `skills/` fixtures 与 catalog discovery helper，先让 built-in/sample roles 的引用有可解析目标。
2. 在 role authoring 相关 API 或 helper 上暴露 catalog/read-only metadata，并补充 preview/review 需要的 skill-resolution context。
3. 升级 role workspace 的 skills section 与 summary/review surfaces，接入 catalog 选择、手动回退和 unresolved 提示。
4. 更新 docs 与 focused tests，验证 sample roles、sample skills、catalog 和 authoring UI 一致。

回滚策略：

- 若 catalog authoring UI 引入明显可用性问题，可先保留 repo-local sample skills 和后端 catalog/helper，但把前端交互降级回只读 resolution cues + 旧输入框，不影响现有 role skill payload contract。

## Open Questions

- skill catalog 的只读接口应当直接挂在现有 roles authoring API 上，还是提供单独的 `GET /api/v1/roles/skills` 风格发现面；本 change 优先选最小、最不漂移的现有后端 seam。
- sample skill metadata 需要从 `SKILL.md` frontmatter 读取到什么深度；本 change 至少需要 path、title/name、简短描述，更多字段可后续再扩。
