## Context

当前 `src-im-bridge` 已经具备三块关键基础设施：

- 平台侧已经有 capability matrix、reply target、native/structured/text delivery 选择逻辑；
- 后端已经有 action binding、control-plane register/heartbeat/ack、bound progress queue；
- provider coverage 也已经扩展到 `feishu`、`slack`、`dingtalk`、`telegram`、`discord`。

但这些基础能力之间仍然停着两个断层。

第一，`src-go/internal/service/im_service.go` 的 `/api/v1/im/action` 仍以 placeholder acknowledgement 为主。Bridge 已经能把 Slack/Discord/Telegram/Feishu/DingTalk 的交互统一规范化，但后端没有把这些共享 action 稳定映射成真实的任务分配、任务分解和 review 决策。

第二，控制面和异步进度回放仍然偏 text-first。`IMControlDelivery` 和 `QueueBoundProgress(...)` 目前主要以字符串 `content` 为载体，而 `src-im-bridge/cmd/bridge/control_plane.go` 也只把回放 payload 交给 `DeliverText(...)`。这意味着 direct notify 可以走 native/structured path，但 queued/replayed progress 与 terminal delivery 会丢失 rich payload、provider-native update preference 和 fallback metadata。

这次设计需要补齐的是“动作真实执行”和“rich payload 可靠投递”这两个跨 `src-go` 与 `src-im-bridge` 的闭环，而不是重新做 provider coverage、重新设计 command engine，或者改变单活 provider 的运行模型。

## Goals / Non-Goals

**Goals:**
- 让共享 IM action 在后端执行真实业务操作，而不是继续返回“已收到”的占位文本。
- 让 text、structured、provider-native payload 都能通过 control plane、compatibility HTTP 和 bound progress 路径一致地投递、回放与显式降级。
- 复用现有 capability matrix、reply target、binding 和 signature/ack 机制，而不是新增第二套通知或交互管道。
- 保持平台无关的 action contract 与 delivery contract，让 provider 差异只体现在 renderer、reply strategy 和 fallback choice。

**Non-Goals:**
- 不新增新的 IM provider，也不把 `wecom`、`line`、`qq`、`wechat` 一起纳入本次 change。
- 不重写现有 `/task`、`/agent`、`/review` 命令解析器。
- 不承诺一次性把所有平台都做成完全等价的 rich UI；平台差异仍允许显式降级。
- 不废弃 compatibility `/im/send` 与 `/im/notify`，只把它们收敛到同一套 canonical delivery semantics。

## Decisions

### 1. 引入 canonical typed outbound delivery envelope

后端需要把当前 text-only `IMControlDelivery` 扩展成 typed delivery envelope，让一个 delivery 能显式承载：

- 纯文本内容
- structured payload
- provider-native payload
- reply target
- delivery/fallback metadata
- 目标平台与 delivery kind

这样 control plane、compatibility HTTP 和 bound progress 都能围绕同一份 delivery contract 工作。

选择这个方案，是因为现状最大的断层不是“不会发 rich payload”，而是“只有 direct path 能发 rich payload，queue/replay path 会降回纯文本”。如果继续把 `content` 作为唯一权威字段，再在 `content` 里塞 JSON 或 provider-specific blob，后续回放、签名、fallback 诊断和测试都会变得脆弱且难以演进。

备选方案：

- 保持 control plane text-only，只在 direct notify 走 rich path。优点是改动小；缺点是异步进度、重连回放和 terminal delivery 永远做不到和 direct notify 一致。
- 为 control plane 和 compatibility HTTP 分别定义不同 payload。优点是迁移简单；缺点是同一通知语义会分裂成两套 contract，后续 provider 行为更难保持一致。

### 2. 将 `/api/v1/im/action` 从 placeholder handler 提升为 action execution seam

后端需要新增一个明确的 action execution seam，负责把共享 action 名称映射到真实业务操作并返回 canonical action result。这个 seam 需要覆盖至少当前 Bridge 已经暴露出来的动作：

- `assign-agent`
- `decompose`
- `approve`
- `request-changes`

它的职责不是解析平台 payload，而是消费已经被 Bridge 规范化好的 action envelope，再调用现有 task dispatch、decomposition、review 等 service。

选择集中式 action execution，而不是继续让 Bridge 直接调用多条业务 API，是因为 shared action 的核心价值就在于“平台无关”。如果把执行逻辑重新散落回 `src-im-bridge/commands/*` 或 provider adapter，平台间会重新产生行为漂移，reply-target 和 fallback metadata 也更难保持一致。

备选方案：

