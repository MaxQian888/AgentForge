## Context

AgentForge 的产品文档已经把 Role Plugin 定义为 MVP 核心能力：角色以 YAML 形式存在，目录建议为 `roles/{role_id}/role.yaml`，结构覆盖 `apiVersion`、`kind`、`metadata`、`identity`、`capabilities`、`knowledge`、`security`、`extends` 和 `overrides` 等字段，同时 Go 端是角色配置的真相源，运行时再把角色投影给 TS Bridge。当前仓库的真实实现还停留在更早的扁平模型上：`src-go/internal/model/role.go` 只支持少量旧字段；`src-go/internal/handler/role_handler.go` 直接读写根目录 `roles/*.yaml` 并内嵌预设角色；`src-go/internal/role/registry.go` 虽然存在，但没有成为 API 和执行链路的统一入口；`src-bridge` 侧消费的 `role_config` 也仍是一个简化平面对象。

这意味着仓库现在同时存在三套“角色真相”：PRD 里的目标 schema、Go 侧的旧模型、以及 Bridge 侧的简化执行配置。如果不先把这些边界收敛，后续无论是角色文件扩展、Agent 绑定、还是插件化演进，都会继续堆积兼容负担。

## Goals / Non-Goals

**Goals:**
- 让 Go 侧支持按 PRD 约定发现和加载 Role YAML，包括 `roles/{role_id}/role.yaml` 的目录化结构与最小兼容的旧文件结构。
- 建立统一的角色注册与解析链路，替换当前 handler 本地文件扫描和硬编码预设角色的分叉实现。
- 定义 PRD Role YAML 到运行时执行配置的归一化和投影规则，明确哪些字段服务于 API 展示，哪些字段服务于 Agent 执行。
- 为当前 Agent 启动链路补一个最小的 `role_id` 绑定入口，使角色执行投影不只停留在 registry 能力，而是真正进入 `spawn -> bridge execute` 的运行时路径。
- 支持基础的角色继承与覆盖合并语义，尤其确保安全约束采用更严格的合并策略。
- 让角色 API、样例 YAML 和测试覆盖都围绕同一份角色真相工作。

**Non-Goals:**
- 不在本次中交付角色可视化编辑器、角色市场或数据库持久化角色仓库。
- 不要求前端在本次内完成完整的角色创建/选择 UI。
- 不把 TS Bridge 变成 Role YAML 的直接读取方；YAML 仍由 Go 解析，Bridge 只消费归一化后的执行配置。
- 不在本次中引入工作流插件、审查插件或其他非 Role 插件类型的统一运行时改造。

## Decisions

### 1. 采用“PRD Schema + Go 归一化 + Bridge 执行投影”的双层模型

Role YAML 会先在 Go 侧按 PRD 结构解析成完整领域模型，再从中投影出一份给运行时使用的扁平执行配置。这样可以把“配置资产的表达能力”和“执行面最小合同”分开：

- 完整角色模型保留 `metadata`、`identity`、`capabilities`、`knowledge`、`security`、`collaboration`、`triggers` 等更丰富的 PRD 字段。
- 执行投影只保留当前 Agent/Bridge 真正消费的字段，例如角色名称、系统提示、允许工具、预算、最大轮次、权限模式和并发限制。
- Go 是 YAML 的唯一读取和归一化入口，Bridge 不负责理解 `extends`、目录布局或 schema 兼容。

备选方案：
- 让 TS Bridge 直接读取角色 YAML。优点是减少一次投影；缺点是会让 Go 与 TS 各自维护一套 YAML 解析和兼容逻辑，不符合 PRD 中“Go 是配置真相源”的约束。
- 只扩展当前扁平模型。优点是改动更小；缺点是无法真正承接 PRD 的角色能力，后续还会再做一次破坏式迁移。

### 2. 角色加载统一收口到可复用的 Role Registry/Store，而不是继续让 handler 自己扫文件

本次会把角色发现、解析、验证、归一化、缓存和查询抽到 `src-go/internal/role` 下的统一服务对象，由 API handler 和后续执行链路共同复用。该组件负责：

- 扫描 `ROLES_DIR` 下的目录化角色和兼容的旧 `.yaml/.yml` 文件。
- 解析 PRD schema、执行基础校验，并产出规范化角色对象。
- 暴露 list/get/save/update 等操作，供 `RoleHandler` 使用。
- 在需要时输出执行投影，而不是让调用方自己拼接字段。

备选方案：
- 保留 `RoleHandler` 直接读写文件，仅把 parser 扩一下。短期可行，但会继续保留 API 与执行链路之间的重复逻辑。

### 3. 规范化目录布局写入 `roles/{role_id}/role.yaml`，读取阶段兼容旧平铺文件

PRD 已经明确角色文件路径是 `roles/{role_id}/role.yaml`，因此新写入和规范化后的落盘路径以目录化结构为准。但为了避免仓库现有角色文件和临时资产一次性全部失效，读取阶段会兼容两种形态：

- 规范形态：`roles/<role-id>/role.yaml`
- 兼容形态：`roles/<name>.yaml` / `roles/<name>.yml`

当同一角色同时存在两种来源时，以规范目录化路径优先，并返回明确冲突/覆盖语义。API 的 create/update 默认写入规范路径，逐步把旧资产收敛过去。

