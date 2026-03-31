## Context

当前 src-im-bridge 的共享命令引擎已经不是空壳：main.go 会注册 task、agent、cost、review、sprint、help 和自然语言 fallback，review 子命令也已经补到 deep、approve、request-changes。与此同时，Go 后端还已经暴露了更多适合 IM 远程控制的稳定 API，例如 agents 的 get 或 pause 或 resume 或 kill、项目 queue 的 list 或 cancel、项目 memory 的 search 或 store，以及项目 members 与 teams 相关读接口。

真正的问题不是 IM Bridge 没能力，而是命令协议缺少一个与仓库真相同步的 canonical catalog。README 仍主要宣传早期命令，help 文本和 PRD 口径也与当前实现及后端能力不完全一致；新增命令如果继续散落在各个 handler 和文档里，漂移只会继续扩大。这个 change 需要横跨 src-im-bridge 的 command handlers、API client、help 文本、smoke fixtures 和文档，但不应该重新设计 control-plane、provider contract 或产品数据模型。

## Goals / Non-Goals

**Goals:**
- 建立一份 canonical operator command catalog，让 handler 注册、help 输出、usage 错误、README 与 runbook 都围绕同一套命令面工作。
- 在现有共享命令引擎上补齐高价值的 IM read 或 control flows，优先覆盖 task、agent、queue、team summary、memory 这几类后端已有稳定 API 的产品面。
- 保持所有受支持平台继续复用同一套 slash command 语义和 reply-target 传播，而不是为单个平台发明特殊命令分支。
- 通过兼容 alias 和 focused verification，让现有命令用户无感迁移到更完整的 canonical 口径。

**Non-Goals:**
- 不新增 IM 平台，也不改动 provider registry、control-plane、typed delivery 或 rich interaction 的基础契约。
- 不追求把 Dashboard 的所有产品面一股脑搬进 IM，例如完整 wiki 编辑、plugin 管理、scheduler 运维或表单设计。
- 不引入新的认证模型；仍复用现有 bridge API key 与 project scope。
- 不把自然语言 fallback 升级成新的执行架构；它只负责发现和引导，不取代显式 slash commands。

## Decisions

### 1. 用统一 command catalog 驱动注册、help 与文档，而不是继续维护分散文本

命令漂移的根因是当前实现把注册逻辑、usage 字符串、help 文本、README 和 smoke 示例分散维护。此次变更将引入一层统一的 command catalog 或等价元数据结构，由它描述命令族、子命令、alias、usage 和简短说明；handler 仍各自实现业务逻辑，但 help 与 fallback 提示不再手写独立命令列表。

这样做的原因是这次 change 的核心问题是命令真相失配，不是再多写几个 handler。如果没有统一 catalog，新增的 agent control、queue、memory 等命令很快又会和帮助文本脱节。

备选方案是继续保留静态 helpText，并靠 review 人工同步 README 和 tests。这个方案在当前仓库状态下已经证明不可靠，因此不采用。

### 2. 新增命令只覆盖 IM 友好的 read 或 control 面，不做全量 CRUD parity

这次只补那些已经有稳定 API、且适合 IM 短文本或轻交互的命令面。优先目标是：
- agent runtime 控制，例如 status、pause、resume、kill
- task 轻量 workflow 控制，例如状态流转
- queue 可见性与取消
- project team summary 或 member summary
- project memory search，以及一条轻量 note 或 record 写入路径

这类命令能显著提升 IM 入口对本体能力的覆盖，但不会把 scope 膨胀成在聊天窗口里重做整个 Dashboard。像完整 wiki 编辑、plugin 生命周期管理、scheduler 作业编辑之类能力虽然仓库已有 API，却不适合在这条 change 里一起塞进来。

备选方案是直接追求产品全量 parity。这样会把 specs 和 tasks 扩成多个独立子系统，不符合这次 focused seam 的目标，因此拒绝。

### 3. 继续通过 typed AgentForgeClient 封装现有 API，而不是新造 IM 专用 backend command bus

