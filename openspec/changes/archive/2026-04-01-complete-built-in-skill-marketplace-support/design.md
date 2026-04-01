## Context

AgentForge 当前已经有几条与 skill 直接相关、但尚未在 marketplace 产品面汇合的成熟 seam：

- `src-go/internal/role/skill_document.go`、`skill_package.go`、`skill_catalog.go` 已能解析 `skills/**/SKILL.md` frontmatter、Markdown body、`agents/openai.yaml` interface 配置，以及 references/scripts/assets 等部件计数。
- `GET /api/v1/roles/skills` 已把 repo-local skills 暴露为权威 role skill catalog，但它面向 authoring，返回的是“可引用 skill 列表”，不是“可直接在 marketplace 里浏览的 skill 产品详情”。
- `src-go/internal/handler/marketplace_handler.go` 已能把 marketplace skill artifact 安装到 `skills/<slug>/SKILL.md` 并验证其可被 role skill catalog 发现，但 `/api/v1/marketplace/consumption` 目前只围绕 install 派生记录，内置 repo-local skills 不在这个真相面里。
- `app/(dashboard)/marketplace/page.tsx` 和 `components/marketplace/marketplace-item-detail.tsx` 目前把 skill 当作普通 item type 渲染，只展示 description/tags/license/latest version；没有 `SKILL.md` Markdown、frontmatter YAML、agent YAML、repo-local provenance 或 built-in action state。
- 根前端依赖当前没有专门的 Markdown/YAML 渲染库；仓库里现有的 YAML 展示主要是 `lib/roles/role-management.ts` 里的手写 `renderRoleManifestYaml(...)`，不适合作为 marketplace skill detail 的长期路线。
- 仓库真实内置 skills 当前集中在 `skills/react`、`skills/typescript`、`skills/testing`、`skills/css-animation`。这些包已经具备 `SKILL.md` 与 `agents/openai.yaml`，但还没有像 `plugins/builtin-bundle.yaml` 那样的官方 bundle 真相源来定义“哪些 skill 是当前 checkout 的官方 built-ins，以及它们该如何在市场里展示”。

因此，这次 change 的本质不是再造一个新 skill 系统，而是把现有 repo-local skill package / role catalog / marketplace 三条线收敛到一个 repo-truthful 的 operator product surface 上。

## Goals / Non-Goals

**Goals:**

- 为当前 checkout 的 repo-owned built-in skills 建立一个官方 bundle 真相源，避免 marketplace 前端或 docs 继续靠 `skills/` 目录扫描结果做隐式推断。
- 为 skill 定义统一的 package preview contract，使 built-in skills 与 standalone marketplace skill items 都能展示结构化 Markdown/YAML 详情。
- 让 `/marketplace` 在 skill 维度上成为真实产品面：内置 skills 可见、远端 skills 可预览、provenance 可区分、downstream handoff 明确。
- 扩展 consumption truth，使 repo-owned built-in skills 能被识别为“已在 role skill catalog 可用”的本地能力，而不再被误判为“未安装”。
- 明确前端优先使用成熟 Markdown/YAML 库渲染 skill detail，而不是延续手写字符串渲染。

**Non-Goals:**

- 不重做整个 standalone marketplace 的 publish/install lifecycle；远端 marketplace item CRUD、review、feature、version upload 继续沿用现有 `src-marketplace` 能力。
- 不把 built-in skills 同步进 `src-marketplace` 数据库作为种子项，也不引入新的 SaaS registry/remote sync 流程。
- 不把 role authoring catalog 改写成市场工作区；`GET /api/v1/roles/skills` 继续以 authoring 需求为主，不直接承担 marketplace 全量 detail 负载。
- 不在本次中把 role workspace 现有手写 YAML preview 全面迁移到新库；本次只要求 skill market detail 采用 library-backed Markdown/YAML 渲染。

## Decisions

### 1. 复用 built-in plugin bundle 模式，为内置 skills 新增 `skills/builtin-bundle.yaml`

本次将引入 repo-owned `skills/builtin-bundle.yaml`，作为官方 built-in skills 的唯一真相源。它不重复存储 `SKILL.md` 已经能解析出的基础文案，而只声明市场展示与 provenance 必需、但无法可靠从 skill package 推导出的元数据，例如：

- official built-in skill id / slug
- package root（相对 `skills/`）
- category / tags / featured priority
- docs reference / repository reference
- optional marketplace badge or ordering metadata

bundle loader 会像 built-in plugin bundle 一样校验：

- bundle 中声明的 skill package root 必须存在且包含 `SKILL.md`
- package 必须能被当前 `DiscoverSkillCatalog(...)` 解析为 canonical `skills/<id>` path
- bundle 元数据与实际 skill package frontmatter 不得冲突到影响 marketplace 呈现

选择 bundle 而不是直接扫描全部 `skills/**` 的原因：

