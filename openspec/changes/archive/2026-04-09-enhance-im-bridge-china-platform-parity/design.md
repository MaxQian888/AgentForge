## Context

当前 `src-im-bridge` 的中国平台支持已经不是“有没有目录”的问题，而是“声明出来的能力是否与运行时真相一致”的问题。

已有事实：

- `src-im-bridge/platform/feishu`, `platform/dingtalk`, `platform/wecom`, `platform/qq`, `platform/qqbot` 都已经存在 live/stub 适配器与 targeted tests；本地 scoped 验证 `go test ./platform/feishu ./platform/dingtalk ./platform/wecom ./platform/qq ./platform/qqbot -count=1` 当前可通过。
- `src-im-bridge/cmd/bridge/platform_registry.go` 已经把这些 provider 接入统一 registry；`src-im-bridge/cmd/bridge/control_plane.go` 会把 `CapabilityMatrix`、基础 booleans 与少量 metadata 注册到 backend，但目前 metadata 还不足以表达“这个平台是 full native lifecycle 还是 text-first fallback”。
- `src-im-bridge/core/delivery.go`、`core/reply_strategy.go`、`notify/receiver.go` 已经形成共享 typed envelope + rendering plan + fallback reason 的主干，这意味着这次更适合继续收紧 provider truth，而不是再开一套中国平台专用 delivery pipeline。
- 真实能力明显不对称：Feishu 已有 callback token / delayed update / rich card lifecycle；DingTalk 与 WeCom 代码中仍保留显式 richer fallback 文案；QQ / QQ Bot 则更接近 text-first 或 markdown-first 路径。
- 官方接入约束也强化了这种不对称：Feishu 卡片交互要求 3 秒内响应且延时更新 token 有 30 分钟窗口；钉钉强调 Stream + card callback；企业微信依赖 callback + template card / markdown 语义；QQ Bot webhook/OpenAPI 具备 markdown/keyboard，但并不天然等于 Feishu 式 mutable card lifecycle。

因此，这次 change 的核心不是“再添加一个平台”，而是让中国平台的 capability matrix、rendering profile、reply-target、health/registration metadata、README/runbook 与真实 provider 行为重新对齐。

## Goals / Non-Goals

**Goals:**
- 为 Feishu / DingTalk / WeCom / QQ / QQ Bot 定义统一但 truthful 的 readiness / parity 表达，让 runtime、health、registration、docs 和 tests 使用同一套真相。
- 保留现有 provider registry、typed envelope、reply-target、control-plane 数据流，只在现有 seam 上增量收紧 capability 与 fallback 合同。
- 让 `/im/send`、`/im/notify`、`/ws/im-bridge` replay、`/api/v1/im/action` 对中国平台的 richer delivery / async completion 产生一致且可解释的结果。
- 把“Feishu 是完整 rich-card baseline，其他平台按官方能力 truthful 对齐”写进规格、runbook 与 focused verification matrix。

**Non-Goals:**
- 不新增全新中国平台，也不重新命名/替换现有 `IM_PLATFORM` 或凭据环境变量。
- 不为了追求表面 parity 而伪造 QQ、QQ Bot、WeCom 具备 Feishu 式 delayed mutable card lifecycle。
- 不重做前端 IM 控制台或 TS Bridge runtime 架构；如需 UI 强化 readiness tiers，仅暴露必要 contract，界面增强可后续另起 focused change。

## Decisions

### Decision 1: 在现有 provider metadata 上增加 readiness tier，而不是另建一套平台真相表

做法：扩展 provider descriptor / `core.PlatformMetadata` / bridge registration metadata，让每个平台显式声明类似 `full_native_lifecycle`、`native_send_with_fallback`、`text_or_markdown_first` 这类 readiness tier，以及与之对应的 mutable-update / callback / native-surface truth。`src-im-bridge/cmd/bridge/control_plane.go` 继续沿用当前 registration 流程，只新增可选 metadata 字段，不改现有桥接注册主结构。

原因：
- 当前 `BridgeRegistration`、`CapabilityMatrix` 与 `/im/health` 已经是现成真相出口；增量扩展比引入平行 truth table 风险更低。
- backend、runbook、README、后续 operator surface 都能复用同一套 runtime-produced metadata，而不是再维护手写平台矩阵。

备选方案：
- 只改 README / runbook，不改 runtime metadata。放弃，因为文档最容易继续漂移。
- 在 backend 单独维护 readiness 配置。放弃，因为会与 provider 实现脱节，且增加双向同步成本。

### Decision 2: 以 Feishu rich-card lifecycle 作为中国平台 parity baseline，但不要求所有平台伪装成同一能力层级

做法：保留 `feishu-rich-card-lifecycle` 作为 richest baseline；对 DingTalk / WeCom / QQ / QQ Bot 只要求：
- 把自己真实支持的 callback、native send、editable update、reply-target 恢复语义说清楚；
- 当 richer path 不可用时，通过统一 fallback reason 返回 truth，而不是复用 Feishu 的 happy path 术语。

原因：
- 当前代码已经证明 Feishu 是唯一具备完整 callback token + delayed update 生命周期的中国平台；强推“平台看起来一样”只会制造规格假象。
- 官方文档本身就给出了不同的交互模型，设计应尊重 provider contract，而不是掩盖差异。

