## Context

AgentForge 项目成员模型是目前最早期做好的表之一：`member` 包含 human 和 agent 两类，共用身份/展示字段。但几条约束性决定从未落到代码里：

1. 项目内"谁能做什么"完全没有分层，只靠认证中间件区分"登录/未登录"。
2. agent 发起的动作（dispatch、team run、workflow exec）只记录 agent 身份，没有绑定"谁触发的这次 agent 执行"，因此无法用人类权限约束 agent 动作。
3. `projectH.Create` 不会把创建者自动登记为 project owner，导致创建完后仍需手动挂成员（这部分目前靠外部流程兜底）。

现在加 RBAC 不只是"给 API 前加 middleware"——它要求重设三条分界：

- member 数据模型里人类的 project role 与 agent 自己的 agent role manifest（`roleId`）必须分离，不能复用同一字段。
- agent 动作的发起点必须解析 initiator human，并把 initiator 的 projectRole 带到校验点。
- 前端不再作为唯一 gate；后端是**authoritative**，前端只做可见性优化。

这是跨 model、middleware、service、store、UI 的 cross-cutting 改造，必须先把 decisions 锁死再动代码。

## Goals / Non-Goals

**Goals:**
- 建立 `projectRole` 四级分类与 action→roles 矩阵，作为项目访问控制的唯一真相。
- 保证 agent 动作在发起入口处用 initiator human 的 projectRole 校验，使 agent 不成为权限绕过通道。
- 项目创建者自动登记为 owner；末位 owner 不可被降级或移除，防止项目 orphan。
- 前后端同时 gate 写操作，但以后端为 authoritative。
- 迁移：现有 member 回填 `editor`，除了项目创建人（如记录中可推断）回填 `owner`；不可推断时标记一条迁移遗留条目。

**Non-Goals:**
- 不做自定义角色（custom roles / permission packs）。四级硬编码。
- 不做跨项目或组织级 RBAC（org-level admin）。本 change 只到项目边界。
- 不引入正式的 invitation 流（按 Wave 2 `add-member-invitation-flow` 单独做）；现阶段 `member` 直接创建即生效，但创建请求必须显式指定 `projectRole`。
- 不引入资源级 ACL（比如单个 wiki page 对 viewer 额外开放）；以后可加，但不在本 change。
- 不改 agent `roleId`（角色清单）的语义，只和人类 `projectRole` 区分命名。

## Decisions

### 1. 四级角色 `owner | admin | editor | viewer` 硬编码

- `owner`：包含 `admin` 全部能力 + 删除项目、修改其他 owner 的角色、转让/新增 owner。
- `admin`：管理 members（不能改 owner 的 role）、修改 project settings、配置 automation/dashboard 模板；不能删除项目。
- `editor`：创建/更新/删除项目下的任务、文档、触发 dispatch/team run/workflow execute；不能改成员、不能改 settings。
- `viewer`：只读。不能触发任何会产生写入或费用（cost）的动作。

**Why this**：四级是多数协作产品（GitHub、GitLab、Notion）的最小可用角色集，也是团队内部最早产生分层需求的维度。更细的层级（reviewer、planner 之类）可以在未来通过"角色能力扩展"加，不影响基础四级。

**Alternative rejected – 自定义角色与权限矩阵**：初期复杂度爆炸、测试路径发散、易与 agent `roleId` 混淆；不在内测期阶段引入。

**Alternative rejected – 三级（admin/editor/viewer）**：没有 `owner` 就无法表达"项目归谁"，以及"哪些操作只有归属人能做（删除/转让）"。

### 2. action→roles 矩阵集中声明、中间件读

后端集中在 `src-go/internal/middleware/rbac.go` 声明一张 `map[ActionID]MinRole` 表，中间件按路由或显式 action ID 读矩阵；service 层可选复用同一矩阵做二次校验（例如 agent 动作入口）。action ID 形如 `project.settings.update`、`task.dispatch`、`team.run.start`。

**Why this**：矩阵集中可读、可被测试单独覆盖、能和 audit log 的 action 枚举对齐（Wave 1 第二个 change 会复用同一套 action ID）。

**Alternative rejected – 每个 handler 自己 `if role < admin`**：分散、易漏、无法静态审计；也和 audit log 会重复定义动作名。

### 3. agent 动作的 RBAC 在 dispatch / run-start 入口执行，参数里带 `initiatorUserID`

task dispatch、team run start/retry、workflow execute、automation trigger（非 scheduler 自动触发的人工入口）这些路径：

- handler 必须解析当前认证 user，把 `initiatorUserID` 传入 service。
- service 拿 `initiatorUserID + projectID` 解析 `projectRole`，用矩阵校验对应 action。
- 不能用 agent 自身的 `roleId` 替代——agent 没有"项目访问权限"这层概念。
- scheduler/IM webhook/automation 自动触发路径必须显式标记为 `systemInitiated=true` 或传入 initiator，如果是自动路径，则继承**上次人工配置该 automation 时的人的权限快照**，避免自动化被用来绕过。

**Why this**：这是 RBAC 和 agent 体系结合的关键约束点。没有这条，viewer 可以点一下 automation rule 让 agent 代劳；有这条，automation 本身的配置权限已经在 admin 级。

**Alternative rejected – agent action 只校验 agent role**：agent role 描述的是"agent 能做什么任务"，不是"谁能叫这个 agent 做事"，两者语义不重合。

**Alternative rejected – scheduler 自动触发不做校验**：会留下"谁配置 automation 谁就能绕过"漏洞，需要 `initiator snapshot` 补。

### 4. 项目创建者自动 `owner`；末位 owner 保护

