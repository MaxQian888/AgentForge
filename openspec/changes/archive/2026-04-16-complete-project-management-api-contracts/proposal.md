## Why

AgentForge 已经具备项目创建、bootstrap handoff、workflow templates、sprint planning 与 project dashboard 等项目管理表面，但项目上下文在前后端仍未形成一套统一、可验证的合同。尤其是 workflow template / workflow definition 的项目作用域、`/projects` 列表摘要字段，以及 dashboard bootstrap 对模板就绪度的统计口径仍存在语义漂移，导致“项目管理完整”在真实运行中仍会出现跨项目误读、假完整指标或错误作用域。

## What Changes

- 建立一套项目管理显式上下文与 API 合同，统一项目作用域在前端路由、store、后端 handler、repository 和响应 DTO 中的表达方式。
- 补齐 workflow template 与 workflow definition 相关接口的项目作用域传递和边界校验，确保模板可见性、发布、克隆、执行、删除都绑定当前项目上下文而不会泄露或误落到错误作用域。
- 校准 `/projects` 列表及相关项目摘要返回字段，使前端项目卡片、项目统计和 bootstrap/overview 入口展示真实项目状态，而不是依赖前端默认值伪造完整性。
- 统一 dashboard bootstrap 所消费的模板/规划就绪度数据口径，使项目 bootstrap readiness 与 docs/workflow/sprint 等工作区看到的当前项目真相一致。
- 保持 scope 聚焦于现有项目管理合同校准，不重新打开已完成的 task board、project dashboard workspace、docs template center 或 sprint workspace 大范围重构。

## Capabilities

### New Capabilities
- `project-management-api-contracts`: 定义项目列表摘要、项目详情、显式项目上下文解析、以及跨项目管理工作区共享的项目作用域 API 合同与边界约束。

### Modified Capabilities
- `project-bootstrap-handoff`: bootstrap summary 与 handoff readiness 必须基于当前项目的真实模板/规划数据口径，而不是依赖漂移的聚合结果。
- `workflow-template-library`: workflow template discovery、publish、clone、execute 与 delete 需要消费统一的显式项目上下文合同，并保证自定义模板不会跨项目泄露。

## Impact

- 前端项目管理入口与状态层：`app/(dashboard)/projects/page.tsx`, `components/project/project-card.tsx`, `app/(dashboard)/page.tsx`, `lib/stores/project-store.ts`, `lib/stores/dashboard-store.ts`, `lib/stores/workflow-store.ts`, `lib/route-hrefs.ts`.
- 项目作用域工作区与 handoff 消费面：`app/(dashboard)/workflow/page.tsx`, `app/(dashboard)/docs/page.tsx`, `app/(dashboard)/sprints/page.tsx`, `app/(dashboard)/project/page.tsx`, `app/(dashboard)/project/dashboard/page.tsx`.
- 后端项目与 workflow 合同：`src-go/internal/server/routes.go`, `src-go/internal/middleware/project.go`, `src-go/internal/handler/project_handler.go`, `src-go/internal/handler/workflow_handler.go`, `src-go/internal/repository/project_repo.go`, `src-go/internal/repository/workflow_definition_repo.go`, `src-go/internal/service/workflow_template_service.go`, 以及对应 model / test 覆盖。
- 相关 API/DTO：`/api/v1/projects`, `/api/v1/projects/:id`, workflow template / workflow definition 相关接口，以及 dashboard/bootstrap 所依赖的项目摘要聚合输入。