- 继续维持 `im_service.go` 里的 switch + 文本确认。优点是最省改动；缺点是用户会看到可点击按钮，但动作并不真正闭环。
- 让 Bridge 在 action 回调后自行调用 `assign/decompose/review` 等多个 API。优点是后端改动少；缺点是业务规则、授权与失败语义会重新散落到 Bridge，难以与 Web/API 主路径保持一致。

### 3. 让 progress/terminal delivery 与 direct notify 共用同一 delivery builder

`Notify(...)`、`Send(...)`、`QueueBoundProgress(...)` 以及 action completion reply 都应该走同一套 outbound delivery builder，而不是每条路径各自拼 payload。这样可以保证：

- 平台匹配规则一致
- fallback reason 一致
- rich/native/update preference 一致
- 回放和 direct path 的行为一致

这也意味着 `src-im-bridge` 侧的 `notify.Receiver` 与 `control_plane.go` 不能再分别维护“rich path”和“text replay path”，而是要统一到同一组 delivery resolution helper。

备选方案：

- 只让 `Notify(...)` 支持 rich envelope，progress/terminal 继续 text-only。优点是对既有队列最保守；缺点是最关键的异步场景仍然丢 rich parity。

### 4. 保留兼容 HTTP 入口，但把它降级为 canonical envelope translator

`/im/send` 与 `/im/notify` 仍然要保留，因为现在它们既是 fallback path，也是一些调试/运维路径的现实依赖。但它们不应该再拥有独立语义，而是应当被约束为：

- 接收 legacy fields 时，转换成 canonical typed envelope
- 接收 canonical typed fields 时，按同一 delivery strategy 执行
- 返回与 control plane 一致的 fallback/diagnostic metadata

这样 rollout 和 rollback 才能保持真实：当 control plane 不可用时，compatibility HTTP 只是 transport fallback，不是 semantics fallback。

备选方案：

- 立即移除 compatibility path。优点是 contract 更干净；缺点是会让现有 fallback/运维路径一次性中断，风险过高。

## Risks / Trade-offs

- [跨后端与 Bridge 的模型变更] → `src-go/internal/model/im.go`、client payload、receiver payload 和 control-plane payload 需要一起演进；通过 canonical envelope 和向后兼容字段过渡降低一次性断裂风险。
- [动作执行引入业务状态边界] → 某些 action 可能遇到已完成 review、不可分解任务、无法启动 Agent 等阻塞状态；通过 canonical action result 返回显式 blocked/failed outcome，而不是伪装为 success。
- [rich payload 回放让签名/ack 载荷更复杂] → 用统一的 canonical serialization 参与签名，避免 text/native/structured path 出现不同签名策略。
- [平台 rich parity 仍不完全一致] → 明确要求显式降级和 operator-visible fallback metadata，把“能力差异”变成可见 contract，而不是隐藏失败。
- [compatibility path 双栈期增加维护负担] → 通过共享 builder 和 translator 限制重复逻辑，只允许 transport 不同，不允许 delivery semantics 分叉。

## Migration Plan

1. 在 `src-go` 中定义 canonical outbound delivery envelope 与 action result model，并保持对现有 text-only compatibility payload 的读取兼容。
2. 让 `Notify(...)`、`Send(...)`、`QueueBoundProgress(...)` 与 control-plane queue 统一构造 typed delivery，而不是各自拼装字符串内容。
3. 让 `src-im-bridge` 的 control-plane consumer 与 notify receiver 共用 delivery resolution helper，确保 replay 和 direct notify 行为一致。
4. 引入 action execution seam，把 `/api/v1/im/action` 当前的 placeholder switch 替换成真实业务调用，并保留 reply target / metadata 透传。
5. 更新 `.env.example`、README 和 smoke/manual verification docs，补齐 action flow、rich replay 和 compatibility fallback 的验证路径。

回滚策略：

- 若 typed delivery 引入风险，可暂时让 builder 只填充 text 字段，但保留 envelope 结构不回退到字符串拼接。
- 若某个 provider 的 native replay 路径不稳定，可按 capability matrix 显式退回 structured 或 text，而不是关闭整个 control plane。
- 若 action execution 某条业务链存在阻塞，可保留 canonical failure/blocked outcome，不回退到“成功但未执行”的 placeholder 行为。

## Open Questions

- canonical envelope 是否需要在 v1 就允许同一 delivery 同时携带 text + structured + native，还是强制一条主 payload 加可选 fallback metadata 即可？
- review action 的 v1 闭环是否只覆盖 `approve` / `request-changes`，还是要顺带补 `comment` / `dismiss` 之类更细粒度交互？
- control-plane queue 的 typed payload 是否需要持久化到磁盘/数据库级别，还是当前内存态 pending queue + reconnect replay 已足够支撑本阶段实现？