备选方案：
- 直接强制迁移只支持目录化结构。优点是规则简单；缺点是会立刻打断现有角色文件和已有测试。
- 长期同时支持两种写入路径。优点是兼容成本低；缺点是会让角色真相长期分叉。

### 4. 继承与覆盖在 Go 侧解析期完成，安全配置按“更严格优先”合并

PRD 已经把 `extends` / `overrides` 和安全治理作为角色系统的一部分。本次会在 Go 侧完成继承解析，而不是把父子角色关系留到运行时再解释。合并原则：

- 元数据和身份字段采用“子级覆盖父级”的显式覆盖规则。
- 集合类字段根据语义做增量或替换；例如标签、知识源、能力包可合并去重。
- 安全相关字段采用更严格约束优先，例如更小的预算、更收敛的允许路径、更严格的权限模式和输出过滤。
- 若出现循环继承、父角色缺失或非法覆盖，角色加载失败并返回可定位错误。

备选方案：
- 暂时不做继承，只支持单文件角色。实现更快，但会与 PRD 和已有 schema 明显脱节，后续还需要再次改模型。

### 5. Bridge 合同继续保持执行配置最小面，但显式承认其来自角色投影

`src-bridge` 当前的 `role_config` 已经是一个扁平结构，适合当执行投影的承载体。本次不让 Bridge 直接吃完整 PRD Role YAML，而是：

- 明确 `role_config` 是 Go 从 Role YAML 归一化出的 execution profile。
- 扩展或收敛字段命名，使其和 Go 侧投影语义一致。
- 对缺失、冲突或超出当前运行时支持范围的字段，在 Go 侧完成裁剪或拒绝，而不是把复杂度推给 Bridge。

备选方案：
- 把完整 YAML 透传给 Bridge 并由其自行挑字段。这样会让 TS 侧不得不理解大量暂时不执行的角色语义，扩大耦合面。

### 6. 在 Agent spawn API 提供最小 role 引用入口，由 Go 完成解析与投影

为了让“角色 ↔ Agent 实例绑定”在 MVP 范围内真正可用，本次采用最小后端绑定面，而不是等待完整前端配置面板一起交付：

- `POST /api/v1/agents/spawn` 接受可选 `roleId`，仅表示“引用哪一个已存在角色”，不在该请求里重复提交 Role YAML 或手写 execution profile。
- `AgentService` 在启动执行前通过统一 `role` store 解析 `roleId`，构建 execution profile，并把投影后的 `role_config` 注入 Bridge `ExecuteRequest`。
- 角色投影同时作为运行时约束来源，优先影响 `max_turns`、`allowed_tools`、`permission_mode`，并对预算上限施加更严格约束。
- `agent_runs.role_id` 作为运行记录的一部分被持久化，便于 API 返回、审计以及后续前端展示。

这样既满足了 PRD 中“创建 Agent 时可绑定角色、TS 仅消费完整角色配置投影”的要求，又避免在本次里提前承诺完整的前端角色选择 UI。

## Risks / Trade-offs

- [PRD schema 范围较大，当前运行时只消费其中一小部分] -> Mitigation: 明确“完整角色模型”和“执行投影”两层边界，避免因为运行时暂未使用就丢失文档字段。
- [读取兼容旧结构会增加解析复杂度] -> Mitigation: 只在读取阶段兼容，写入统一收敛到目录化规范路径，并在日志/错误里提示旧路径。
- [继承合并规则如果定义不清，容易出现不可预期行为] -> Mitigation: 先对关键字段给出确定性规则，尤其安全字段采用更严格优先，并以测试夹具覆盖冲突场景。
- [Bridge 现有简化 role_config 与 Go 新模型之间可能产生字段漂移] -> Mitigation: 在 spec 中把 execution profile 定义成显式合同，并在 Go/TS 两侧补校验测试。
- [把硬编码预设角色切到 YAML 资产后，启动和测试会更依赖磁盘内容] -> Mitigation: 保留明确的内置角色来源约定，并在测试中使用临时目录和固定 fixture 控制输入。

## Migration Plan

1. 扩展 `src-go/internal/model/role.go` 与 `src-go/internal/role/*`，引入 PRD 对齐的完整角色模型、校验和执行投影逻辑。
2. 重构角色注册/存储入口，让 `RoleHandler` 与后续调用点都依赖统一的角色 registry/store。
3. 引入目录化路径支持和旧平铺 YAML 兼容读取，更新内置预设角色资产与样例文件。
4. 对齐 `src-go/internal/bridge/client.go` 与 `src-bridge/src/schemas.ts` 的角色执行配置合同。
5. 为 Agent 启动链路接入最小 `roleId` 引用能力，把解析后的 execution profile 真实传入 Bridge，并持久化运行记录中的 `role_id`。
6. 补充角色加载、继承、冲突、API、执行投影与 spawn 绑定的测试与文档。

回滚策略：

- 如果完整 PRD schema 迁移造成阻塞，可先保留统一 registry 和目录化兼容层，同时限制执行投影只覆盖当前运行时必需字段。
- 如果目录化迁移对现有角色资产影响过大，可临时保持读取兼容并延后自动迁移，但 API 新写入仍坚持规范路径。

## Open Questions

- PRD 中 `collaboration`、`triggers`、`memory` 等字段第一版是完整保存但暂不执行，还是需要为其中部分字段增加最小可观察行为；本设计默认前者。