命令 handler 继续依赖 src-im-bridge/client/agentforge.go 中的 typed methods。新增命令时，优先补客户端封装，然后让 commands 层调用这些方法并做 IM 友好的结果整形。只有当现有 API 响应对 IM 明显不友好时，才考虑补极薄的 DTO 适配，而不是重新扩一套 /api/v1/im/command 子协议去聚合所有操作。

原因是后端 domain APIs 已经存在且具备测试覆盖，重新发明 IM 专用 command bus 只会复制一层业务语义，增加 drift 面。Bridge 本来就是 operator-facing adapter，适合做展示整形，但不适合重新拥有一套独立业务协议。

备选方案是把新增命令全部经由后端 HandleCommand 再二次分发。这个路径当前本身就只覆盖较小子集，而且会让 Bridge 和 Go 各自维护命令路由，不采用。

### 4. 用 canonical 名称加兼容 alias 演进命令口径

现有命令和文档之间已经有命名漂移，例如 PRD 更偏向 /agent status，而当前实现是 /agent list。此次设计会为这类情况定义 canonical 名称，并为旧入口保留 alias 或兼容 usage，确保现有用户不会因为 help 对齐而断掉。task workflow 也遵循相同原则，例如可将 move 与 transition 统一到一个 canonical 入口，同时接受兼容写法。

这样做可以同时满足文档要和产品真相一致与已有命令不要硬 breaking这两个目标。

备选方案是一次性替换旧命令名。这样虽然更干净，但会制造没有必要的使用中断，因此不采用。

### 5. fallback 只做发现和引导，命令执行仍以显式 slash command 为准

main.go 里的 mention fallback 目前通过 NLU classifier 走后端 intent 识别。这条链路继续保留，但它的职责是把自然语言请求映射到 canonical command catalog 或返回更准确的帮助提示，而不是悄悄扩展成另一套比 slash command 更强的执行入口。

这样可以避免slash command 的能力边界和自然语言意图边界再次漂移，也能让平台间行为更稳定。

备选方案是把更多 operator action 直接塞进自然语言 fallback。这个方向会让 specs 难以测试，也不利于多平台一致性，因此这次不做。

## Risks / Trade-offs

- [Scope creep from product completeness] → 将范围冻结为 task、agent、queue、team summary、memory 加文档与 help 对齐，明确排除 wiki 或 plugin 或 scheduler 等更大产品面。
- [IM 输出过长或难读] → 命令响应以摘要、状态、链接和下一步动作为主；长列表做截断，并在必要时返回过滤建议。
- [alias 与文档再次漂移] → 由统一 catalog 生成 help 和测试矩阵，并把 README 或 runbook 更新纳入同一批任务。
- [受保护 API 的失败语义在 IM 中不清楚] → 为新增 client wrappers 和 command handlers 补失败路径测试，统一把 404、409、权限不足等错误转换为可读的 IM 回复。
- [不同平台对新命令的 reply-target 细节处理不一致] → 不为新命令引入 provider-specific 逻辑，只沿用现有 shared engine 与 reply-target 传播，并通过跨平台 smoke fixture 复核真实入口。

## Migration Plan

1. 先引入统一 command catalog，并把现有 task、agent、review、sprint、cost、help 迁移到 catalog 驱动的注册与帮助输出，确保行为不变。
2. 分批补充 client methods 和 command handlers，优先落 agent control 与 task workflow，再落 queue、team summary、memory。
3. 在命令处理稳定后，同步更新 README、help、platform runbook 和 smoke fixtures，避免中途再次产生文档漂移。
4. 以 src-im-bridge 的单元测试和 focused cross-platform smoke 作为验收主线；因为命令保持兼容 alias，所以部署不需要数据迁移。
5. 如需回滚，可先移除新命令注册并保留旧命令 catalog；本次不涉及持久化 schema 迁移，因此回滚风险较低。

## Open Questions

- team summary 应该只覆盖项目成员 roster，还是同时暴露 active team runs 的 list 或 status；这需要结合现有 /projects/:pid/members 与 /teams/* 的真实响应可读性再定。
- memory 写入命令是否默认固化为 project-scoped operator note，还是要求用户显式指定 category；前者更适合 IM，后者更接近底层 API。
- task workflow 的 canonical 命令名应该定为 move 还是 transition；两者都可保留 alias，但需要在 help 与 docs 中选一个作为主口径。
