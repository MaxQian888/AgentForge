## Context

当前 AgentForge 的 IM Bridge 已经完成了三件关键基础工作：

- `src-im-bridge` 各平台 adapter、`replyTarget` 序列化、以及 `core/delivery` / `core/reply_strategy` 的共享 typed delivery 主干已经存在。
- `src-go/internal/service/im_control_plane.go` 与 `/ws/im-bridge` 已经把 registration、heartbeat、queueing、ack、replay、以及 bound progress delivery 打通。
- 中国平台的 readiness tier 与 truthful fallback 已经落到 README、runbook、health metadata、以及 focused tests 中，但 runtime 真相仍不对称：Feishu 具备 callback-token + delayed native update；DingTalk、WeCom、QQ Bot 主要还是 send-or-reply 加 fallback；QQ 仍然是 text-first。

这意味着当前问题已经不是“有没有 provider”，而是“同一条长任务 progress / terminal update / action completion 链路，在中国平台上到底能不能优先兑现 provider-native reply/update path”。如果不继续补这一层，control-plane、`/api/v1/im/action`、以及 replay recovery 仍会在最关键的异步回写阶段退回 generic text send，导致 capability matrix 与真实 runtime 行为继续脱节。

## Goals / Non-Goals

**Goals:**
- 让 DingTalk、WeCom、QQ Bot 在现有 provider seam 上尽量补齐 provider-owned reply/update lifecycle，而不是长期停留在“native send only + generic fallback”。
- 让 QQ 的 text-first 边界保持真实，但把 progress / terminal update / replay / action completion 的回退路径收紧成统一、可解释、可测试的合同。
- 让 control-plane replay、bound progress delivery、以及 `/api/v1/im/action` completion 复用同一条 provider-aware rendering / update decision path。
- 让 capability matrix、health / registration metadata、以及 fallback reason 与新的 reply/update truth 一致。
- 把验证面收紧到中国平台 package tests、reply/update contract tests、以及 control-plane replay / progress focused verification，形成可复用 gate。

**Non-Goals:**
- 不新增新的 IM 平台，也不重做现有 `IM_PLATFORM` / transport / credential contract。
- 不追求所有中国平台都伪装成 Feishu 式 mutable-card parity；平台官方不支持的能力保持 explicit fallback 或 blocked truth。
- 不在本 change 中重做 `/im` operator console 或前端状态面；那条线继续留在现有 `enhance-frontend-panel` seam。
- 不顺手清理 email provider、非中国平台 delivery 语义、或其他与本链路无关的测试噪音。

## Decisions

### Decision 1: provider-owned update path 继续留在各平台 adapter，而不是在 shared core 里抽象一层伪统一 mutable updater

做法：
- 继续把 reply/update 行为压在 `src-im-bridge/platform/dingtalk`, `platform/wecom`, `platform/qq`, `platform/qqbot` 的 live/stub seam 中。
- shared core 只负责根据 capability matrix、reply target、delivery envelope、以及 action outcome 选择“优先 update / reply / follow-up / text fallback”的策略，不直接 hardcode 平台 API 细节。

原因：
- 现在唯一真正具备独立 `UpdateNative(...)` 的是 Feishu；其他中国平台即便能补更多 lifecycle，也不等价于同一种 API 语义。
- 如果在 core 层发明一套“统一 mutable update”抽象，会很快再次把 QQ / QQ Bot / WeCom / DingTalk 的差异抹平，回到文档比实现更乐观的问题。

备选方案：
- 在 `core/reply_strategy.go` 中新增强约束的通用 updater 接口，要求各 provider 都实现。放弃，因为这会鼓励假 parity，并扩大改动面。

### Decision 2: progress streaming、action completion、compat notify、replay recovery 必须走同一条 provider-aware delivery decision path

做法：
- 继续以 typed envelope 为 canonical contract。
- bound progress、terminal update、`/api/v1/im/action` completion、以及 replayed delivery 在进入 transport 之前，都必须先通过同一套 rendering / reply / update 决策，而不是分别各自 fallback。
- fallback reason、delivery method、以及 action status 沿这条路径统一写回 delivery metadata。

原因：
- 当前 repo 已经有共享 delivery 主干；真正缺的是让所有异步回写入口都用同一条主干，而不是在 handler / service 层额外短路。
- 如果 replay 和 direct completion 不复用同一路径，最容易再次出现“实时链路能 native reply，但重放后只剩 text send”的漂移。

备选方案：
- 保留当前分散的 progress / action completion special-case，逐个 provider 修补。放弃，因为后续 archive 后的 spec truth 很难长期一致。

### Decision 3: QQ 继续保持 text-first，不把“reply reuse”误升级为 richer update capability

