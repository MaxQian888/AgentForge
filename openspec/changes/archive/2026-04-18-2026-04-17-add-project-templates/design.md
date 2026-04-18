## Context

AgentForge 有几种既有"模板"概念，边界需要在本 change 里先说清楚避免被错位实现：

- `workflow-template-library`：工作流级模板（一条 DAG 可被复用），**不是**项目级配置快照。
- `built-in-workflow-starters`：开箱即用的 workflow 起步集合，系统维护。
- `docs/document-templates`：文档模板（wiki page 模板），纯内容。
- `marketplace-item-consumption`：plugin/skill/role 安装通道。

**项目模板** 的独特作用：它是"一个项目的初始配置画像"——新建项目时一次性导入 settings + fields + views + dashboards + automations + workflow definitions + task statuses + 角色占位——让新项目直接就绪。

这是目前唯一没被覆盖的一层，也是 bootstrap handoff 真正能给新用户快速到"可开工"状态的关键。

## Goals / Non-Goals

**Goals:**
- 项目模板实体独立于 workflow/doc/marketplace 已有模板，schema 明确声明包含哪些配置子树。
- 支持 system（内置）、user（本人保存）、marketplace（从市场安装）三种来源；权限模型清晰。
- 克隆过程作为一个原子操作：或者整个项目连同配置一起建成，或者什么都不建（事务边界）。
- 模板快照不含业务数据（members/tasks/wiki/runs/logs/memory/invitations），避免误导用户"克隆了项目就把历史也拷了过来"。
- 不破坏现有空白建项目路径（`POST /projects` 不带模板参数继续工作）。

**Non-Goals:**
- 不实现 marketplace 侧项目模板发布流（只实现接收端）。
- 不做模板 diff / 应用差异更新（克隆是"复制当时的配置"，之后独立演化）。
- 不做模板版本回滚/变更历史（user 模板可被作者修改覆盖；system 模板随代码发布）。
- 不做模板间继承或组合（一个模板不从另一个模板派生）。
- 不做参数化模板（{{projectName}} 占位替换之类）—— snapshot 是直接复制值。

## Decisions

### 1. Snapshot schema 明确声明，固定版本号

`snapshot_json` 是结构化 JSON，顶层字段枚举清单：

```json
{
  "version": 1,
  "settings": { "reviewPolicy": {...}, "defaultCodingAgent": {...}, "notificationPrefs": {...} },
  "customFields": [ { "definitionFields": ..., "reorder": ... } ],
  "savedViews": [ ... ],
  "dashboards": [ { "layout": ..., "widgets": [...] } ],
  "automations": [ { "ruleDef": ..., "configuredByUserIdOmitted": true } ],
  "workflowDefinitions": [ { "definitionJson": ..., "templateRef": null } ],
  "taskStatuses": [ ... ],
  "memberRolePlaceholders": [ { "label": "PM", "suggestedRole": "admin" } ]
}
```

`version` 字段用于未来演化；apply 时按 version 走 upgrade path。字段为数组时保持顺序（saved views 顺序、custom fields 顺序都有语义）。

**禁止**字段：任何 `*_id`、`created_at`、`updated_at` 等会与新项目实例冲突的字段在 snapshot 序列化时剥离。

**Why this**：固定 schema + 版本号让模板可演化；明确清单避免"无意间把什么都存进去"。

**Alternative rejected – 存原始数据库行**：会把 `project_id`、`user_id` 等硬绑标识带进来，复制到新项目时全部要重写，容易漏。

### 2. 三种来源的权限与存储边界

| source | 存储 | 创建 | 编辑 | 删除 | 可见 |
|--------|------|------|------|------|------|
| `system` | 内置 bundle（代码/配置） | 只能随 release 新增 | 不可运行时编辑 | 不可删 | 所有登录用户 |
| `user` | DB 行，绑定 `owner_user_id` | `admin+` 在某项目上 save-as-template | owner 本人 | owner 本人 | owner 本人 + admin 全局可见？（见下） |
| `marketplace` | DB 行，来自 marketplace install | marketplace install 创建（自动归到 user source，`owner_user_id=installer`） | installer（但通常不改） | installer | 同 user |

user 模板是否可见给同组织其他用户？当前设计：**默认私有**（只 owner 可见）；若未来需要组织共享，引入 `visibility=private|org` 字段扩展。

**Why this**：先以最小共享边界开始；共享权限是独立治理话题，不和基础模板能力耦合。

### 3. 克隆流程是事务，失败回滚

`CloneFromTemplate(ctx, templateID, createProjectReq, initiatorUserID)` 执行顺序：

1. 开事务；
2. 复用 Wave 1 RBAC 里的项目创建原子动作（写 project + 建 owner member）；
3. 按 snapshot 字段顺序应用到新 projectID：settings → customFields → savedViews → dashboards → automations → workflowDefinitions → taskStatuses；
4. 任一步失败整体回滚；
5. 成功后提交，返回新 projectID；
6. 返回后异步发射 audit event `project_created_from_template`（包含 templateID 与 version）。

order 里 automations 先于 workflowDefinitions 是**反**的吗？不是——automation 可能引用 workflow，所以 workflow 先、automation 后。上面的顺序有误。**修正顺序**：settings → customFields → savedViews → taskStatuses → workflowDefinitions → dashboards → automations（保留之前列表时的语义：先基础定义，再依赖基础定义的资源）。

**Why this**：按依赖拓扑应用避免中间态；事务保证整体性；新建项目如果中断不会留下半配项目。

