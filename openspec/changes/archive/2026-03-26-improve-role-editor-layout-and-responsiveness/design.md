## Context

AgentForge 当前的角色定制链路在合同层已经基本成立：角色库、结构化 draft、YAML 预览、preview/sandbox 入口和执行摘要都已经存在，`role-management-panel` 与 `role-authoring-sandbox` 也已经归档成主 spec。当前缺口不再是“有没有角色编辑器”，而是“这个编辑器是否真的适合高密度 authoring”。现有 `components/roles/role-workspace.tsx` 仍然把大量分区内容堆进单个长表单，并在右侧再放一组上下文卡片；这在大屏上已经显得拥挤，在中等和窄屏下更容易让角色库、当前编辑区、预览/沙盒上下文互相争抢空间。

这次设计要解决的是 UI/UX 结构问题，同时保持现有 role contract、preview/sandbox API 和文档事实不漂移。几个关键约束是：

- 不重新打开 role schema、preview/sandbox backend 或角色持久化的新需求，已有 contract 继续复用。
- 角色 authoring 仍需对齐 `docs/role-authoring-guide.md` 推荐顺序：模板/继承起步 -> identity -> capabilities/knowledge/governance -> preview -> sandbox -> save。
- 响应式适配不能靠“简单竖排所有内容”收尾，必须保证角色库、编辑动作、验证上下文仍可抵达且不丢状态。
- 当前组件已相当庞大，这次实现方向需要把布局与分区责任拆开，否则后续继续补字段会再次失控。

## Goals / Non-Goals

**Goals:**

- 将角色编辑器重构为清晰的 authoring workspace，让角色库、章节导航、分区编辑、说明/预览上下文各自有稳定位置。
- 定义桌面、中等宽度和窄屏下的响应式呈现规则，确保角色编辑、预览、沙盒和 YAML 检查在不同断点都可用。
- 让编辑器内的章节顺序、字段说明和引导文案与 `docs/role-authoring-guide.md`、PRD 术语保持一致。
- 为布局切换和关键 authoring flow 补充前端测试，防止后续改动再次破坏 editor 信息架构。

**Non-Goals:**

- 不修改 Go 侧 role schema、preview/sandbox 接口或角色持久化逻辑。
- 不新增 Role Marketplace、版本历史、团队共享、拖拽能力组合等更宽的产品面。
- 不在本次内解决所有字段控件精细化问题；重点是布局、导航、上下文组织和文档对齐。
- 不将角色编辑器单独抽成新的路由体系；仍在当前 dashboard roles 页面内完成。

## Decisions

### 1. 将 `RoleWorkspace` 拆成“catalog rail + editor shell + context rail”三类职责，而不是继续在一个组件里堆布局

实现上应把当前大组件至少拆成：

- catalog rail：角色库、创建入口、当前选择态
- editor shell：模板/继承起步、章节导航、当前分区内容、保存动作
- context rail：作者指南摘要、执行摘要、YAML 预览、preview/sandbox 区

这样做的原因是：

- 这次变化的核心是布局与信息架构，职责不拆开就很难为不同断点定义清晰行为。
- 当前单文件已经同时承担数据选择、布局分栏、字段渲染和上下文卡片管理，继续增长只会让响应式变更更危险。
- catalog/editor/context 三个区域天然对应不同的 responsive 折叠策略。

备选方案：

- 仅在现有 `RoleWorkspace` 上调 grid class 和局部顺序。优点是改动小；缺点是无法稳住后续断点切换和测试边界。

### 2. 使用“章节导航 + 分组顺序”表达作者流程，而不是默认所有分区平铺长滚动

编辑器主体应显式体现 authoring 顺序：

1. setup：模板、继承、当前模式
2. identity：基础身份和高级身份
3. capabilities
4. knowledge
5. security / collaboration / triggers
6. review：YAML、summary、preview/sandbox、save

桌面端可以让多个 section 同页显示，但仍要有清晰的导航锚点或切换器；中小屏则更适合一次聚焦一个 section，并保留上一步/下一步或章节切换。

这样做的原因是：

- `docs/role-authoring-guide.md` 已经给出推荐流程，UI 应该直接承接，而不是让用户自己决定滚到哪里。
- 角色字段很多，平铺式表单会让“现在正在做什么”变得模糊，尤其是中小屏。
- 章节导航能够自然承载校验态、未完成态和当前 section 定位。

备选方案：

- 保留纯长表单，只加 sticky 目录。优点是实现更快；缺点是窄屏下仍然很难形成稳定 authoring 流程。

### 3. 定义三档响应式行为，按 presentation 变化而不是按 capability 缩水

推荐断点策略：

