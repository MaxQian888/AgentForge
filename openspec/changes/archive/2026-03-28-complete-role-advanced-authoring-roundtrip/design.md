## Context

AgentForge 当前的角色自定义能力已经过多轮拆分补齐：Go 侧有统一 role store、PRD 对齐的 YAML schema、preview / sandbox helper 和 execution profile 投影；前端也有结构化 role workspace、skills 编辑、执行摘要和响应式布局。现在真正剩下的缺口不再是“有没有角色编辑器”，而是“高级角色定义能否在 authoring 面中被安全地编辑、解释并 round-trip 保留”。

当前代码里，这个缺口主要体现在三类位置：

- `src-go/internal/model/role.go`、`src-go/internal/role/*` 和 `docs/role-yaml.md` 已经承认更完整的高级字段，例如 `capabilities.custom_settings`、tool host 配置、knowledge memory、`overrides`、更完整的 shared source 细节等。
- `lib/roles/role-management.ts` 与 `components/roles/*` 仍然主要按“当前 UI 暴露的字段集合”重建 draft/payload，一些高级字段只能依赖 `baseRole` 回填、只读保留，或者根本没有 authoring 入口。
- preview / sandbox 已经能返回 normalized/effective manifest，但当前角色作者在工作台里仍然不容易判断某个高级值来自父角色、模板还是当前草稿，也难以看清“这次保存会保留什么、覆盖什么、忽略什么”。

因此，这次 change 的目标是把“高级角色自定义”从文档/Go 合同里的隐含支持，提升为 dashboard 里可解释、可校验、可保留的真实 authoring 能力，同时不重新打开运行时消费范围。

## Goals / Non-Goals

**Goals:**

- 让角色工作台可以安全编辑当前项目文档和 Go 合同已经支持的高级角色字段，而不是只覆盖基础字段。
- 消除 role draft、template、inheritance、preview、sandbox、save 之间的 lossy round-trip，避免保存时静默丢失高级配置。
- 让工作台对高级字段提供清晰的“来源与影响”反馈，包括继承来源、当前覆盖状态、只读/受限字段边界，以及保存后的有效配置。
- 保持 Go 为 Role YAML single source of truth，前端只做 authoring 和校验，不引入第二套 schema 解释逻辑。
- 用 focused tests 和文档同步把这个 seam 固化下来，防止后续继续回退成“UI 只支持一部分，保存时剩余字段被吞掉”。

**Non-Goals:**

- 不把当前未被 Bridge 消费的高级字段变成新的 runtime 行为；execution profile 仍保持现有最小 runtime-facing contract。
- 不在本次里实现角色 Marketplace、团队共享、版本历史、回滚 UI 或跨项目 fork 流程。
- 不把所有高级字段都强行做成大型结构化所见即所得控件；对于高度开放或语义过宽的段，允许采用更受控的文本/YAML 子编辑器，只要校验和 round-trip 语义清楚。
- 不修改角色继承、preview、sandbox 的基础后端架构，只在现有 seams 上增强 authoring 可见性和一致性。

## Decisions

### 1. 角色草稿改为“完整 manifest 保守编辑”，不再按 UI 子集重建 payload

前端 draft 模型改为持有完整的 normalized role manifest，并在其上维护 UI 友好的编辑片段，而不是只根据当前表单字段重新构造提交 payload。显式支持的字段继续用结构化控件编辑；未显式展开的高级字段则作为保留段随 draft 一起 round-trip。

这样做的原因：

- 这是解决静默丢字段最直接的办法。只要某段配置已经被加载进 draft，就不应该因为当前页面没控件而在 update 时消失。
- 模板起步、继承起步、preview / sandbox、编辑已有角色都会共享同一份保守 manifest 语义，减少不同入口的漂移。

备选方案：

- 继续沿用当前“由表单字段反推 payload”的模式，再逐个补字段。缺点是每增加一段高级字段都会重复制造新的丢失风险，而且难以保证未来字段扩展安全。

### 2. 高级字段分成“结构化编辑段”和“受控原文段”

这次不追求把所有高级字段都拆成大量表单控件，而是按稳定性分两类：