### 4. automations 克隆时剥离 `configuredByUserID`

- snapshot 存 automation 规则定义（trigger、action、conditions），但不存 `configuredByUserID`（权限快照）。
- 克隆后，新 automation 以"未激活"状态存在；initiator 必须重新确认激活，激活时系统把 `configuredByUserID` 设为 initiator。
- 这条和 RBAC 的 `rbac_snapshot_invalid` 规则一致——模板不能把旧发起者的权限带进新项目。

**Why this**：权限快照不可跨项目复制；否则模板会成为权限绕过通道。

### 5. workflowDefinitions 克隆复用 `workflow-template-library` 的 snapshot

- 如果项目的某个 workflowDefinition 本来就是从某 workflow template 派生（有 `templateRef`），snapshot 保留 `templateRef` 但也序列化当前 definition 内容；克隆时复用 workflow-template-library 的已有 clone 能力。
- 这保证两个系统的 workflow 快照口径一致，不是在项目模板里再造一套。

**Why this**：避免 workflow 层的两套克隆语义不一致。

### 6. Marketplace 侧复用现有 install seam

- `src-marketplace` 在 item_type 里新增 `project_template`。
- 主 Go 后端的 `/api/v1/marketplace/install` 接收到 `item_type=project_template` 时，把 marketplace 的模板 payload 转写为 user source 的项目模板（`source=marketplace`，`owner_user_id=installer`）。
- 本 change **只实现接收**；marketplace 侧发布流（publish/version/review）走它自己的 spec，follow-up 再扩展 `marketplace-item-consumption` 支持 `project_template`。

**Why this**：单独发 marketplace 侧 spec 不在本 change 范围；但先把接收端做完，这样 marketplace 一旦发布就能消费。

### 7. `memberRolePlaceholders` 是 advisory

- snapshot 里的 `memberRolePlaceholders`（例如"PM / Tech Lead"）不会自动创建 pending 邀请。
- 它仅作 UI 建议：新建项目后，导航侧一个 checklist 提示"建议加 PM 和 Tech Lead"。
- 不做自动邀请有两个原因：(1) 模板不知道具体人是谁；(2) 邀请流已独立 change 管理，不想耦合。

**Why this**：保持"模板 = 配置"边界清晰。邀请是独立动作，不该被模板隐式触发。

## Risks / Trade-offs

- **[Risk] snapshot schema 演化导致旧模板不兼容**：→ **Mitigation**：`version` 字段 + upgrade path；超过 N 个版本后允许 deprecation 提示用户重建模板。
- **[Risk] 模板数据过大（比如包含大量 dashboard widget 配置）**：→ **Mitigation**：snapshot 作为 JSONB 存，单 template 不超过 1 MB（sanity bound）；超过需重新审视是否真的是"配置"。
- **[Risk] 克隆事务过大**：很多 subresource 在一个事务里 apply 可能超时。→ **Mitigation**：按 subresource 分批 commit？——不，事务完整性优先；给事务留足时间 + 限制模板大小更合理。
- **[Risk] workflowDefinitions 与 workflow-template-library 的克隆实现漂移**：两套代码对"克隆 workflow"可能出现不一致。→ **Mitigation**：project-template clone 调用 workflow-template-library 的 clone 服务作为子例程，不重实现。
- **[Risk] user 模板隐私泄漏**：误把包含敏感 settings 的模板分享。→ **Mitigation**：snapshot 构造器对 settings 子集白名单，明确哪些字段会进 snapshot；sensitive（token、secret、oauth config）一律 strip。

## Migration Plan

1. Schema + model + repo + snapshot sanitizer（复用 audit sanitizer 的 denylist 思路）。
2. service：`BuildSnapshot(fromProjectID) → snapshot_json`、`ApplySnapshot(toProjectID, snapshot)` —— 先单独写并测，再组合成 `CloneFromTemplate` 事务。
3. handler + 路由：模板 CRUD + save-as-template + POST /projects 扩展。
4. 内置 system 模板 bundle：至少提供 `"Starter Agile Project"` 一个系统模板作为 baseline，覆盖基础 customFields/dashboards/automation，类似 `built-in-workflow-starters` 的组织方式。
5. 前端：new-project-dialog 扩展模板选择；模板管理页；save-as-template dialog。
6. Marketplace install seam 接收端改造。
7. 端到端测试：空白创建不变；模板创建后新项目等价于模板快照；save-as-template 往返（从项目 A 存模板 → 从模板建项目 B → 配置相等）；破坏性测试：删除已被 marketplace 引用的 user 模板不影响已克隆项目（snapshot 是复制非引用）。
8. 回滚：可以。模板表独立，删除它不影响业务；feature flag 控制 POST /projects 是否接受 templateId。

## Open Questions

- user 模板是否引入"组织共享"（`visibility=org`）？本 change 先私有，follow-up 决定。
- system 模板是否支持运行时通过 config 扩展而非代码？当前坚持代码 + bundle，和 `builtin_bundle` 模式一致；避免"配置里定义系统模板"的治理复杂度。
- `taskStatuses` 是当前项目是否支持自定义任务状态的产物；如果 AgentForge 仍是固定几状态，这个字段就是空列表占位。项目任务状态的扩展属 follow-up。
- 从模板创建项目时 `memberRolePlaceholders` 如何在 UI 呈现？作为 bootstrap checklist 的一部分更自然；这条和 project-bootstrap-handoff 的 spec 一起演进。
