## Context

AgentForge 的 Go dispatch control-plane 已经完成了任务分配、手动 spawn、队列 admission、queue promotion 和基础 budget guardrail 的主链路，但当前 operator-visible truth 仍然断在几处：`DispatchAttempt` 只保留了结果级 outcome，`AgentPoolQueueEntry` 也只保留了 entry reason，promotion recheck 的 recoverable-vs-terminal 语义没有稳定沉淀成可消费字段，`src-im-bridge` 客户端侧的 dispatch DTO 还会进一步削薄 queue 和 guardrail 信息。结果是系统内部其实知道“为什么没有启动”和“当前还能不能恢复”，但 API、queue roster、dispatch history 和 IM reply 没有统一把这些事实讲清楚。

这条 change 的范围要保持窄：只补 Go 派发器及其紧耦合 consumer contract，不重开 TS Bridge upstream capability 对齐，也不扩成新的 dashboard 产品面。真正要补的是 dispatch truth 的厚度，而不是再造一套新的派发拓扑。

## Goals / Non-Goals

**Goals:**
- 让 dispatch history 成为可诊断的事实来源，而不是只保留 outcome/reason 的薄记录。
- 让 queue roster 与 promotion recheck 显式表达 latest guardrail verdict，并区分仍可恢复的 queued 状态与终态失败状态。
- 让 assignment dispatch、manual spawn、queued promotion 在 started / queued / blocked / skipped 分支上输出同一套 canonical metadata。
- 让 `src-im-bridge` 客户端和 IM-facing formatter 不再丢失 queue、guardrail 和 budget metadata。
- 通过 additive 方式扩展现有 DTO 与 handler，避免破坏当前调用拓扑与已有入口。

**Non-Goals:**
- 不把 Go runtime proxy 扩展到 TS Bridge 新增的 shell/thinking/mcp-status 全部路由。
- 不重做前端 dispatch 页面或新增新的 operator UI。
- 不引入新的 runtime、provider 或新的 admission 拓扑。
- 不把整个 dispatch decision engine 重写成另一套服务框架；本次只收紧现有 seam 的 canonical contract。

## Decisions

### Decision 1: DispatchAttempt 需要升级为 verdict-bearing history record

`DispatchAttempt` 不再只承担“记一条 outcome”的作用，而要成为 dispatch history 的权威诊断记录。设计上会为 attempt 增补能复原当次 dispatch 决策的关键字段，例如 resolved runtime tuple、queue reference / priority、以及 preflight / admission 摘要中真正需要跨面消费的 machine-readable metadata。

这样做的原因是：当前 operator 看到 blocked 或 queued，只能反推 reason；而一旦 runtime/provider/queue/promotion trace 丢掉，后续就无法确认是同一条 admission 语义，还是另一路 consumer 自己拼出来的结论。

**Alternative considered:** 继续把 richer metadata 留在 queue entry 或内存态 websocket payload，dispatch history 仍然只保留 outcome/reason。  
**Rejected because:** 这样无法支撑 task-scoped history、postmortem 诊断和跨 consumer 对齐，archive 后 spec 也会继续要求 history surface 却没有真实数据源。

### Decision 2: Queue lifecycle 使用显式 verdict，而不是继续依赖 reason string heuristics

队列状态除了 `queued` / `promoted` / `failed` / `cancelled` 之外，还需要显式保留 latest guardrail verdict，至少要支持：
- 仍在排队，等待可恢复条件消失；
- promotion recheck 因预算或临时基础设施条件被重新挂回 queued；
- promotion recheck 因 task/member/runtime 上下文失效而终态失败。

这里不要求引入全新数据库表，但要求 queue roster 和 promotion lifecycle DTO 能表达这些语义，而不是让 consumer 继续解析 `reason` 文案猜测。

**Alternative considered:** 维持当前 queue status，不新增 verdict 字段，只在 formatter 里按关键词区分 recoverable 与 terminal。  
**Rejected because:** 这会继续把 operator truth 建立在字符串启发式上，和当前要修的核心问题完全相反。

### Decision 3: Assignment / manual spawn / promotion 共享同一套 outward dispatch metadata

三条入口内部仍可复用现有 service seam，但对外 contract 必须统一：
- `TaskDispatchResponse`
- dispatch history DTO
- queue roster DTO
- IM action response 中嵌入的 dispatch DTO

这些 surface 至少要共享同样的 queue / guardrail / budget / promotion truth。这样 Web、IM 和 operator surface 才不会出现“同一条 dispatch，在一个入口里看得到 queue priority，在另一个入口里只剩一句 agent pool is at capacity”的漂移。

**Alternative considered:** 只在 Go HTTP DTO 扩字段，IM client 暂时保持简化结构，后面再补。  
**Rejected because:** 这会把当前 change 变成半完成状态；而 proposal 明确把 IM-facing dispatch truth 也列为紧耦合 consumer contract。

### Decision 4: API evolution 采用 additive migration，旧调用方先兼容再切换

已有 response shape 不做破坏式重命名；新增字段采用 additive 方式落到：
- dispatch response
- dispatch history
- queue roster
- IM client structs

旧 consumer 即使短期不读取新字段，也不会立刻坏掉；但新 spec 会要求 canonical consumer 改用 machine-readable fields，而不是继续依赖 free-form `reason`。

**Alternative considered:** 直接替换现有 DTO 并移除简化字段。  
**Rejected because:** 当前 Go、IM、前端仍有多处消费老字段，强切会把 focused change 扩成跨栈迁移。

## Risks / Trade-offs

- **[Risk] DTO 字段一旦扩展过多，会把 dispatch history 变成快照垃圾堆** → **Mitigation:** 只保留对 operator / IM / queue 复盘真正有用的 canonical metadata，不直接塞入整份 runtime 或 pool 原始对象。
- **[Risk] queue verdict 语义和数据库现有 status 语义打架** → **Mitigation:** 保持 status 作为生命周期主状态，verdict 作为最新 guardrail / promotion 结果补充字段，不用 verdict 取代 status。
- **[Risk] IM client 与 Go DTO 不同步，导致新的 truth 又在 client 层丢失** → **Mitigation:** 在同一条 change 内同步更新 `src-im-bridge/client/agentforge.go` 的 dispatch DTO 和 formatter contract。
- **[Risk] manual spawn、assignment、promotion 的 outward metadata 统一后暴露出已有内部差异** → **Mitigation:** 以 outward contract 一致为第一目标，必要时在 service 层补一层 shared helper 来统一 shaping，而不是立即重写整个 decision engine。

## Migration Plan

1. 先更新 OpenSpec delta specs，锁定 dispatch history、queue verdict 和 IM-facing dispatch truth 的行为要求。
2. 在 `src-go` model / repository / service / handler 层扩展 attempt 与 queue DTO，并让 assignment / manual spawn / promotion 的 outward shaping 对齐。
3. 在 `src-im-bridge` 客户端与 formatter 层镜像新的 dispatch contract，避免字段在消费层被削薄。
4. 跑 focused `src-go` 和 `src-im-bridge` 验证；若中途发现不兼容，可先保留旧字段并局部回滚新字段消费，而不必回滚整个 dispatch lifecycle。

## Open Questions

- queue verdict 最终是以单一 `latestVerdict` 字段表达，还是拆成 `guardrailType/guardrailScope/recoveryDisposition` 更利于 consumer 使用？当前倾向是后者，避免把多重语义继续塞进一个字符串。
- dispatch history 是否需要直接持久化 preflight snapshot 的完整数值，还是只保留 operator 真正需要的摘要字段？当前倾向是只保留摘要，避免 attempt 记录膨胀。
