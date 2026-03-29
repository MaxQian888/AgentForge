## Why

`docs/PRD.md`、`docs/part/PLUGIN_SYSTEM_DESIGN.md` 和 `docs/role-authoring-guide.md` 都把 `capabilities.skills` 定义成角色能力模型里的显式知识层，用来表达可自动加载或按需加载的专业技能树。当前仓库虽然已经支持结构化 `{ path, auto_load }` 技能引用，但角色工作台仍然只让操作者手填 path，仓库里也缺少与示例 role 对齐的 skill catalog、解析来源和可见的 unresolved 状态，导致这条链路仍停留在“能保存字符串”而不是“功能完整的 skills support”。

现在需要把角色对 skills 的支持补到文档描述的产品真相：操作者应当能在同一条 authoring 流里发现可用 skills、理解它们来自哪里、知道当前配置是否已解析，并在保持手动 path 灵活性的同时避免继续盲填不存在或难以解释的 skill 引用。

## What Changes

- 新增一个面向角色 authoring 的 skill catalog 能力，从仓库支持的 skill roots 中发现可用 skills，并返回规范化的 path、来源、标题/简介与解析状态，避免前端硬编码 skill 列表。
- 为角色工作台补上基于 catalog 的 skills authoring 体验，包括搜索/选择已有 skill、继续保留手动 path 输入、显示 auto-load 与 on-demand 语义说明，以及对 unresolved 或缺少元数据的 skill 给出清晰反馈。
- 为角色 review、preview 或 sandbox 相关反馈补上 skill-resolution cues，让操作者能在保存前看到当前 draft 中哪些 skills 已解析、哪些来自继承或模板、哪些只是保留的手动引用。
- 让内置示例 role 引用的 skill path 与仓库里的 canonical sample skills/fixtures 对齐，避免文档和示例 role 长期引用并不存在的 skills。
- 增加 focused tests 和文档更新，覆盖 skill catalog 发现、角色 skills 选择/回填、unresolved 警示、手动 path 回退，以及 sample skills 与角色示例的一致性。

## Capabilities

### New Capabilities
- `role-skill-catalog`: 为角色 authoring 提供 authoritative 的 skill inventory、来源元数据与解析状态，而不是依赖前端硬编码或纯手填 path。

### Modified Capabilities
- `role-management-panel`: 角色工作台需要从“结构化 skill rows”升级到“可发现、可选择、可解释、可回退为手动 path 的完整 skills authoring”。
- `role-authoring-sandbox`: preview 或 sandbox 相关作者反馈需要显示 role skills 的解析状态、来源信息和 unresolved 提示，帮助操作者在保存前判断技能树是否可解释。

## Impact

- Affected backend/catalog seams: role authoring APIs or helpers that surface role-related discovery metadata, plus any server-side skill inventory or fixture loading added for the dashboard.
- Affected frontend role surfaces: `components/roles/*`, `lib/roles/role-management.ts`, `lib/stores/role-store.ts`, and any role preview/sandbox result mapping used by the dashboard.
- Affected sample assets and docs: canonical skill fixtures under supported skill roots, sample role manifests under `roles/`, `docs/role-authoring-guide.md`, and role/skill design notes that currently imply skills exist without a repo-truthful catalog.
- Affected verification: focused tests for skill catalog discovery, role workspace skill selection and manual fallback, preview/sandbox resolution cues, and sample role-to-skill consistency.
