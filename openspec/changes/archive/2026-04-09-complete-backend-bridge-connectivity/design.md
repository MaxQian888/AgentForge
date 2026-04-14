## Context

AgentForge 当前已经具备三条真实的后端集成线：

- `src-go` 通过 `internal/bridge/client.go` 调用 `src-bridge` 的 canonical `/bridge/*` 合同，承接 agent execute、status、pause、resume、health、runtime catalog、以及 lightweight AI 能力。
- `src-go` 通过 `internal/service/im_control_plane.go`、`internal/service/im_service.go` 与 `src-im-bridge` 的注册、心跳、targeted delivery、reply-target、ack/replay 流程协作。
- `src-im-bridge` 通过 `client/agentforge.go` 与 Go backend 交互，再由 Go backend 代理需要 TS Bridge 的 runtime 或 AI 能力。

这些能力已经通过多条归档 change 分别落地，但 source of truth 仍然是碎片化的：Bridge runtime completeness、runtime catalog、IM control plane、IM action execution、AI proxy、team/runtime context 等合同分散在不同 spec 中。结果是当前仓库虽然已有多数基础实现，却没有一条统一设计明确回答下面几个关键问题：

- 哪个进程是 TS Bridge 与 IM Bridge 之间的唯一中枢？
- 哪些调用必须经由 Go proxy，哪些调用必须直接进入 backend workflow？
- runtime/provider/model/team/MCP/reply-target 等上下文需要在哪些 hop 中无损传播？
- 当 external runtime、Bridge upstream、或 IM Bridge 实例失联时，系统应该把断点暴露到哪一层？

这次 change 不打算重做架构，而是把现有真实拓扑提升为统一合同，并把“局部已完成、整体未封口”的后端连接 seam 收口成一组可以验证的实现任务。

## Goals / Non-Goals

**Goals:**

- 明确并锁定 Go backend 作为 TS Bridge、IM Bridge、external runtimes 之间的唯一后端编排中枢。
- 定义单 Agent、Team Agent、IM AI 命令、IM action、progress/terminal delivery、runtime diagnostics 等关键链路的 canonical hop 和 required context。
- 要求 Go↔Bridge、Go↔IM Bridge、IM Bridge↔Go proxy 的失败、回退、诊断与状态传播保持 truthful，而不是返回模糊的“连接失败”。
- 为后续实现提供 focused seam：优先修补现有 backend/bridge/im-bridge 调用缺口，而不是新增前端或平台功能。

**Non-Goals:**

- 不新增新的 IM 平台接入，也不扩展新的 UI/operator 面板。
- 不把架构改成 TS Bridge 直接调用 IM Bridge，或让 IM Bridge 绕过 Go backend 直连 TS Bridge。
- 不重新设计 runtime catalog、provider settings、或 team product workflow 的前端交互层；只处理后端连接完整性。

## Decisions

### Decision 1: Go backend remains the only backend orchestration hub

Go backend 继续作为唯一后端中枢，负责：

- 将 agent/runtime/lightweight AI 请求代理到 TS Bridge；
- 将 IM 实例注册、delivery 选择、bound progress、reply-target 恢复、ack/replay 统一纳入 control plane；
- 将 IM action、task/review/dispatch 等持久化业务工作流留在 backend service 层；
- 将 runtime 与 delivery 诊断面统一暴露给 IM/operator surfaces。

**Why this over direct TS Bridge → IM Bridge coupling?**

- TS Bridge 直接了解 IM Bridge 会重复引入 bridge instance discovery、delivery routing、reply-target persistence、签名校验与 retry/replay 逻辑。
- Go backend 已经拥有 task、team、run、review、subscriptions、reply bindings 等业务上下文，直接让 TS Bridge 访问 IM Bridge 会把业务边界拆散。
- 当前代码真实实现已经是 Go 作为中枢；这次 change 的价值是把它变成明确合同，而不是另起一套新拓扑。

### Decision 2: Define connectivity completeness by flows, not by modules

设计按五条端到端 flow 收口，而不是按仓库目录分别修补：

1. **Go → TS Bridge runtime flow**：execute / pause / resume / status / health / runtime catalog。
2. **IM Bridge → Go → TS Bridge AI flow**：decompose / generate / classify-intent / runtime diagnostics。
3. **IM Bridge → Go backend workflow flow**：task create / dispatch / review / save-as-doc 等 shared action。
4. **Go backend → IM Bridge control-plane flow**：register / heartbeat / targeted delivery / bound progress / terminal outcome / ack / replay。
5. **Diagnostics flow**：明确暴露到底是 upstream Bridge、runtime readiness、还是 IM instance availability 出现故障。

**Alternative considered:** 继续按 `src-go`、`src-bridge`、`src-im-bridge` 三个子树独立补全。  
**Rejected because:** 这种方式容易继续制造“每边都看起来合理，但中间 hop 丢上下文”的问题。

