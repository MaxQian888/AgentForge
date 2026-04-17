## Context

AgentForge 的项目管理已经分布在多个现有表面：`/projects` 负责项目入口，`/` dashboard 负责 bootstrap summary，`/settings`、`/docs`、`/workflow`、`/sprints`、`/project`、`/project/dashboard` 负责 project-scoped 管理与执行。前一轮 OpenSpec 已经把显式 handoff query contract 做出来，但当前前后端还有三类关键漂移：

1. `/projects` 列表与项目卡片依赖 `status`、`taskCount`、`agentCount` 等摘要字段，而当前后端 `ProjectDTO` 并未返回这些字段，前端只能用默认值兜底。
2. workflow template / workflow definition 的一部分接口仍是非 `projectGroup` 路由，前端通过 `X-Project-ID` 传递上下文，但后端并没有读取这个 header；同时 definition 的 `Get/Update/Delete` 仅按 `id` 操作，缺少项目边界校验。
3. dashboard bootstrap summary 的 playbooks readiness 同时消费 project-scoped docs/sprints 与 unscoped workflow template 统计，导致 bootstrap readiness 与实际 workflow workspace 真相可能不一致。

这个 change 是典型 cross-cutting seam：它跨 Next.js pages、Zustand stores、Echo routes、handler/service/repository 和 OpenSpec 既有 capability，且如果不先对齐技术决策，后续实现很容易把“显式上下文”再次做成新的漂移源。

## Goals / Non-Goals

**Goals:**
- 为项目管理相关 API 建立一套统一、显式、可验证的项目上下文合同。
- 让 `/projects` 列表、项目详情、project entry / bootstrap surface 使用真实服务端摘要，而不是前端默认值伪造完整性。
- 让 workflow template 与 workflow definition 的查询、发布、克隆、执行、删除都绑定当前项目上下文，并在边界错误时显式失败。
- 让 dashboard bootstrap readiness 与 docs/workflow/sprint 工作区使用相同的 project-scoped 数据口径。
- 保持现有前端 handoff query contract 不变，避免把已完成的 bootstrap/template/sprint workspace 重新翻修一遍。

**Non-Goals:**
- 不重做项目管理 UI、task board、project dashboard workspace 或 docs template center。
- 不引入新的 setup wizard 或新的项目生命周期状态机。
- 不在本 change 中重构所有非 projectGroup 的后端路由；只处理当前项目管理 seam 直接依赖的接口。
- 不扩展到 review、scheduler、team orchestration 等相邻但独立的控制面。

## Decisions

### 1. 用“显式 project-scoped contract”作为项目管理 API 的唯一真相
对当前 seam 中会读写 project-owned workflow/template state 的接口，设计上采用**显式项目上下文**作为唯一真相，且该真相必须能在 handler 层直接解析和校验。实现上优先走 `projects/:pid/...` 这类 path-scoped 合同，并允许在迁移期保留受控兼容层；不再把仅靠前端约定 header 或 ambient selection 视为 authoritative。

- **Why this**：`/projects/:pid/...` 已经是仓库里大多数项目管理资源的现有模式，并天然复用 `ProjectMiddleware` 与 `GetProjectID(c)`；相比 header，path 更可观测、更难遗漏，也更容易在测试里复现。
- **Alternative rejected – `X-Project-ID` header`**：前端已经尝试这么做，但后端完全没消费，说明它过于隐式，极易再次产生 drift。
- **Alternative rejected – ambient selected project`**：浏览器当前选中的项目不是可信后端合同，不能作为多标签页、重放请求、或服务端校验的依据。

### 2. 项目入口摘要由服务端负责生成并返回
`/api/v1/projects` 与 `/api/v1/projects/:id` 需要返回项目入口面真正消费的摘要字段，包括但不限于项目生命周期状态、任务总数、agent 总数，以及后续 bootstrap / project card 需要的最小一致摘要。前端 store 只做 normalize，不再“脑补”这些字段。

