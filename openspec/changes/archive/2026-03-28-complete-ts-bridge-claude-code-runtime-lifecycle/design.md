## Context

AgentForge 当前已经把 `claude_code` 放进 TS Bridge 的 runtime registry，并通过 `handleExecute -> streamClaudeRuntime` 走真实 Claude SDK 路径。但现状仍有三个关键断点：

- `buildClaudeQueryOptions(...)` 主要构造最小执行参数，Bridge 已解析的 runtime/provider/model 与部分 Claude-specific launch context 没有形成稳定的 adapter 输入合同。
- `SessionSnapshot` 只保存 task/session/request/status/spend 等浅层信息，pause、budget stop、runtime error 之后并没有持久化足够的 Claude continuity metadata。
- `/bridge/resume` 当前通过读取 snapshot.request 再次调用 `handleExecute(...)`，语义上更接近 replay，而不是恢复已有 Claude session continuity。

这次 change 只聚焦 `claude_code` 这条 runtime lifecycle，不复用 `complete-ts-bridge-api-surface` 那类更广的 Go API/前端操作面范围，也不重新打开多 runtime 产品配置问题。目标是让 TS Bridge 对 Claude Code 的连接语义本身变得完整、可恢复、可诊断。

## Goals / Non-Goals

**Goals:**
- 为 Claude-backed runtime 定义完整 launch context contract，确保执行真正使用 Bridge 已解析的 runtime/provider/model、权限模式、工具/MCP 配置和支持的 continuity inputs。
- 为 pause、budget stop、runtime failure、cancel 等终止路径持久化真实的 continuity snapshot，而不是只落浅层 request replay 数据。
- 让 `/bridge/resume` 对 `claude_code` 使用基于 continuity state 的恢复语义，并在状态不满足时返回显式错误。
- 让 status/snapshot/diagnostic surfaces 能稳定表达当前运行身份与是否具备 resume 前置条件，避免 Go 或 operator 继续猜测。
- 补齐 focused tests，覆盖 execute -> pause -> resume、budget stop -> snapshot、missing continuity -> explicit failure 等关键生命周期场景。

**Non-Goals:**
- 不扩展 Codex/OpenCode command runtime 的恢复协议。
- 不改动 Go control-plane、前端 dashboard、或 project settings catalog 的更广产品面。
- 不在本次引入新的凭据托管系统或跨进程会话数据库；continuity persistence 仍基于现有 Bridge session storage seam 演进。
- 不重写 Claude SDK 的事件模型，只在 Bridge adapter 内做规范化提取与持久化。

## Decisions

### 1. 把 Claude launch context 与 continuity state 分成两层持久化语义

`ExecuteRequest` 仍然是 Go -> Bridge 的启动输入真相，但 Bridge 需要把 Claude-specific runtime state 单独投影到 snapshot 中，而不是把 `request` 当成全部恢复上下文。设计上把 snapshot 扩展为两层：

- `request`: 原始且已归一化的启动参数，用于诊断、审计和非连续性兜底。
- `continuity`: Claude runtime 在执行过程中提取到的恢复元数据，例如 provider session handle、resume token、上次 checkpoint 时间、是否允许恢复、最近一次不能恢复的原因。

这样可以避免 pause/resume 继续误用“重放启动请求”冒充连续恢复，也能把 request-level 身份与 runtime-level continuity 分清楚。

备选方案：
- 继续只存 `request`，resume 时重跑。否决，因为这无法满足“保持 Claude 会话连续性”的产品语义。
- 把 continuity 元数据只放内存不持久化。否决，因为 pause、Bridge 重启、或后续诊断都会丢失恢复依据。

### 2. 为 Claude runtime 增加显式的 continuity extractor 和 launch builder

`claude-runtime.ts` 需要从当前“直接把 query options 丢给 SDK 然后翻译事件”升级为两个明确步骤：

- `buildClaudeLaunchContext(...)`: 从归一化 request 和 active plugin set 计算 Claude adapter 真正需要的 launch tuple，确保 model、权限模式、allowed tools、MCP server config、以及可选 continuity inputs 都由一个函数统一负责。
- `extractClaudeContinuity(...)`: 从 Claude SDK 的流式事件或终态结果里提取可恢复元数据，并持续更新到 runtime/snapshot。