### Decision 3: Preserve execution context as a canonical envelope across hops

对 external runtime 调用链，Go backend 生成一份 canonical execution envelope，并要求以下字段在需要的 hop 中保持一致：

- `runtime`
- `provider`
- `model`
- team-related execution context（如存在）
- bridge/runtime diagnostics metadata
- `bridge_id`
- `reply_target`
- delivery identity / terminal delivery metadata（如存在）

这里的原则不是“每个接口都带全量字段”，而是“每条 flow 上必须保留它后续 hop 所需的最小完整上下文，且任何被丢弃的字段都必须是有意且可证明不需要的”。

**Alternative considered:** 让 downstream service 在 status/run/task 表里反推缺失上下文。  
**Rejected because:** 当前仓库已经证明这种做法会带来 runtime identity drift、team context drift、以及 reply-target 猜测。

### Decision 4: Capability routing from IM Bridge must be explicit and truthful

IM Bridge 的命令与 action 不再以“是否方便调用”为路由依据，而按能力类型分三类：

- **Bridge-proxied**：依赖 TS Bridge 的 AI/runtime diagnostics 能力，必须经由 Go proxy。
- **Backend-native**：task/review/dispatch/wiki 等业务工作流，直接调用 Go backend canonical API。
- **Backend-mediated delivery**：所有 progress、terminal outcome、structured outbound payload 都经由 Go control plane 去命中具体 Bridge instance。

当 capability 不可用时，响应必须说明是：

- Bridge upstream unavailable；
- external runtime not ready；
- IM Bridge instance unavailable / binding stale；
- backend workflow rejected。

不能把这些失败统一折叠成一个模糊错误。

### Decision 5: Verification will stay focused and cross-stack

这次 change 的验证不追求全仓一把梭，而是围绕后端连接 seam 做 focused proof：

- `src-go`：bridge client、agent service、im service、im control plane、action execution、routes/handlers。
- `src-bridge`：runtime registry、server routes、request/response schema、runtime identity and diagnostics tests。
- `src-im-bridge`：client calls、capability routing、control-plane startup/binding/delivery tests。

如果 repo-wide 仍存在历史噪音，任务层会把它们明确标记为 out of scope，不让“全仓不绿”掩盖本 change 的真实连接结果。

## Risks / Trade-offs

- **[Risk] cross-stack contract change 容易出现字段名或责任重复** → **Mitigation:** 先写 spec delta，要求所有 hop 使用 canonical surfaces，并用 focused tests 验证同一字段在上下游的一致性。
- **[Risk] 现有归档 spec 与代码真实行为存在漂移** → **Mitigation:** 以当前 live code seam 为准，必要时补充新的 requirement，而不是重述已经失真的旧描述。
- **[Risk] fallback 逻辑掩盖真实上游故障** → **Mitigation:** 要求 fallback 响应保留 failure source，并在 diagnostics surface 中可见。
- **[Risk] IM delivery 成功与 workflow 成功被混为一谈** → **Mitigation:** 将 action execution outcome 与 terminal delivery settlement 分开表达，避免“动作成功但消息没送到”被伪装成全成功。
- **[Risk] external runtime 差异过大，导致 contract 过宽或过假** → **Mitigation:** 统一要求 preserve runtime identity 和 diagnostics，但不强装功能对等；能力差异继续通过 truthful unsupported 暴露。

## Migration Plan

1. 为新 capability 与受影响 capability 写 delta specs，先把真实拓扑、flow、context、fallback、diagnostics 合同固定下来。
2. 在 `src-go` 补齐/收紧 canonical bridge client、runtime context propagation、IM control plane delivery/binding/result handling、AI proxy routing。
3. 在 `src-bridge` 补齐 runtime identity、diagnostics、status/resume 相关的缺口，并保持 canonical `/bridge/*` surface 不漂移。
4. 在 `src-im-bridge` 收紧 capability routing、Go proxy usage、reply-target / bridge binding 传播与 operator-facing failure messages。
5. 跑 focused verification，并在 change 内记录哪些 repo-wide failure 属于历史噪音、哪些属于本 change 回归。

部署上不需要单独的数据迁移；兼容性策略是保留现有 canonical API surface，不引入新的主要调用面。若中途发现实现回归，可逐模块回滚对应代码而保留 specs/change 留痕。

## Open Questions

- 是否需要把 MCP-specific diagnostics 也纳入本 change 的 mandatory operator output，还是先限定在 runtime / AI / IM control-plane connectivity？当前倾向是只要求它在 bridge diagnostics metadata 中保持可见，不扩成新的 MCP UI/flow。
- 对于非 Team 的单 Agent 路径，是否需要与 Team path 一样显式记录更多 execution lineage 元数据？当前倾向是仅要求保持 runtime/provider/model/reply-target 等与 connectivity 直接相关的字段。