- **Why this**：项目卡片、项目列表统计、bootstrap entry 都是运营判断入口，默认值会制造假完整性；服务端返回 authoritative summary 才能让各个入口共享同一真相。
- **Alternative rejected – frontend fallback defaults**：这只能掩盖接口不完整，不能证明系统真实状态。
- **Alternative rejected – separate summary endpoint just for cards**：会新增第二套项目入口合同，反而让 drift 更隐蔽。

### 3. workflow definition / template 操作必须做项目边界校验
所有会读取或变更 workflow definition/template 的 handler、service、repository 路径都必须验证目标记录与显式 project context 的关系。允许的情况只有：
- 记录属于当前项目；
- 记录是允许复用的全局 template 来源（system / marketplace）；
- 设计明确允许跨源复制，但结果必须落到当前项目。

除此之外，一律显式拒绝，而不是读取成功后再靠前端过滤或 silently 操作零值 project。

- **Why this**：当前最大的真实风险不是“查不到”，而是“误操作到别的项目或零 UUID 项目作用域”。
- **Alternative rejected – frontend-only filtering**：无法防止直接 API 调用或 store 漏传上下文。
- **Alternative rejected – repo 继续按 `id` 更新删除**：会让 handler/service 失去最后一道 ownership guard。

### 4. dashboard bootstrap 与目标工作区共用同一套 project-scoped 统计口径
bootstrap summary 的 playbooks / planning readiness 必须使用与目标工作区相同的 project-scoped 查询合同；尤其 workflow template readiness 不能继续依赖 unscoped 请求。dashboard store 可以继续做聚合，但聚合输入必须来自同一作用域真相。

- **Why this**：bootstrap 的价值是把用户送到正确下一步；如果 readiness 与实际 workspace 不一致，handoff 反而会误导用户。
- **Alternative rejected – dashboard 单独维护一套近似统计**：会继续制造第二套“看起来像项目真相”的影子合同。

## Risks / Trade-offs

- **[Risk] 路由合同调整会影响现有 workflow store 与测试** → **Mitigation**：先定义 canonical contract，再在实现中提供小范围兼容层或同步改造调用方，确保受影响面集中在 workflow/template seam。
- **[Risk] 项目摘要字段上移到服务端后，需要新增聚合查询或统计成本** → **Mitigation**：只返回当前 UI 已真实消费的最小摘要字段，避免顺手做成大而全的 analytics API。
- **[Risk] 边界校验收紧后，可能暴露出当前前端传参不完整的问题** → **Mitigation**：把“缺少 project context 时显式失败”当作本 change 的验收项，而不是回退到隐式默认行为。
- **[Risk] bootstrap 与 workflow template 口径统一后，部分项目可能从“ready”变成“attention”** → **Mitigation**：接受这类真相回归；修正错误 ready 比继续展示假完整更重要。

## Migration Plan

1. 定义新的项目管理 API 合同与 spec delta，明确哪些项目管理接口必须绑定显式 project context。
2. 实现服务端项目摘要返回与 workflow/template ownership guard，同时保留必要的短期兼容入口以降低切换风险。
3. 更新前端 store 与 dashboard 聚合输入，统一消费 canonical project-scoped contract。
4. 用 focused tests 验证：
   - `/projects` 列表与详情返回真实摘要；
   - workflow template / definition 请求缺少或错配 project context 时显式失败；
   - bootstrap readiness 与 workflow/docs/sprint 工作区真相一致。
5. 如果发布后需要回滚，优先回退前端调用方到旧入口，并暂时保留服务端 guard 日志；不要回退 ownership 校验到“按 id 任意操作”的状态。

## Open Questions

- 是否需要在本 change 内完全删除旧的 id-only workflow definition 入口，还是先保留受控兼容层一轮再归并？当前设计倾向“先校验后兼容”，实现时可按调用面大小决定。
- `status` 的服务端项目摘要枚举是否只保留当前前端已消费的 `active/paused/archived`，还是基于 bootstrap readiness 派生更细粒度状态？本 change 先以不扩大产品语义为原则，优先补齐当前 UI 已经展示的字段。