- 当前 `skills/` 目录未来可能同时容纳官方 built-ins、实验技能、测试 fixture 或 staged package，仅靠扫描无法表达“官方市场面”。
- built-in plugin 已经采用 bundle 作为 repo-owned 真相源；skill 复用同类 seam，后续 operator 体验和 drift verification 更一致。
- 让 docs、tests 和 marketplace store 都依赖同一份显式清单，比在前端写白名单或靠目录命名约定更稳。

备选方案：

- 直接把所有 `skills/**` 当成 built-in skills。拒绝原因：会把实验/fixture 资产误暴露到 marketplace。
- 把内置 skills 当作 `src-marketplace` seed data。拒绝原因：远端服务不是 repo-local asset 的权威来源，容易与实际 checkout 漂移。

### 2. 引入统一的 `SkillPackagePreview` DTO，并按 provenance 分别在 `src-go` 与 `src-marketplace` 生产

为保证 built-in skills 与远端 skill items 在前端能走同一套 detail 渲染逻辑，本次定义统一 `SkillPackagePreview` 结构。推荐字段至少包括：

- `canonicalPath`
- `sourceType` (`builtin` / `marketplace`)
- `displayName`, `description`, `defaultPrompt`
- `markdownBody`
- `frontmatterYaml`
- `agentConfigs[]`：每个 agent yaml 的 path、normalized yaml text、以及可直接展示的 interface 摘要
- `requires`, `tools`
- `availableParts`, `referenceCount`, `scriptCount`, `assetCount`

生产方式分两路：

- repo-owned built-in skills：由 `src-go` 直接复用现有 `skill_package.go` / `skill_document.go` 解析本地文件，并通过新的 built-in skill market endpoint 返回 detail + preview。
- standalone marketplace skill items：由 `src-marketplace` 在 skill version 上传成功后解析 artifact 中的 `SKILL.md` 与 `agents/*.yaml`，持久化或缓存 preview 结果，并在 item detail endpoint 中返回，避免前端为预览去下载或解包 artifact。

这样做的原因：

- built-in skills 与远端 skill items 都是 skill package，只是 provenance 不同；前端不应维护两套 detail 渲染模型。
- `src-go` 已经有成熟 parser，built-in preview 不需要重新解析。
- `src-marketplace` 已经控制 skill artifact upload/validation，是生成远端 skill preview 的最合适位置。

备选方案：

- 前端自行解析 `SKILL.md` / YAML。拒绝原因：Web 客户端既拿不到 built-in 本地文件，也不适合解析 zip artifact。
- 只让 built-in skills 有 rich preview，远端 skill 继续显示 description。拒绝原因：skill item type 仍会存在“本地强、远端弱”的产品断层，不能称为完整适配。

### 3. `/marketplace` 以“built-in section + remote list”合并，而不是把 built-ins 强塞进远端分页结果

前端 marketplace store 将继续把 `src-marketplace` 作为远端 marketplace items 的来源，同时新增 `src-go` built-in skill feed。UI 上采用两段式合并：

- 当过滤器允许 skill 时，页面先渲染一个 repo-owned `Built-in skills` section，显示来自 `skills/builtin-bundle.yaml` 的官方 skills。
- 远端 marketplace list 继续来自 `src-marketplace`，保留现有搜索、排序、分页契约，不把 built-ins 塞进远端总数里。
- 选中任意 skill item 后，detail pane 根据 provenance 读取统一 preview shape 并渲染。

选择 dedicated section 的原因：

- built-in skills 数量小、来源稳定、与远端 marketplace 的分页模型不同，强塞进远端分页会让 total/page/filter 语义变脏。
- 即便远端 marketplace service 不可用，repo-owned built-in skills 仍然可见；页面可以表达“远端市场不可用，但本地官方技能仍可浏览”的 partial degradation。
- 这与 built-in plugins 在插件控制面的 operator 认知一致：官方内置资产应被清楚区分，而不是伪装成普通远端条目。

备选方案：

- 通过 Go aggregator 彻底代理远端 marketplace list。暂不采用：能做，但会扩大 `/marketplace` 重构面；当前 built-in skill 数量小，用 section 合并足够真实。
- 完全不在 marketplace 中显示 built-ins。拒绝原因：与用户要求和 repo-owned skill product surface 目标相悖。

### 4. 扩展 consumption truth：built-in skills 以本地 discoverable 资产进入 `/api/v1/marketplace/consumption`

`/api/v1/marketplace/consumption` 不再只返回 install-derived 记录，还要合成 repo-owned built-in skill records。推荐语义：

- `itemType = "skill"`
- `consumerSurface = "role-skill-catalog"`
- `installed = true`
- `provenance.sourceType = "builtin"`
- `localPath = skills/<id>`
- `status = "installed"`，但 detail CTA 不显示 install，而是显示 `Open role authoring` 或其他 downstream handoff

这要求 `src-go` 用 built-in skill bundle + local discovery seam 生成 record，并在前端根据 `sourceType=builtin` 和 item source 做行为分流。

这样做的原因：

- built-in skills 已经是 repo-local 可消费资产，若 consumption 面继续看不到它们，marketplace detail 就只能错误显示“未安装”。
- 复用现有 typed consumption contract，比再造一个“built-in availability”平行 DTO 更稳。

备选方案：