- desktop：三栏同时可见，catalog/editor/context 并列
- medium：双栏或主从布局，editor 为主，catalog/context 通过折叠 rail、sheet、tabs 或分段切换进入
- narrow：单栏 authoring flow，catalog 与 context 变成显式切换面板，但草稿状态、校验态和 preview/sandbox 入口持续保留

关键原则不是“每个断点长什么样”，而是“每个能力都还能抵达”。任何断点都不能把角色库、YAML、summary、preview/sandbox 变成隐藏且难发现的功能。

这样做的原因是：

- 用户明确要求“布局合理正确，注意自适应”，这里需要 spec 级响应式行为而不是只改几个 Tailwind class。
- 当前三栏布局在 `xl` 以下会快速退化，必须有中间态而不是直接从三栏掉到一长列。
- 断点按 presentation 切换，才能避免不同 viewport 功能不一致。

备选方案：

- 只使用 CSS grid 自动换行。优点是简单；缺点是 context rail 与 catalog 常会掉到不可预测的位置，authoring 流会断裂。

### 4. 将文档说明变成 section-aware guidance，而不是静态通用提示卡

当前右侧 `Authoring Guide` 卡片过于泛化。新的 guidance 应该基于当前 section 或至少与 section 对应，例如：

- setup: 何时 template，何时 extends
- identity: role/goal/system prompt 对齐原则
- capabilities/knowledge: package/skill/shared knowledge 的区别
- security: permission/path/output filter 的治理含义
- review: preview 与 sandbox 的差异

说明内容继续来自仓库文档提炼，而不是新造第二套说法。

这样做的原因是：

- 用户明确要求“符合文档的要求”，这不是单纯引用文档，而是把文档中的 authoring 语义放进工作区。
- 静态卡片在大多数字段上帮助有限，section-aware guidance 才能真正减轻作者负担。

备选方案：

- 保留静态 guide 卡片，只扩充文字。优点是实现简单；缺点是阅读成本高，而且和当前编辑步骤脱节。

### 5. 用前端状态驱动的 layout tests 覆盖断点和 rail 可达性，而不是只保留字段提交测试

这次测试重点应新增：

- 角色库、编辑区、上下文区在不同 viewport/容器宽度下的可达性
- 章节导航和当前分区切换
- preview/sandbox/YAML 在 responsive 模式下仍可见或可通过明确入口打开
- 文档引导文案与当前 section 对齐

这样做的原因是：

- 现有测试更偏字段编辑和 API 调用，无法防止“布局改坏但功能仍能提交”的回归。
- 这是典型的 UI 结构变更，必须让测试验证 rail 是否存在、入口是否 discoverable。

备选方案：

- 仅补快照测试。优点是编写快；缺点是对交互和可达性保护不足。

## Risks / Trade-offs

- [拆分 `RoleWorkspace` 期间容易影响现有字段编辑行为] → 先保持现有 draft/serialization 逻辑不变，只重组容器和分区组件边界。
- [响应式断点设计过重，导致实现复杂度上升] → 优先定义三档稳定行为，避免每个小宽度都定制一套 UI。
- [文档引导内容复制过多后再次漂移] → 引导只保留提炼后的短句，术语和顺序严格对齐现有 authoring guide。
- [测试环境难以真实反映 CSS 布局] → 以“入口是否存在、区域是否切换、状态是否保留”为主做可达性测试，不依赖像素级断言。

## Migration Plan

1. 拆分 `components/roles/role-workspace.tsx`，先抽出 catalog rail、section model 和 context rail 组件，保持现有字段逻辑可运行。
2. 引入章节导航与新的 authoring shell，在不改后端 contract 的前提下重组 setup/edit/review 流。
3. 实现 desktop/medium/narrow 三档响应式呈现，并确保 role library、summary、YAML、preview/sandbox 在各断点都有明确入口。
4. 将作者指南文案按 section 接入工作区，更新需要同步的 `docs/role-authoring-guide.md` 文案。
5. 补充 focused 测试并做 scoped lint/test 验证。

回滚策略：

- 若新的多区布局引入严重可用性问题，可保留拆分后的内部组件结构，同时先回退到单页分组布局，不影响既有 role contract 和 preview/sandbox 能力。
- 文档引导与 responsive shell 可逐步启用；若某个 rail 的折叠方案不稳定，可先保留简单 tab/sheet 入口而不是完全回滚整个 change。

## Open Questions

- medium breakpoint 下更适合“右侧 context rail 折叠为 tabs”还是“独立 drawer/sheet”模式，需结合现有 dashboard pattern 再定最终呈现。
- 窄屏是否需要显式的 section stepper，还是仅保留目录切换就足够。
- YAML preview 与 sandbox 结果在窄屏下是否共用一个 review 面板，还是分成两个切换视图更清晰。
