## Why

AgentForge 的 IM Bridge 运行时、平台 provider、control-plane 和 rich delivery 这几条基础线已经基本落地，但 operator-facing 命令协议仍停留在早期基线：additional-im-platform-support 还主要以 /task、/agent、/cost、/help、@AgentForge 为口径，而当前仓库真相已经包含 /review 深度子命令、/sprint、Agent runtime 控制端点、队列管理、Team API 和项目级 Memory API。结果是 README、帮助文本、runbook、spec 与实际后端能力持续漂移，IM 入口更像局部演示，而不是真正可远程操作 AgentForge 的协作面。

现在需要把 IM Bridge 的命令面补到和当前产品真相一致：既修正文档与 help 的失真，也把已经存在于本体中的高价值协作能力通过 IM 暴露出来，让用户可以在 IM 中查看、调度、暂停、恢复和追踪活跃工作，而不是频繁切回 Dashboard。

## What Changes

- 统一 AgentForge 的共享 IM 命令协议口径，对齐 docs/PRD.md、src-im-bridge/README.md、commands/help.go、platform runbook 和实际处理器，消除文档只写早期命令、代码已支持更多子命令的漂移。
- 扩展现有 /agent 与 /task 命令族，补齐本体已有但尚未暴露到 IM 的高价值控制能力，例如 Agent run 的 status、pause、resume、kill，以及任务状态流转等轻量 workflow 控制。
- 新增适合 IM 短文本和轻交互的 operator 命令族，优先覆盖已存在稳定 API 的本体能力，包括队列可见性/取消、项目 Team 或 Team run 摘要，以及项目级 Memory 检索与记录。
- 为帮助文本、usage 输出和自然语言 fallback 建立同一份 canonical command catalog，使 slash command、别名、错误提示和 @AgentForge 引导都围绕同一套命令面工作，而不是继续依赖零散 hard-code。
- 补齐围绕新命令面的客户端封装、Bridge handler 测试、跨平台 smoke fixture、README 与 runbook 验证矩阵，确保所有受支持平台复用同一套命令能力而不是只在单个平台上看起来可用。

## Capabilities

### New Capabilities
- im-operator-command-surface: 定义 AgentForge IM Bridge 的 canonical operator command catalog，覆盖命令族、别名、usage/discovery、自然语言 fallback 引导，以及命令与 task、agent、team、queue、memory 产品面的映射关系。

### Modified Capabilities
- additional-im-platform-support: 共享命令一致性要求需要从早期 /task、/agent、/cost、/help 基线扩展为新的 canonical operator command surface，并要求所有已支持平台对新增命令族继续保持统一归一化与 reply-target 语义。

## Impact

- Affected code: src-im-bridge/commands/*、src-im-bridge/client/agentforge.go、src-im-bridge/cmd/bridge/main.go、src-im-bridge/README.md、src-im-bridge/docs/platform-runbook.md、src-im-bridge/scripts/smoke/**，以及相关 Go tests。
- Affected backend surfaces: 复用现有 src-go API，如 /api/v1/agents/:id/*、/api/v1/projects/:pid/queue、/api/v1/projects/:pid/memory、/api/v1/projects/:pid/members、/api/v1/teams/*；如现有响应不够 IM 友好，可能补充薄适配 DTO 或只读聚合封装。
- Affected UX: IM 中的任务或 Agent 调度、运行中控制、队列观察、Team 协作摘要、Memory 检索与记录、帮助发现与自然语言引导。
- Affected verification: src-im-bridge 命令测试、跨平台 smoke fixtures、文档与 runbook 中的命令矩阵都需要同步更新。
- Breaking change: 不计划引入 breaking API；现有命令应保持兼容，必要时通过 alias 或兼容 usage 迁移到 canonical 口径。
