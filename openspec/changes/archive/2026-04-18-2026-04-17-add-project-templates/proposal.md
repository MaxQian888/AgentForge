## Why

项目初始化当前是空白起步：新建一个项目后需要手动配置 workflow、custom fields、saved views、automation rules、dashboard layout、member 角色占位等才能真正开工。对于团队内"每个新项目结构都差不多"的典型场景，这是重复劳动；对新用户而言，没有参考模板就无从下手。

现有仓库里 `workflow-template-library` 已覆盖 workflow 层的模板，`built-in-workflow-starters` 覆盖开箱即用的流程起步，但没有"**整项目级**模板"——把一个项目的配置快照成可克隆单元的能力。

这个 change 补齐项目模板能力，支撑两种路径：
1. 从模板创建新项目（`POST /projects` 支持 `templateSource + templateId`）。
2. 将现有项目存为模板（`POST /projects/:pid/save-as-template`）。

## What Changes

- 新增 `project_templates` 实体与表：`id`、`source`（`system|user|marketplace`）、`owner_user_id`（user source 专用）、`name`、`description`、`snapshot_json`（结构化配置快照）、`snapshot_version`（schema 演化用）、`created_at`、`updated_at`。
- 模板 snapshot 内容范围（固定 schema，不含业务数据）：
  - `settings`：review policy、默认 coding-agent selection、通知偏好等 project settings 子集（不含 secret/token）；
  - `customFields`：字段定义；
  - `savedViews`：默认视图；
  - `dashboards` + `widgets`：dashboard 布局；
  - `automations`：自动化规则（stripped `configuredByUserID`，由新项目创建者在创建时 rebind）；
  - `workflowDefinitions`：project-owned workflow definitions 的 schema 快照（通过 `workflow-template-library` 复用）；
  - `taskStatuses`：自定义任务状态（若项目支持自定义）；
  - `memberRolePlaceholders`：建议的角色占位（例如"PM / Lead Engineer / Reviewer"），仅作 advisory，不自动建成员。
- 模板**不包含**：members、tasks、wiki pages、comments、runs、logs、memory entries、invitations。这些是"项目产生的内容"而非"项目配置"，不应复制。
- 新增 API：
  - `POST /projects` 请求体扩展 `{templateSource?, templateId?}`；为空则行为不变（空白项目）。
  - `POST /projects/:pid/save-as-template`（`admin+`）：从当前项目快照生成 user source 模板，绑定 owner_user_id=caller。
  - `GET /project-templates`：列出 system + user(current user) + marketplace(代理查询 `src-marketplace`)。
  - `GET /project-templates/:id`：详情含 snapshot 摘要。
  - `PUT /project-templates/:id`（user source，owner 本人）：更新自己的模板。
  - `DELETE /project-templates/:id`（user source，owner 本人）：删除自己的模板。
  - system 模板通过 repo 内置 bundle（类似 `builtin_bundle.go`）静态注册，不可删。
  - marketplace 模板安装后变成 user source 的本地副本（复用现有 marketplace install seam，不在本 change 中扩展 marketplace 后端；标注为 follow-up）。
- 克隆实现细节：`project_lifecycle_service.CloneFromTemplate(templateID, newProjectRequest, initiatorUserID)` 事务内：先创建项目 + 建 owner member（沿用 RBAC change 的原子创建），再按 snapshot 顺序写入各子资源；失败整体回滚。
- 破坏性：无。空 body 的 `POST /projects` 行为不变。

## Capabilities

### New Capabilities
- `project-template-library`：定义项目模板实体、模板来源分类、snapshot schema 边界（配置而非业务数据）、保存/克隆/管理 API 契约。

### Modified Capabilities
- `project-bootstrap-handoff`：bootstrap summary 与创建流可消费模板来源，引导新项目选择模板或空白起步；readiness 口径不变。
- `project-management-api-contracts`：`POST /projects` 接受可选 `templateSource+templateId`；ownership 规则保持（克隆结果仍属于新建项目所有权）。

## Impact

- DB 迁移：新表 `project_templates` + 索引 `(source, owner_user_id)`；`snapshot_version` 独立 column 便于演化。
- 后端：`internal/model/project_template.go`（新）、`internal/repository/project_template_repo.go`（新）、`internal/service/project_template_service.go`（新，含 snapshot 构造/应用）、`internal/handler/project_template_handler.go`（新）、`internal/service/project_lifecycle_service.go`（扩展 CloneFromTemplate）、`internal/service/builtin_bundle.go` 或新增 `builtin_project_template_bundle.go`（内置 system 模板）、`internal/server/routes.go` 挂接。
- 前端：`lib/stores/project-template-store.ts`（新）、`components/project/new-project-dialog.tsx`（扩展模板选择步骤）、`components/project/save-as-template-dialog.tsx`（新）、`app/(dashboard)/project/templates/page.tsx`（模板管理页）、国际化 `messages/**/project-templates.json`。
- Marketplace 集成：`src-marketplace` 通过现有 `/api/v1/marketplace/install` seam 投递 `item_type=project_template` → 主 Go 转存为 user source 模板。本 change **只实现接收与转存**，marketplace 侧发布流在 follow-up。

## Depends On

- `2026-04-17-add-project-rbac`：`project.save_as_template` / `project_template.create|update|delete` ActionID 加入矩阵（save 需 `admin+`；管理自己的 user 模板需登录即可、不受项目 role 约束）；克隆时创建项目走 Wave 1 的 owner 原子登记路径。
