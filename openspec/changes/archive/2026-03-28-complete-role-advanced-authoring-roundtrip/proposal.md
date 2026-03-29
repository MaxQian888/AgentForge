## Why

AgentForge 已经具备结构化角色工作台、预览与 sandbox 流程，但当前“角色自定义”仍没有完整覆盖项目文档承诺的高级 authoring 面。`docs/role-yaml.md`、`docs/role-authoring-guide.md` 和现有主 spec 都把 `custom_settings`、更完整的 tool host / knowledge / memory / overrides 等高级字段纳入 Role YAML 合同，而前端草稿与保存链路仍存在“不暴露、只回填、不可靠 round-trip”的缺口，导致操作者继续保存角色时有静默丢失高级配置的风险。

现在需要把这条 seam 单独补齐，避免已经存在的 Go 侧 role store、preview/sandbox 和 YAML source of truth 再次与 dashboard authoring 漂移，也让“角色可定制”真正符合当前 PRD 和角色作者指南，而不是只对基础字段成立。

## What Changes

- 为角色工作台补齐高级 authoring 分区与字段模型，覆盖当前文档和 Go 侧已支持但 UI 仍未完整编辑或可靠保留的配置，如 `capabilities.custom_settings`、更完整的 tool host 配置、knowledge memory / shared source 细节、以及 `overrides` 的可见性与安全编辑边界。
- 修正角色草稿构建、模板起步、继承起步、预览、sandbox 和保存链路中的 round-trip 语义，确保未在当前界面显式修改的高级字段不会在 create/update 时被静默丢弃或退化。
- 让角色工作台在 preview / sandbox / YAML 视图中明确展示高级字段的 effective state、继承来源和保存影响，帮助操作者在提交前识别“当前值来自父角色、模板还是本地覆盖”。
- 为角色 API 与前端 store 增加 focused 校验和兼容规则，保证高级字段在 list/get/create/update/preview/sandbox 往返中保持一致，并对 unsupported 或只读段给出明确反馈，而不是隐式吞掉。
- 补充文档与 focused tests，使角色作者指南、Role YAML 文档、样例角色和工作台交互对高级角色自定义的描述保持一致。

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `role-plugin-support`: 角色 YAML、规范化模型与 API 需要对高级 authoring 字段提供稳定 round-trip 语义，并明确哪些字段可编辑、只读或受继承/覆盖规则约束。
- `role-management-panel`: 角色工作台需要把高级角色配置纳入结构化 authoring、预览和保存流程，而不是只覆盖基础字段并在保存时丢失其余配置。
- `role-authoring-sandbox`: preview 与 sandbox 结果需要对高级字段、继承来源和 readiness/validation 问题提供更可操作的作者反馈。

## Impact

- Affected frontend role authoring surfaces: `app/(dashboard)/roles/page.tsx`, `components/roles/*`, `lib/roles/role-management.ts`, and role store/types/tests.
- Affected Go role contract and authoring helpers: `src-go/internal/model/role.go`, `src-go/internal/role/*`, `src-go/internal/handler/role_handler.go`, and related preview/sandbox tests.
- Affected docs and examples: `docs/role-yaml.md`, `docs/role-authoring-guide.md`, canonical role YAML assets under `roles/`, and any focused verification fixtures for advanced role fields.
- No new external dependency is required; runtime execution semantics in `src-bridge` remain projection-based and are only updated if role preview or validation output needs richer authoring feedback.
