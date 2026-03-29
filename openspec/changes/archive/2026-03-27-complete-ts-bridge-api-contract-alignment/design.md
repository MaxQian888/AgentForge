## Context

AgentForge 当前已经有一套可运行的 TS Bridge HTTP surface，但 repo 内部对它的描述并不一致。`src-go/internal/bridge/client.go` 稳定使用 `/bridge/*`；`src-bridge/src/server.ts` 同时暴露 `/bridge/*`、裸根路径和 `/ai/*` 兼容入口；`docs/PRD.md` 与若干设计文档仍混杂 `/api/*`、`/agent/*`、gRPC 契约示意与历史说明。结果是“Bridge 是统一 AI 出口”这件事在产品层成立，在接口层却没有单一真相源。

这次 change 的核心不是新增 provider 或 runtime，而是把现有 TS Bridge 能力收敛成一个可持续维护的接口契约：让 Go、Bridge、自测、文档和后续调用方围绕同一套 canonical contract 工作，同时保留必要的兼容别名，避免一次性破坏现有调用。

## Goals / Non-Goals

**Goals:**
- 明确 TS Bridge 的 canonical HTTP + WebSocket contract，并覆盖 agent runtime 与轻量 AI 调用。
- 规定兼容别名的定位与约束，避免它们继续被当成新的主接口传播。
- 让 Go client、Bridge route 注册、测试与项目文档在同一契约下对齐。
- 为后续实现阶段提供 focused verification 范围，确保接口收敛不是停留在文档层。

**Non-Goals:**
- 不重新设计 runtime/provider 能力矩阵。
- 不要求本次 change 立即删除所有兼容别名。
- 不把前端改成直接调用 TS Bridge 内部接口。
- 不修改已归档的 MCP/provider/runtime foundation scope，除非它们与接口收敛直接相关。

## Decisions

### Decision: `/bridge/*` 作为 canonical HTTP route family
- Rationale: 当前 Go client、主 spec 和大多数可执行路径已经稳定使用 `/bridge/*`。相比重新迁移到 `/api/*` 或裸根路径，收敛到现有运行真相的迁移成本最低，也最符合“先看仓库真相再定规范”的项目要求。
- Alternatives considered:
  - 以 `/api/*` 为 canonical：文档历史包袱重，但当前实现和调用方并未以它为主，改动面更大。
  - 以裸根路径为 canonical：实现里存在一部分裸根别名，但语义边界不如 `/bridge/*` 清晰，也更容易和 Go 主服务 API 混淆。

### Decision: 兼容别名保留，但降级为 compatibility-only
- Rationale: 当前桥接服务已经暴露多个兼容入口，直接删除会提高 apply 风险。更稳妥的做法是要求 alias 与 canonical route 共用同一 handler 和 schema，同时在 specs/docs 中把它们降级为平滑迁移路径。
- Alternatives considered:
  - 立即移除所有 alias：契约最干净，但风险高，且不适合当前脏工作树和多方调用并存的仓库状态。
  - 继续默认支持多套等价主接口：会继续放大文档和实现漂移，长期不可维护。

### Decision: 用新 capability 承载横跨 runtime 与 lightweight AI 的总契约
- Rationale: 现有 `agent-sdk-bridge-runtime` 更偏执行语义，`bridge-provider-support` 更偏 provider-aware 轻量 AI；接口总契约横跨两者。新增 `bridge-http-contract` 可以把 canonical route family、别名策略、文档/调用方义务统一落在一个能力里，再用 delta 去收敛现有 capability。
- Alternatives considered:
  - 只修改 `agent-sdk-bridge-runtime`：会把 lightweight AI 和文档治理约束硬塞进 runtime spec，边界不清。
  - 只做文档修订不建 spec：无法为 apply 阶段提供可测试约束，也不符合 spec-driven 工作流。

### Decision: 文档中保留历史协议说明，但必须显式标注“非当前 live contract”
- Rationale: PRD 和设计文档里保留 proto/gRPC 参考对理解演进有价值，但如果不明确标注，很容易被误读成当前实现。文档应把 HTTP+WS 和 `/bridge/*` 明确成 live contract，把其他描述降级为历史/参考。
- Alternatives considered:
  - 全量删掉历史说明：最简洁，但会损失上下文。
  - 保持现状：继续制造错误实现入口。

## Risks / Trade-offs

- [兼容路径长期滞留] → 在 spec 中明确 alias 是 compatibility-only，并把新文档、新测试、新调用方强制绑定 canonical contract。
- [文档修改面较大] → 只收敛与 TS Bridge live contract 直接相关的页面，不把整个 PRD 做无关重写。
- [实现阶段容易只改文档不改测试] → tasks 中显式要求 Go client、Bridge route tests、文档示例一起验证。
- [旧脚本或隐式调用点仍使用历史路径] → 先做 targeted search 与 focused smoke，必要时保留 alias 并补迁移说明。

## Migration Plan

1. 在 specs 中明确 canonical route family、alias policy 与 live contract 语义。
2. 在实现中统一 `src-bridge/src/server.ts` 的 route registration 注释、handler 复用与 route inventory。
3. 让 Go client 与桥接相关测试只以 canonical contract 为主要断言对象。
4. 收敛 PRD/设计文档中的 live 示例与接口说明，历史协议保留但明确降级。
5. 使用 focused verification 确认 canonical routes 正常、兼容 aliases 仍等价、文档不再传播旧入口。

## Open Questions

- 是否要在实现阶段增加一个 Bridge route inventory/metadata endpoint，帮助文档和测试自动校验 live contract。
- 哪些裸根 alias 仍然有真实外部消费者，需要在迁移说明中单独列出。
