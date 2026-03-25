## Context

`docs/PRD.md` 和 `docs/part/PLUGIN_SYSTEM_DESIGN.md` 都把角色技能树定义在 `capabilities.skills` 下，技能项使用 `path` 与 `auto_load` 表达“自动加载的专业知识”和“按需加载的专业知识”。但当前仓库里的真实实现没有把这部分贯通起来：

- Go 侧 `src-go/internal/model/role.go` 的 `RoleCapabilities` 没有技能引用模型。
- `src-go/internal/role/parser.go` / `store.go` 只处理工具、语言、框架、预算等字段，技能项无法 round-trip。
- `lib/stores/role-store.ts`、`lib/roles/role-management.ts` 和 `components/roles/role-workspace.tsx` 只支持浅层 capability 字段，角色草稿在模板、继承、保存时都会丢掉 Skills。
- 角色列表卡片和摘要也没有任何技能可见性，操作者无法判断某个角色会自动加载哪些技能。

这个 change 需要补的不是完整插件生态，而是把“角色里的 Skills”从 PRD 里的文档字段变成当前仓库可配置、可保存、可比较、可测试的真实能力。

## Goals / Non-Goals

**Goals:**

- 为角色 schema、YAML、API 和前端 store 建立统一的 `capabilities.skills` 结构化模型。
- 让角色工作台可以结构化编辑技能路径与自动加载标记，并在模板、继承、编辑回填时完整保留这些数据。
- 让角色卡片与工作台摘要显示技能数量、自动加载/按需加载拆分和关键路径提示。
- 为技能项补齐基础校验与继承合并语义，避免空路径、重复路径和不稳定合并结果。
- 补齐 focused tests，覆盖解析、归一化、UI 编辑和保存行为。

**Non-Goals:**

- 不在本次里实现 Bridge 自动加载 Skills、技能安装、技能市场、技能签名或运行时分发。
- 不把成员页 `skills` 标签系统重构成与角色 Skills 共用一套底层能力图谱。
- 不新增全局技能目录服务或 Marketplace catalog；本次只保证角色里声明的 Skills 能被正确管理和展示。

## Decisions

### 1. 用一等的结构化技能引用模型承接 `capabilities.skills`

角色技能项新增 typed model，例如 `RoleSkillReference { path, autoLoad }`，并与 YAML 的 `auto_load` 字段保持一一映射。

原因：

- 这和 PRD/插件设计文档里的契约完全一致，避免再次发明 tags、字符串拼接或 `custom_settings` 这类临时结构。
- 结构化模型更适合后续做继承合并、表单编辑、摘要统计和 API round-trip。

备选方案：

- 把技能继续塞在 `metadata.tags` 或 `knowledge.documents` 里。缺点是语义错误，无法表达 `auto_load`，也无法和角色能力层对齐。
- 用逗号分隔字符串直接存进 capability 自定义字段。缺点是前后端都需要重复解析，无法稳定扩展。

### 2. 技能项先作为声明式角色配置保留，不改变当前执行 profile contract

本次把 Skills 视为角色配置与操作者可见性数据，而不是当前 Bridge 自动执行的输入。角色 execution profile 继续维持现有运行时最小合同，避免这次 change 横向扩散到 runtime 注入逻辑。

原因：

- 当前产品缺口首先是“看不到、存不住、配不出来”，不是 Bridge 已经定义了 Skills 注入 contract 却没人接。
- 把 UI/schema 补齐和 runtime 自动装载绑定在一起，会直接扩大到插件安装、环境依赖、Bridge 兼容和 Agent 启动语义，超出本次 focused change 边界。

备选方案：

- 在本次里同步把 auto-load skills 注入执行运行时。缺点是需要额外定义 Bridge contract、错误语义和安装来源，风险过大。

### 3. 角色工作台使用结构化行编辑，而不是回退成 CSV 或 raw YAML

前端草稿层新增 skills rows，允许每个技能项独立编辑 `path` 和 `autoLoad`，并通过共享 helper 负责 draft 构建、校验、序列化和摘要统计。

原因：

- 当前角色工作台已经是结构化编辑体验，Skills 继续用 CSV 或 raw JSON 会让能力面割裂。
- 结构化行编辑更容易支持模板复制、继承覆盖和 inline validation。

备选方案：

- 在现有 Capabilities 区加一个逗号分隔输入框。缺点是无法表达 `auto_load`，也无法阻止重复 path 这类错误。

### 4. 校验以“路径有效性 + 去重”为硬约束，不做硬性文件存在校验

本次保存时会阻止空路径和重复路径；技能路径的本地存在性不作为 hard blocker。

原因：

- 角色可能引用未来才会同步到当前工作区的 skill，或者引用安装后才存在的 skills source，强制文件存在校验会让角色配置流过度依赖当前运行环境。
- 路径格式与去重是当前最稳定、最通用的 correctness 约束。

备选方案：

- 保存前强制检查每个 `path` 在本地文件系统都存在。缺点是对 Web 模式、跨环境同步和未来 registry 来源都过于脆弱。

### 5. 继承合并按 skill path 去重，子角色同路径覆盖父角色

角色继承解析时以 `path` 作为稳定 key：父角色技能先进入结果，子角色若声明同一路径则覆盖该项（例如修改 `auto_load`），新的技能则按子角色声明顺序追加。

原因：

- 这样既能保留父角色默认技能顺序，又能避免相同 skill path 在结果里重复出现。
- “同路径覆盖”最符合角色继承的心智模型，比简单拼接更稳定。

备选方案：

- 直接父子数组拼接。缺点是会产生重复技能，UI/摘要/API 都要额外消解冲突。

## Risks / Trade-offs

- [角色 schema 与旧样例 YAML 之间出现双写漂移] → 通过更新 canonical `roles/*/role.yaml` 样例和 parser/store tests 保持真实样例跟随 contract 演进。
- [前端角色工作台继续膨胀] → 把 Skills 相关状态、校验和摘要逻辑集中到 `lib/roles/role-management.ts`，避免组件内散落逻辑。
- [未来 runtime 需要不同的 Skills 语义] → 本次只固化声明层 contract，执行层保持未承诺状态，为后续 runtime change 留出兼容空间。
- [旧代码回退后编辑角色会丢失 Skills] → 把回退边界写清楚：若回退到不支持 Skills 的旧代码，必须同时避免通过旧写路径重新保存含技能的角色。

## Migration Plan

1. 先扩展 Go 侧角色模型、YAML 解析、规范化和继承合并逻辑，保证角色 API 可以稳定读写 Skills。
2. 更新 canonical role assets 和测试夹具，确保样例角色与 parser/store 逻辑一致。
3. 扩展前端 role store、draft helpers 和角色工作台，使 Skills 可以被编辑、校验、预览和保存。
4. 更新角色卡片与摘要可见性，并补齐 focused frontend/backend tests。

回滚策略：

- 这是一个以可选字段为主的增量 change。未声明 `capabilities.skills` 的旧角色保持兼容。
- 如果需要回退，必须一起回退 Go 侧读写链路和前端编辑链路，避免旧写路径重新保存角色时把 Skills 静默丢掉。

## Open Questions

- 后续是否要为角色 Skills 增加“本地可发现但非阻塞”的存在性提示或 catalog API？本 change 暂不包含。
- 后续 Bridge/runtime 是否需要消费 `autoLoad=true` 的技能列表？本 change 明确延后该 contract。