- 结构化编辑段：`custom_settings`、tool host / MCP server、knowledge memory/shared-source 细节、以及其它已经有稳定列表/键值语义的字段。
- 受控原文段：`overrides` 这类开放度高、路径表达强、且容易因为 UI 过度抽象而误导用户的内容。对这类段，工作台提供可见、可校验、带说明的 YAML/JSON 子编辑器，并明确其保存边界。

这样做的原因：

- `overrides` 的语义本质上是声明式 patch，不适合在本次里膨胀成庞大的低价值表单系统。
- 但它也不能继续完全不可见，否则作者无法确认 child role 的真实覆盖面。

备选方案：

- 全部做成结构化表单。缺点是成本高、边界易失真，而且短期内更容易做出半正确的抽象。
- 全部退回 raw YAML 编辑。缺点是会破坏已完成的结构化 authoring 体验，也违背当前 role workspace 的方向。

### 3. preview / sandbox 增加“高级字段来源与保存影响”反馈，而不新增另一套分析服务

继续复用现有 `POST /api/v1/roles/preview` 和 `POST /api/v1/roles/sandbox`，但扩展返回结果或前端解释层，使工作台能展示：

- 哪些关键高级字段是 inherited / explicit / template-derived。
- 当前 draft 中哪些段将在保存时直接写回 canonical YAML。
- 哪些字段仅保留在 normalized role model 中、不会进入 execution profile。

这样做的原因：

- 角色作者真正需要的是“这次保存和 probe 会发生什么”，不是再开一条新的 authoring helper。
- 这些信息本来就建立在现有 normalized/effective manifest 之上，沿用 preview/sandbox 更符合 repo 现有架构。

备选方案：

- 新增单独的 provenance/diff API。缺点是重复后端逻辑，并且把 role authoring 辅助面切得过碎。

### 4. 高级字段校验以 Go 合同为准，前端只做阻断明显错误的辅助校验

前端会对空键、重复键、格式明显错误的 YAML/JSON 子编辑器输入、非法 shared source 行等做快速阻断；最终校验仍由 Go parser/store/preview/sandbox 给出 authoritative 结果。

这样做的原因：

- 角色 schema 真相已经在 Go 侧；把完整高级字段验证复制到前端会再次制造双份逻辑。
- 对 authoring 体验来说，前端只需要足够早地阻断显而易见的错误，并把后端返回映射回对应 section。

备选方案：

- 在前端复制完整 schema 校验。缺点是维护成本高，且更容易和 Go normalization 产生分歧。

## Risks / Trade-offs

- [高级 draft 模型变复杂] → 把“完整 manifest 保留层”和“UI 派生字段层”分清，避免在组件内散落 ad hoc merge 逻辑。
- [`overrides` 子编辑器难以让用户理解] → 在工作台中明确其用途、适用场景、只对 child role 有意义，并配合 preview 中的 effective 结果辅助解释。
- [旧角色或旧 UI 流程继续覆盖新字段] → 增加 focused round-trip tests，覆盖 list/get/edit/save/preview/sandbox 各入口。
- [前端辅助校验和 Go authoritative 校验不一致] → 保持前端校验最小化，并在 preview/sandbox/save 错误展示中回显后端 section-level 问题。
- [范围失控成新的“完整角色系统重构”] → 严格限制在高级 authoring completeness；不新增 runtime 消费，不重开 Marketplace/版本历史/共享等未来能力。

## Migration Plan

1. 扩展 role draft / serialization helpers，使现有角色加载后以完整 manifest 形态进入前端状态，并保留未展开的高级字段。
2. 为 role workspace 增加高级 authoring 分区与受控子编辑器，补上最小必要的来源/影响提示。
3. 调整 role API/preview/sandbox 响应映射和错误展示，确保高级字段结果可被前端解释。
4. 更新 canonical role fixtures、focused frontend/backend tests 和作者文档。
5. 如需回退，整体回退前端 draft/payload 改动与相关响应映射，避免出现“新 UI 保存过的高级字段再被旧写路径吞掉”的混合状态。

## Open Questions

- `overrides` 子编辑器最终采用 YAML 还是 JSON 作为输入形态更贴合当前角色作者习惯；本 change 默认以 YAML-oriented 编辑体验优先。
- preview / sandbox 的“来源”反馈是直接由后端返回明确 metadata，还是由前端根据 normalized/effective manifest 做有限解释；本 change 优先复用现有响应并只在必要处补最小后端字段。
