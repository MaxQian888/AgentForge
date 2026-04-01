## Why

AgentForge 现在已经把 repo-local `skills/**/SKILL.md` 变成角色 authoring 和 runtime projection 的真实能力面，但这些内置 skills 仍然没有像 built-in plugins 那样的官方 bundle 真相源，也没有被完整接进 `/marketplace` 的产品面。当前 marketplace 对 skill 条目仍停留在通用 description/tags/license 展示，无法把 `SKILL.md` 的 Markdown 主体、frontmatter YAML、`agents/*.yaml` 接口配置和 repo-local provenance 作为一等详情展示出来，导致“仓库里真实存在的内置技能”和“市场里可浏览、可理解、可消费的技能”依然脱节。

现在需要补这条线，是因为仓库已经有一套成熟的 skill package 解析能力、角色 skill catalog、以及 skill marketplace install handoff。如果继续让内置 skills 只存在于 `skills/` 目录和 role authoring 选择器里，市场会继续把 skill 当成最弱的 item type: 可以发布和安装，但不能像内置 plugin 一样被 operator 真正看懂、比较、预览和跳转使用。

## What Changes

- 为当前 repo-owned built-in skills 定义一条官方真相源，明确哪些 `skills/*` 是当前 checkout 随仓库维护并面向市场展示的内置技能，以及它们的市场分类、标签、展示优先级和 provenance。
- 新增 skill package preview 合同，要求后端基于现有 `SKILL.md` / `agents/*.yaml` 解析能力返回结构化 skill detail，使前端能展示 Markdown 内容、frontmatter YAML、agent interface YAML、依赖、工具需求和可用部件，而不是继续把 skill detail 压扁成 description 文本。
- 扩展 `/marketplace` 工作区，使其能够区分 repo-owned built-in skills 与 standalone marketplace skill items，并为 skill 类型提供真正可用的 detail surface、truthful action state 和 downstream handoff，而不是只有通用卡片和安装按钮。
- 扩展 marketplace consumption 语义，让 repo-owned built-in skills 也能以“已在 role skill catalog 可用”的本地能力状态被 truthfully 呈现，不再要求所有 skill 都经过一次 marketplace install 才能在市场里被识别为可消费资产。
- 明确前端 skill detail 渲染必须优先复用成熟库实现 Markdown 和 YAML 展示能力，而不是继续新增手写字符串拼装或 ad-hoc 渲染逻辑。

## Capabilities

### New Capabilities
- `built-in-skill-bundle`: define the official repository-owned built-in skill inventory, market-facing metadata, provenance, and canonical layout rules for maintained repo-local skills.
- `skill-package-preview`: define the structured preview contract for skill packages, including parsed `SKILL.md` markdown content, frontmatter YAML, agent interface YAML, dependency metadata, and source-part inventory.

### Modified Capabilities
- `marketplace-operator-workspace`: change workspace requirements so `/marketplace` can surface repo-owned built-in skills, distinguish built-in versus standalone marketplace provenance, and render rich skill detail with markdown and yaml support.
- `marketplace-item-consumption`: change consumption requirements so repo-owned built-in skills can appear as already discoverable local assets with truthful consumer-surface state and handoff actions, not only as install-derived marketplace records.

## Impact

- Affected repo-owned skill assets: `skills/*/SKILL.md`, `skills/*/agents/*.yaml`, related references/assets, and the new built-in skill bundle metadata source that defines which repo-local skills are official built-ins.
- Affected backend seams: `src-go/internal/role/*` skill parsing helpers, new or adjacent built-in skill bundle/detail handlers, `src-go/internal/handler/marketplace_handler.go`, marketplace DTO shaping, and any local discovery or consumption helpers reused by `/marketplace`.
- Affected frontend seams: `app/(dashboard)/marketplace/page.tsx`, `components/marketplace/*`, `lib/stores/marketplace-store.ts`, and shared content-rendering components needed to show skill markdown/yaml detail truthfully.
- Affected dependencies and verification: root `package.json` frontend rendering dependencies for Markdown/YAML support, focused frontend/backend tests for built-in skill listing/detail/consumption behavior, and targeted checks that keep bundle metadata aligned with the real `skills/` packages.