做法：
- QQ 只补齐 text-first progress / terminal update / replay semantics、reply-target restoration、以及 downgrade metadata；不额外引入并不存在的 native payload 或 mutable update 宣称。
- `qq-im-platform-support` 只收紧“如何 truthfully 回写”，不追求新的 rich surface。

原因：
- 当前 `platform/qq/live.go` 没有 native callback 或 native update 能力；它真正需要的是更稳定、明确、可测试的 text-first async lifecycle，而不是被包装成 richer parity。

备选方案：
- 通过伪造 card-like text 或额外 metadata 把 QQ 描述成弱交互平台。放弃，因为这会污染 capability truth。

### Decision 4: metadata contract 采取 additive 演进，不破坏现有 consumer

做法：
- 继续保留现有 health / registration / capability booleans。
- 新增或细化 async update preference、fallback category、reply-target usability、以及 provider-owned completion hints 时，只做 additive metadata / capability matrix 扩展。

原因：
- 当前 backend、operator、README、runbook 已经依赖现有 registration / health payload；直接替换字段会把本次 focused change 变成跨栈重构。
- additive path 更适合这次“补齐 lifecycle”而不是“重写 contract”。

备选方案：
- 直接把现有 capability matrix 改成全新 schema。放弃，因为会让 apply 面过大、验证成本上升。

### Decision 5: 验证以 focused contract tests + replay/update smoke matrix 为主，不以 repo-wide green 为前提

做法：
- 以 `src-im-bridge` 中国平台 package tests、delivery / reply-strategy / notify focused tests、以及必要的 `src-go` control-plane focused tests 作为本 change 的可信 gate。
- 对与本 change 无关的 repo 噪音保持边界清晰，不把 unrelated red 扩成阻塞。

原因：
- 当前 checkout 已有与 IM 生命周期无关的测试噪音；若要求 repo-wide 一次性清空，会让 proposal scope 失真。
- 用户更在意真实边界和可复现验证，而不是表面“大满贯绿”。

备选方案：
- 以全仓 `go test ./...` / root all-in-one gate 作为唯一完成条件。放弃，因为不符合当前仓库真相。

## Risks / Trade-offs

- [Risk] DingTalk、WeCom、QQ Bot 的官方回写能力与 Feishu 并不等价，若设计过度追求统一，容易再次制造假 parity。  
  → Mitigation：把 update semantics 保持在 provider seam，并要求 capability matrix 与 fallback metadata 直接反映真实行为。

- [Risk] replay、action completion、progress streaming 统一走一条 delivery path 后，任何 shared core 回归都可能同时影响多个回写入口。  
  → Mitigation：增加 focused contract tests，分别覆盖 direct notify、bound progress、`/im/action` completion、以及 reconnect replay。

- [Risk] additive metadata 字段短期内可能未被所有 consumer 读取，导致“实现补齐了但 operator 看不全”。  
  → Mitigation：保持旧字段兼容，并在 runbook / verification matrix 中明确哪些新字段是可选增强而非破坏性前置条件。

- [Risk] 若实现过程中发现某个平台官方确实不支持期望中的回写方式，proposal 可能需要从“补齐原生 update”转为“强化 explicit fallback”。  
  → Mitigation：在 spec 中把“优先原生路径，无法兑现时明确 downgrade”写成合同，而不是提前承诺所有平台都能 edit in place。

## Migration Plan

1. 先在现有 provider seam 上补 capability matrix / reply-target / update decision 所需的最小 contract 扩展，保持 health / registration additive 兼容。
2. 逐个平台收紧 DingTalk、WeCom、QQ Bot、QQ 的 reply / update / fallback 语义，并让 progress streaming / action completion 复用相同 decision path。
3. 运行中国平台 focused tests 与 control-plane replay / progress verification，记录真实可复现命令。
4. 更新 runbook / smoke matrix，使新 lifecycle truth 与 operator-visible 文档一致。
5. 若 rollout 后发现某个平台新回写策略不稳定，可回退到该 provider 既有 send-or-reply path，同时保留新的 metadata 与 downgrade reporting，不需要回滚 provider registry 或 control-plane 主链路。

## Open Questions

- DingTalk 当前是否只应收紧 session-webhook / conversation reply 的优先级与 downgrade reporting，还是还需要引入更强的 card-completion builder 才算 apply-ready？
- WeCom 的 richer send 与 callback reply 边界，是否需要在本次 change 中进一步区分“reply-first but non-mutable”与“direct-send richer fallback”两个 operator-visible tier？
- QQ Bot 是否要在本次 change 中把 keyboard-compatible completion 提升为 reply-target-aware primary path，还是先维持 markdown-first send with explicit fallback？
- `src-go/internal/handler` 当前被 unrelated `memory_explorer_service.go` 编译错误阻塞；本次 apply 是否接受以 IM-scoped focused verification 作为主 gate，并把该噪音明确标注为 repo-external blocker？
