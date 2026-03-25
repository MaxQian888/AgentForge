## Why

`docs/PRD.md` 和 `docs/part/PLUGIN_SYSTEM_DESIGN.md` 都把角色里的技能树定义成 `capabilities.skills` 的一等能力，技能项至少需要 `path` 与 `auto_load` 语义，用来表达角色的可复用专业知识与按需加载策略。但当前仓库里的角色链路还没有真正支持这部分：Go 侧 `RoleCapabilities` / YAML 解析没有技能模型，角色 API 和样例角色不会保留技能项，前端角色工作台也只支持工具、语言、框架等浅层能力字段，导致操作者只能把技能信息塞进 tags、prompt 或备注里。

现在需要把角色 Skills 这条链路补齐到可实施状态，避免 AgentForge 的 Role Plugin 能力继续停留在“文档已定义、产品面缺失”的半成品阶段，也为后续 `/opsx:apply` 提供一条聚焦且完整的实施范围。

## What Changes

- 为角色 manifest、Go 侧规范化模型、YAML 读写和角色 API 增加 `capabilities.skills` 支持，技能项采用结构化 `{ path, auto_load }` 形式并在 list/get/create/update 链路中保留。
- 为角色工作台补上结构化 Skills 编辑体验，支持添加/删除技能项、切换自动加载、保留声明顺序，并在保存前阻止空路径或重复路径这类无效配置。
- 为模板起步、继承起步和角色编辑回填补上技能链路，使 Skills 不再在 draft 序列化或角色复制过程中被静默丢弃。
- 为角色列表卡片和工作台摘要补上技能可见性，展示技能数量、自动加载与按需加载拆分，以及关键技能路径提示，减少操作者必须打开 YAML 才能理解角色能力的情况。
- 更新样例角色与聚焦测试，覆盖技能项的解析、归一化、API round-trip、前端编辑和继承合并行为。
- 保持当前执行运行时 contract 稳定；本 change 只补齐 Role Skills 的声明、管理和可见性，不在本次里把技能项直接变成 Bridge 自动注入或插件市场安装流程。

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `role-plugin-support`: 角色 manifest、规范化存储和角色 API 需要支持 `capabilities.skills` 的结构化技能树与继承语义。
- `role-management-panel`: 角色工作台和角色列表需要支持结构化 Skills 编辑、校验、回填与摘要展示。

## Impact

- Affected Go role contract and persistence seams: `src-go/internal/model/role.go`, `src-go/internal/role/*`, `src-go/internal/handler/role_handler.go`, related role tests, and canonical role YAML assets under `roles/`
- Affected frontend role surfaces: `app/(dashboard)/roles/page.tsx`, `components/roles/*`, `lib/roles/role-management.ts`, and `lib/stores/role-store.ts`
- Affected examples and verification: sample role manifests, parser/store/API tests, and focused role workspace/card tests
- No new external dependency is required; runtime execution, plugin marketplace, and member/team skill-matching remain outside this change
