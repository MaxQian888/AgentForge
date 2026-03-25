## Why

AgentForge 已经开始依赖后台定时能力来驱动任务预警、worktree 清理、运行时健康恢复和成本校准，但当前仓库里的实现仍是零散的 `ticker`、启动时 sweep 和未接线的队列雏形，缺少统一注册、持久化、观测、手动触发与桌面模式调度策略。PRD 明确把任务调度列为 Go Orchestrator 的核心职责，这个缺口已经开始影响任务风险检测、运维可见性和后续插件/工作流扩展，因此需要先补齐一个真实可落地的 scheduler control plane。

## What Changes

- 新增统一的后台调度控制面，提供内建定时任务注册、启停、运行记录、失败状态、手动触发和下一次执行信息。
- 把现有零散后台 loop 收敛到统一调度器，首批纳入任务进度检测、僵尸 worktree 清理、Bridge 健康/恢复巡检与成本校准任务。
- 为 Web/Dashboard 提供可观察的定时任务 API 和管理界面，让操作者能查看任务状态、最近一次结果、失败原因并执行手动重跑。
- 在桌面/本地模式下引入基于 Bun 最新 `Bun.cron` 的 OS 级调度注册策略，用于在 Tauri/Bun sidecar 场景下保持跨平台定时能力，而不是继续依赖仅在进程存活期间有效的内存 loop。
- 为调度任务补充统一的实时事件、告警和审计记录，避免“任务默默停了但没有人知道”。

## Capabilities

### New Capabilities
- `background-scheduler-control-plane`: 统一管理系统内建定时任务的注册、执行、持久化、观测、手动触发，以及桌面模式下的 Bun.cron OS 级调度桥接。

### Modified Capabilities
- `task-progress-alerting`: 将任务进度 detector 从隐式 ticker 提升为受控的定时任务，并暴露运行状态、配置与失败恢复语义。
- `agent-worktree-lifecycle`: 将 worktree stale-state inspection 和 garbage collection 纳入受控定时任务体系，支持周期运行与人工补救触发。

## Impact

- Affected code: `src-go/cmd/server`, 新的 `src-go/internal/scheduler/**`, 任务进度/工作树/Agent 服务接线, 调度相关 repository/handler/routes, Web 管理页与 store, `src-bridge` 的 Bun 侧本地调度入口。
- APIs / events: 新增 scheduler 管理 API、运行历史与手动触发端点，以及对应 WebSocket/notification 事件。
- Data model: 需要定时任务定义、运行历史、最后一次健康状态与桌面模式本地注册元数据。
- Dependencies / runtime: Go 侧引入统一 scheduler 抽象；桌面/本地模式利用 Bun v1.3.11 的 `Bun.cron` 能力完成跨平台 OS-level cron 注册。