- 为 built-in skills 单独新增 availability endpoint，前端自己拼状态。拒绝原因：会让同一个 `/marketplace` item detail 同时依赖两套状态模型。
- 把 built-in skills 标成 `blocked` 且不允许 action。拒绝原因：与真实 repo 状态相反。

### 5. Skill detail 渲染必须使用成熟库：`react-markdown` + `remark-gfm` + `yaml`

前端新增 shared skill content renderer，明确采用成熟库而不是手写渲染：

- Markdown：使用 `react-markdown` 渲染 `markdownBody`，并接入 `remark-gfm` 支持列表、表格、任务列表等常见 skill 文档语法。
- YAML：使用 `yaml` 包解析/规范化 frontmatter 与 agent config 文本，再在只读 code block 中展示，避免继续复制 `renderRoleManifestYaml(...)` 这种手写序列化逻辑。
- 安全边界：不启用 raw HTML 渲染；保留 Markdown 默认转义，避免 detail pane 变成任意 HTML 执行入口。

选择这些库的原因：

- 它们与当前 Next.js 16 + React 19 前端栈兼容，集成成本低，语义清晰。
- 用户已经明确要求“前端应该有 markdown 和 yaml 的渲染支持，注意积极用库”，这与继续手写 serializer 相冲突。
- YAML 在这个场景主要是阅读与比对，不需要引入更重的编辑器栈。

备选方案：

- 继续用手写字符串拼接渲染 YAML/Markdown。拒绝原因：当前 marketplace skill detail 正是因此太弱，且易与 skill package 真相漂移。
- 为 skill detail 引入完整编辑器。拒绝原因：本次是预览/展示，不是 authoring/编辑。

### 6. 为 built-in skills 增加 bundle drift verification，而不是只依赖人工维护

本次需要补一个 focused verification seam，类似 `plugin:verify:builtins`，用于校验：

- bundle 中的每个 built-in skill 都存在并可被 catalog 解析
- preview contract 可成功生成 Markdown/YAML detail
- 当前四个 repo-owned built-ins 至少满足最低 market-facing completeness

实现上可以是新的 Node/TS 脚本、Go test，或两者结合，但 contract 上必须有稳定的自动化验证入口，避免后续某个 built-in skill 缺少 `agents/openai.yaml` 或 frontmatter 时悄悄退回“只有 description”的弱展示。

## Risks / Trade-offs

- [远端 skill preview 与本地 built-in preview 可能因解析逻辑分叉而漂移] → Mitigation: 固定统一 DTO，并为 `src-go` / `src-marketplace` 两侧补同构 preview fixture tests。
- [built-in skill section 与 remote marketplace list 并存，会引入 partial-unavailable 状态复杂度] → Mitigation: 在 store 中显式区分 `remoteServiceStatus` 与 `builtInStatus`，避免 UI 靠空数组推断。
- [新增 Markdown/YAML 渲染库会带来前端依赖和样式维护成本] → Mitigation: 只在 skill detail 用 shared renderer，保持无 raw HTML、无重编辑器依赖，避免大范围扩散。
- [built-in bundle 元数据可能与 skill package frontmatter 重复，增加维护成本] → Mitigation: bundle 只存 market-facing metadata 和 provenance，包内已有字段继续以 `SKILL.md`/agent YAML 为准。
- [若远端 marketplace item 没有 skill artifact 或 preview 解析失败，detail 会退化] → Mitigation: 把 preview parse failure 作为稳定 detail state 显式返回，而不是静默回退为空 description。

## Migration Plan

1. 新增 `skills/builtin-bundle.yaml` 与 bundle loader，先把当前四个 repo-owned built-in skills 纳入官方清单。
2. 定义统一 `SkillPackagePreview` DTO，并在 `src-go` 为 built-in skills 增加 list/detail endpoint 与 consumption synthesis。
3. 扩展 `src-marketplace` skill version upload/detail 流，使 skill items 能返回 preview 数据。
4. 更新 `/marketplace` store 和 detail UI，引入 Markdown/YAML 渲染库并接入 built-in skill section。
5. 补 built-in bundle drift verification、preview contract tests 和前端详情渲染 tests。

回滚策略：

- 如果远端 skill preview 解析链路不稳定，可以先保留 built-in skill preview 与 built-in section，同时让远端 skill detail 显式显示 `preview unavailable`，而不是阻塞 built-in surface 落地。
- 如果前端库接入引发样式或静态导出问题，可临时保留 DTO 与 endpoint，只将 YAML/Markdown 渲染降级为 library-backed plain text code blocks，而不回退到手写 serializer。

## Open Questions

- agent config preview 是否第一阶段只覆盖 `agents/openai.yaml`，还是允许展示所有 `agents/*.yaml|yml` 文件；本次推荐至少支持当前仓库已经使用的 `openai.yaml`，并为多文件扩展保留数组结构。
- built-in skill bundle 是否需要记录固定版本号，还是以当前 checkout 的 repo 状态为唯一版本真相；本次更倾向于后者，避免给 repo-local built-ins 引入虚假的 registry version 语义。
