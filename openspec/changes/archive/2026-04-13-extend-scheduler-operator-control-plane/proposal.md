## Why

AgentForge 已经有一条真实可用的 scheduler backend：`src-go/internal/scheduler/*`、`/api/v1/scheduler/*`、`lib/stores/scheduler-store.ts` 与现有 `/scheduler` 页面已经支持 built-in job 的列表、启停、改 cron、查看 run history 与手动触发。但活跃 `enhance-frontend-panel` 中的 `scheduler-control-panel` 需求已经明显超过当前后端合同：它要求 pause/resume/cancel、历史清理、更丰富的指标与 upcoming schedule、以及更完整的 job 配置编辑，而当前后端只有 `UpdateJob(enabled/schedule)` 和 `TriggerManual` 这一层最小控制面。现在需要先把 scheduler 的 operator-facing backend 补齐，否则前端只能继续建立在隐式映射、缺失动作和过度乐观的状态语义上。

## What Changes

- 扩展现有 built-in scheduler control plane 的 operator 合同，补齐显式的 pause/resume 语义、运行中 job 的可取消能力、以及与 run lifecycle 对齐的状态字段，而不是继续把所有控制操作压缩成 `enabled`/`lastRunStatus` 两个最小字段。
- 为 scheduler API 增加 operator 需要的观测与管理能力：更丰富的聚合指标、按 job/status/time 的运行历史过滤、历史清理/保留策略、以及 upcoming schedule preview，确保面板能基于真实后端数据构建列表、详情和日历视图。
- 扩展 built-in job 的可编辑配置合同，让 Go backend 能按 job 类型暴露受控配置 schema / metadata / validation 结果，而不是让前端假设所有 job 都能被自由创建或任意编辑。
- 收紧前后端边界：保留 built-in catalog 作为 scheduler 的唯一事实源，不把当前系统误描述成“任意用户自定义 cron/job builder”，并为 unsupported action 返回 truthful diagnostics。
- 补齐 Go scheduler / handler / repository / WS 事件验证，以及当前 scheduler store/page 的 consumer contract 验证，避免 `/scheduler` 现有页面与后续 scheduler panel 再次漂移。

## Capabilities

### New Capabilities
- None.

### Modified Capabilities
- `background-scheduler-control-plane`: 将现有 built-in scheduler 控制面从基础 list/update/trigger 扩展为 operator-ready 的生命周期控制、历史治理、指标与 schedule preview 合同，同时保持 built-in catalog 而非任意用户创建 job 的边界。

## Impact

- Affected backend seams: `src-go/internal/scheduler/service.go`, `registry.go`, `repository/scheduled_job*.go`, `handler/scheduler_handler.go`, `internal/server/routes.go`, 以及相关 model / ws broadcaster / tests。
- Affected frontend consumer seams: `lib/stores/scheduler-store.ts`, `app/(dashboard)/scheduler/page.tsx`, `components/scheduler/*`，以及活跃 `enhance-frontend-panel` 中 `scheduler-control-panel` 的真实后端依赖。
- API impact: 扩展 `/api/v1/scheduler/jobs/*` 与 `/api/v1/scheduler/stats` 合同，可能新增 cancel / history cleanup / preview / config-metadata 等 operator endpoints；需要保持认证与现有错误响应语义一致。
- Runtime impact: built-in job handlers 需要支持 cancellation / operator state projection / richer run metrics，同时继续兼容 Go-owned scheduler registry 与 Bun scheduler adapter 的现有部署模式。