- `POST /projects` 的 handler 在项目写入成功后，于同事务内为 `currentUserID` 创建一条 `members` 记录，`type=human`，`projectRole=owner`。
- 若 `PATCH /projects/:pid/members/:mid` 试图把最后一个 `owner` 降级，或 `DELETE` 试图删除最后一个 `owner`，返回 `409 CONFLICT`，错误码 `last_owner_protected`。
- 不允许没有 owner 的项目存在。

**Why this**：没有这两条就会出现"项目创建后无人管理"和"所有 owner 被误降级导致项目 orphan"两类不可恢复状态。

**Alternative rejected – 允许 orphan，由系统管理员 impersonate**：内测期没有系统管理员视图；会留一个必须手动运维的坑。

### 5. `projectRole` 在 DTO/DB/UI 里名字固定；agent `roleId` 保持原名

- DB 列：`members.project_role VARCHAR(16) NOT NULL DEFAULT 'editor'`。
- DTO：`MemberDTO.projectRole`（camelCase，前端一致）。
- 前端 store：`member.projectRole`。
- agent manifest 绑定保持 `member.agentConfig.roleId`（或现有字段名），永远不复用 `projectRole` 这个名。

**Why this**：两个概念同名是长期 drift 源；硬区分避免任何歧义。现有 memory `Project RBAC scope` 已记录此约束。

### 6. 迁移：现有 member 回填 `editor`，无法追溯创建人的项目不自动补 owner

- 迁移脚本：`UPDATE members SET project_role='editor' WHERE project_role IS NULL`。
- 项目缺 owner 的记录：migration 生成一份 `migration_reports/2026-04-17-projects-without-owner.md`（文件级 artifact），交由运维手工补；**不自动**把"第一个 member"升为 owner，因为无法确认语义对。
- migration 后有 owner 的项目立即生效 RBAC；无 owner 项目在 API 层保持"任何 admin+ 不可写"的保护状态（只剩 editor/viewer，所有写入被拦），迫使运维尽快补 owner。

**Why this**：不自动补 owner，避免把错误的"创建人"推定为 owner；同时不放行写操作避免数据漂移。

## Risks / Trade-offs

- **[Risk] 测试面暴涨**：每个 action 的四级角色都要有拦截测试。→ **Mitigation**：矩阵集中化后，只需对矩阵本身写 table-driven 测试 + 对每种 action 类型做一条端到端拦截测试，不需要 N×4 用例全排。
- **[Risk] agent 动作的 initiator 解析遗漏某条路径**：如果有 agent 路径没接入 `initiatorUserID`，RBAC 绕过就出现。→ **Mitigation**：在 service 签名层做 `initiatorUserID string`（非空 required）+ `systemInitiated bool`，编译期强制；再加一个后端 test 穷举所有 agent 动作入口都传了 initiator。
- **[Risk] 前端 gate 与后端矩阵不一致会引发"UI 看起来能点但后端拒"或相反**：→ **Mitigation**：前端 gate 从后端返回的 `/auth/me/projects/:pid/permissions` 端点派生（返回当前用户在当前项目的 allowed actions），不本地硬编码矩阵。
- **[Risk] 末位 owner 保护可能阻塞合法的运维动作**：→ **Mitigation**：保护只阻止 `降级/删除`；`新增 owner` 不受限；运维可先加再删，不存在死锁。
- **[Risk] automation initiator snapshot 过期**：如果配置 automation 的人后来降级/离开项目，automation 还在以其旧权限运行。→ **Mitigation**：automation 激活时校验 snapshot 中的 user 仍 ≥ admin；若失效则 automation 自动 pause 并通知 owner。本 change 只定 spec，真正实现可在 Wave 2。

## Migration Plan

1. **Schema & migration 先行**：加 `project_role` 列，给所有现有成员填 `editor`，生成无 owner 项目报告。
2. **后端矩阵与中间件**：定义 action ID 枚举、矩阵、RBAC middleware、`initiatorUserID` 解析，接入 projectGroup。写矩阵单测。
3. **agent 动作入口**：逐个改造 dispatch/team run/workflow exec/automation 路径，service 签名加 initiator；覆盖"缺少 initiator 时编译失败"与"viewer initiator 被拒"测试。
4. **member API 改造**：`POST /projects/:pid/members` 必填 `projectRole`，`PATCH` 支持 role 变更并校验授权（admin 不可改 owner），末位 owner 保护落在 handler。
5. **项目创建 owner 自动登记**：`projectH.Create` 事务里插 owner member。
6. **前端 store/hook/UI**：`useProjectRole(projectID)` hook + 组件按 role gate。调用 `/auth/me/projects/:pid/permissions` 获取允许的 action 集合。
7. **文档/测试回归**：更新 API 参考，补端到端 RBAC 覆盖测试。
8. **回滚策略**：schema 迁移不可回滚（破坏性期允许）；如果发现严重逻辑问题，可临时把 RBAC middleware 设为"全 allow" 的 feature toggle 紧急放行，同时记录 incident；不保留长期 toggle。

## Open Questions

- automation 的 `initiatorUserSnapshot` 的 schema 细节（记录 role 值 vs 记录 userID 每次解析）是否在本 change 做？当前建议：本 change 只记录 userID，每次触发时动态解析；snapshot 冷冻留给 Wave 2 automation 增强。
- `viewer` 是否允许评论任务和文档？当前草案：viewer 可读不可写，包括评论；但 wiki comments 是很多协作场景的"外部利益相关者反馈"来源，也许需要 `commenter` 第 5 级。开放等多人反馈。
- `owner` 在降级自己时的交互：是否提供一步完成的"转让 owner 并降级自己"原子动作？当前设计允许分两步（先加别人为 owner，再自降），但 UX 上可能有改进空间。
