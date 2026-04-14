## Why

AgentForge 已经把多类 repo 内部 skills 变成真实工作流资产：`skills/*` 支撑 role catalog 与 marketplace，`.agents/skills/*` 承载 repo-local 开发技能，`.codex/.claude/.github` 里还有 OpenSpec 工作流技能镜像。但这些 skill family 现在缺少统一的 authoring/verification 规则：frontmatter 字段、agent config 命名、reference/test 配置、镜像同步方式和 provenance 记录都不一致，导致 repo 维护者很难判断“哪些 skill 是规范资产、哪些只是历史拷贝、哪些已经漂移”。

现在需要补这条线，是因为仓库已经不再只是“有几份 SKILL.md 示例”，而是同时依赖这些 assets 驱动 marketplace、role authoring、Codex/Claude/OpenSpec 工作流和外部同步来源。继续缺少统一标准，会让 skill 漂移、重复维护和消费方差异越来越难控。

## What Changes

- 为仓库内部维护的 skills 建立统一的治理模型，明确不同 skill family 的 canonical source、用途边界、必需元数据、允许的可选部件和验证 profile。
- 引入 repo-level internal skill registry / validation contract，覆盖 `skills/*`、`.agents/skills/*` 以及 `.codex/.claude/.github` 里的镜像技能，显式检查 frontmatter、agent config、mirror drift 和 provenance。
- 补齐 repo-maintained skill authoring / update 文档，说明何时写 references/scripts/tests、何时允许 profile-specific 例外、以及如何处理 `skills-lock.json` 记录的 upstream-synced skills。
- 把当前内置 built-in skills 与 OpenSpec workflow skills 接入这套统一标准，避免 `pnpm skill:verify:builtins` 只验证 marketplace 前提、却放过 repo 其他内部 skill 漂移。

## Capabilities

### New Capabilities
- `internal-skill-governance`: 定义 AgentForge 仓库内部 skills 的分类、canonical source、authoring profile、同步规则与自动化验证契约。

### Modified Capabilities
- None.

## Impact

- Affected skill assets: `skills/**`, `.agents/skills/**`, `.codex/skills/**`, `.claude/skills/**`, `.github/skills/**`, and `skills-lock.json`.
- Affected repo tooling: package scripts, new or updated validation/sync scripts, and any verification entrypoints currently centered on `pnpm skill:verify:builtins`.
- Affected docs: new or updated internal skill authoring guidance plus any repository docs that describe built-in or repo-local skills.