备选方案：
- 抽象出“通用 delayed update”并要求所有中国平台实现。放弃，因为 QQ / QQ Bot / WeCom 当前官方模型并不支持等价语义。

### Decision 3: 所有中国平台的 richer delivery / action completion 继续走同一条 rendering plan，而不是在 handler 层分叉

做法：复用 `core/delivery.go`、`core/reply_strategy.go`、`notify/receiver.go` 的共享 typed envelope 数据流；平台差异只体现在 provider-owned rendering profile、native sender / updater、reply-target 恢复逻辑与 fallback reason 生成上。`/im/send`、`/im/notify`、replay、`/im/action` 都必须复用这条路径。

原因：
- 当前共享 delivery 主干已经存在，继续把差异压到 provider seam 才能保证 compat HTTP 与 control-plane replay 结果一致。
- 如果改成 handler 层大量 `switch platform`，后续文档、测试和 replay 很快再次漂移。

备选方案：
- 单独为中国平台写一套 special-case notify/action path。放弃，因为会重复已有能力并破坏跨 transport 一致性。

### Decision 4: 用“契约测试 + focused smoke + runbook matrix”定义 completeness，而不是把 completeness 等同于 live provider 目录存在

做法：
- 为中国平台补 focused contract tests，校验 metadata 宣称、native surface、fallback reason、reply-target 序列化/恢复、action completion truth 是否一致。
- 继续保留 package-local `go test` 入口，并在 runbook 中补中国平台 matrix，明确哪些步骤是 send-only、哪些支持 callback、哪些仅 text/markdown。
- 文档与 smoke fixture 从 provider metadata 反推验证项，减少“实现改了但文档没变”的概率。

原因：
- 用户要的是“检查现有支持是否完整”，这本质上是审计问题，不只是再加实现。
- 当前 targeted tests 能证明基础不坏，但还不足以证明 capability 声明与运行结果一致。

备选方案：
- 只做 live-provider 手工联调。放弃，因为依赖外部凭据，难以作为 repo 内稳定 gate。

### Decision 5: 对 backend / operator surface 采用 additive contract，旧字段不回退

做法：新增 readiness / parity 相关 metadata、downgrade reason 细化字段和 runbook truth，但不移除现有 `supports_*` booleans，也不破坏既有 registration / health consumer。旧 consumer 看不到新字段时仍可工作；新 consumer 可以读取更细粒度 truth。

原因：
- 当前 active change `complete-backend-bridge-connectivity` 仍在收尾，直接重构 control-plane 主 payload 容易造成交叉冲突。
- additive path 更适合 focused OpenSpec change，也更利于 archive 后 main spec 合并。

备选方案：
- 直接把旧 booleans 替换成复杂 readiness 结构。放弃，因为会扩大跨仓库修改面并提高验证成本。

## Risks / Trade-offs

- [Risk] runtime metadata 增加 readiness tier 后，provider descriptor、README、runbook 仍可能再次漂移。  
  → Mitigation：让 focused tests 断言 metadata 与真实 sender/updater 行为一致，并在 runbook 中直接引用 runtime field 名称。

- [Risk] DingTalk / WeCom / QQ Bot 的官方能力边界后续变化较快，spec 容易再度过时。  
  → Mitigation：把平台特定限制收敛到 provider-owned profile 和 runbook，避免在 shared core 中散落硬编码假设；文档里保留 provider-specific verification checkpoints。

- [Risk] 继续保留 QQ text-first、QQ Bot markdown-first、WeCom limited-update truth，可能会让“平台完整性”看起来不够漂亮。  
  → Mitigation：本 change 的目标是 truthful completeness，不是伪装 parity；只要 readiness tier、fallback reason 与 operator diagnostics 清晰，就满足工程真实性。

- [Risk] control-plane / backend 现有 consumer 只理解扁平 booleans，不消费新 metadata。  
  → Mitigation：采用 additive contract；旧字段继续保留，新字段只增强，不阻断现有链路。

## Migration Plan

1. 扩展中国平台 provider metadata / rendering profile / registration metadata，增加 readiness tier 与 parity truth 字段，同时保持现有 provider id 与 env contract 不变。
2. 收紧 Feishu / DingTalk / WeCom / QQ / QQ Bot 的 reply-target、native update、fallback reason 与 action completion 行为，使 shared delivery pipeline 能输出一致的 provider-aware truth。
3. 为中国平台补 focused contract tests，校验 metadata 声明、native surface、fallback reason、reply-target 恢复与 action completion outcome。
4. 更新 `src-im-bridge/README.md` 与 `src-im-bridge/docs/platform-runbook.md`，把中国平台 matrix 从“是否支持”改成“支持到什么 tier、何时会 fallback”。
5. 如上线后发现新 metadata 或 stricter fallback 影响现有 consumer，可先忽略新增 metadata 字段并回退 provider-specific 行为收紧，但不需要回滚 `IM_PLATFORM` 或 transport 入口。

## Open Questions

- WeCom 应在 readiness tier 中归类为 `native_send_with_fallback`，还是需要单独的 `callback_send_without_mutable_update` 粒度？
- QQ Bot 的 keyboard 是否要在本 change 中上升为 shared native action contract 的一部分，还是先维持 markdown/keyboard send-only truth，等 callback 语义更稳后再扩？
- backend/operator surface 是否需要在本 change 中直接消费 readiness tier，还是先只把 contract 落到 registration/health/docs，UI 读取留待后续 focused change？