这样可以让 launch 参数和 continuity 提取都变成可测试、可演进的独立 seam，避免逻辑继续散落在 event loop 与 route handler 里。

备选方案：
- 在 `streamClaudeRuntime` 里继续内联拼装和提取。否决，因为会让生命周期逻辑继续耦合、难以覆盖测试。

### 3. `/bridge/resume` 对 Claude runtime 改为“continuity-first”，不再默认 replay

当 snapshot 标记为 `runtime=claude_code` 时，`/bridge/resume` 必须先验证 continuity state 是否存在且可恢复：

- 若 continuity 完整，则通过 Claude adapter 的 resume path 恢复，并沿用既有 `session_id` / runtime identity 语义。
- 若 snapshot 只有 request 没有 continuity，返回显式 409/422 风格错误，说明当前 snapshot 不满足 Claude resume 前置条件。
- 只有 legacy 非-Claude path 或明确 compatibility mode 才允许 replay 式重新 execute。

这样能把“resume”一词重新绑定到真实恢复语义，并把缺口暴露成可以被修复的 contract failure，而不是静默伪成功。

备选方案：
- 保留当前 replay 语义但改文档措辞。否决，因为这会继续让上游误判功能已完整。

### 4. Continuity readiness 进入 status 和 snapshot 元数据，而不是只留在错误文本里

为了让 Go 或操作面在调用 resume 之前就能知道是否可恢复，Bridge status/snapshot metadata 需要补充最小 continuity 状态，例如：

- 当前 runtime 是否持有可恢复 continuity state。
- 最近一次 snapshot 的 continuity 更新时间。
- 若不可恢复，阻塞原因属于 missing_state、expired_state、runtime_mismatch 还是 provider rejection。

这里不要求前端立即消费这些字段，但合同必须先存在，避免后续任何调用方都只能先点 resume 再读错误。

备选方案：
- 只在 `/bridge/resume` 失败时返回错误。否决，因为 status 面无法做前置判断，也不利于诊断。

## Risks / Trade-offs

- [Risk] Claude SDK 事件里可提取的 continuity 元数据形态可能不稳定 -> Mitigation: 在 adapter 内封装 extractor，并让 snapshot 保存标准化字段而不是原始 provider payload。
- [Risk] `SessionSnapshot` 扩展后旧快照缺少新字段 -> Mitigation: 读取时保持向后兼容，对旧快照明确标记 `resume_ready=false`，而不是静默 replay。
- [Risk] resume 从 replay 改成 strict continuity-first 后，部分现有测试或手工流程会从“看似成功”变成显式失败 -> Mitigation: 同步更新测试与文档，把 failure 设计成清晰的 contract error。
- [Risk] status 元数据扩展可能影响现有调用方解析 -> Mitigation: 采用追加字段，不修改已有 runtime/provider/model 基本字段语义。

## Migration Plan

1. 先扩展类型与 snapshot schema，引入 continuity metadata 的标准化结构，并让现有读写路径对旧快照保持兼容。
2. 重构 Claude adapter，拆出 launch context builder 与 continuity extractor，在 execute/pause/terminal snapshot 路径持续写入 continuity。
3. 改造 `/bridge/resume` 和相关 execute helper，使 Claude runtime 走 continuity-first 恢复语义，并为缺失状态返回显式错误。
4. 扩展 status/snapshot payload 与 focused tests，验证 pause/resume、budget stop、runtime failure、旧快照兼容等场景。
5. 最后更新相关 OpenSpec spec 与开发文档，把 replay-based resume 标为 legacy/insufficient path。

回滚策略：
- 若 continuity extractor 不稳定，可先保留新的 snapshot shape 与显式错误语义，临时关闭实际 resume 恢复实现，但不回退到静默 replay。
- 若新增 status 字段影响上游，可先保留字段追加但延后消费，避免删除已发布合同。

## Open Questions

- Claude SDK 当前可稳定依赖的 continuity/resume 元数据具体字段名与来源，是否全部来自 query result，还是也需要读取中间事件。
- Claude resume 成功后是否必须沿用原 `session_id`，还是允许生成新 bridge session 但保留 continuity lineage；实现前需结合现有 Go 持久化假设确认。
- continuity readiness 是直接放进 `/bridge/status/:id` 主 payload，还是同时进入 snapshot/event data 但保持 status 为最小追加字段，需在实现前定稿。
