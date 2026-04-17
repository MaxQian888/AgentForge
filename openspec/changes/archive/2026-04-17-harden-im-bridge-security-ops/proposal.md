## Why

IM Bridge 当前的安全/运维面（签名、幂等、限速、审计）只有"能工作"级别的实现，离企业级可运维还有明显缺口：

- **幂等 dedupe 在内存里**：`notify.Receiver.processed` 是进程内 `map[string]struct{}`，重启即丢、不跨副本共享。控制面 HTTP 重试 + bridge 滚动重启的组合里，同一 `deliveryId` 会再次通过 HMAC 校验并被当成新消息真的发出去，用户侧就是重复推送。
- **签名不校验时间戳窗口**：`verifyCompatibilitySignature` 只做 HMAC 比对，`X-AgentForge-Delivery-Timestamp` 被视作签名材料但不检查时间偏差。一条被捕获的合法签名请求可在任意时间点重放，重启把 dedupe map 清空后更是无防御。
- **限速只有一个维度**：`core.RateLimiter` 用 `platform:chatId:userId` 做 key，没有 per-tenant / per-chat / per-action 分层。同一用户可以用同一 session 在多个命令上聚合打满额度，运营端无法按命令类型限流。
- **审计日志缺失**：`/im/send`、`/im/notify`、`/im/action` 只在 logrus 文本日志里留痕，没有结构化事件流。出现"这条消息到底谁下发的 / 是不是重复了 / 为什么降级"这类运营问题时无可追溯的来源。
- **出站无净化**：命令返回的 `result.Result` 直接进入 `DeliverText`，对各 IM 平台的 `@everyone/@here/@all`、长度限制、零宽字符等没有统一策略。PRD 8.7 风险 #10 明确把 OpenClaw 2026.01 的 API key 泄漏事件（21,000+ 公网暴露实例）列为风险样板 —— 出站内容污染在 IM 场景是真实攻击面。
- **IM 命令无 bridge 侧前置**：任何能让 bot 看到自己消息的 IM 用户都能发 `/task create`；策略判定必须一次 round-trip 到后端，负载上浮且灰度不可控。
- **凭据无热重载**：provider 凭据都是 env 变量，轮转必须重启进程。当前 single-active-provider 模型里一次重启 = 一段业务不可达。

## What Changes

### 核心加固

- **持久化 dedupe + 限速状态**：新增 `src-im-bridge/core/state` 包，基于 SQLite 的 `${IM_BRIDGE_STATE_DIR}/state.db`（默认 `.agentforge/state.db`）保存已处理 `deliveryId`、`nonce` 和滑窗限速计数。在内存 map 之外并行承担真理，重启后继续防重。
- **时间戳窗口校验**：入站签名请求必须通过 `|now - X-AgentForge-Delivery-Timestamp| ≤ IM_SIGNATURE_SKEW_SECONDS`（默认 300s）。窗口外请求即使 HMAC 正确也拒绝。dedupe TTL 与该窗口对齐，避免存储无限增长。
- **结构化审计日志**：新增 `src-im-bridge/audit` 包，append-only JSONL 写到 `${IM_BRIDGE_AUDIT_DIR}/audit.jsonl`（默认 `.agentforge/audit`），轮转策略 size+age。每条 event 字段：`direction`（ingress/egress/action）、`surface`（/im/send|/im/notify|/im/action|control_plane）、`deliveryId`、`platform`、`bridgeId`、`chatIdHash`（SHA-256 截断）、`userIdHash`、`action`、`status`、`deliveryMethod`、`fallbackReason`、`latencyMs`、`signatureSource`、`timestamp`。可选通过控制面 ack 的新增 `audit_event` 字段回传后端汇聚。
- **多维限速 policy**：`RateLimiter` 接受 `RateLimitPolicy` 列表，每条含 `KeyDimensions`（any subset of `tenant|chat|user|action_class|command|bridge`）、`Rate`、`Window`。Engine 在命令路由前后分别调用 policy：`command` 维度用于 `/task`/`/agent` 区分，`action_class` 用于后端返回类别后补限速。默认 policy 集合保持当前 20/min per `platform:chat:user` 行为，新增 `10/min per action_class=write`。
- **出站净化策略**：新增 `core.SanitizeEgress(RenderingProfile, Text) (Text, []Warning)`。`IM_SANITIZE_EGRESS=strict|permissive|off`（默认 strict）下：剔除 `@everyone|@here|@all`-风格广播提及；按 provider 的 `TextLengthLimit` 智能分段；剔除 `U+200B/U+200C/U+200D/U+FEFF` 零宽字符。warning 进入审计 event 的 `metadata.sanitize_warnings`。
- **Bridge 侧命令 allowlist**：`IM_COMMAND_ALLOWLIST` env（空 = 不限制），支持按命令前缀和 `platform:` 前缀组合（`feishu:/task,feishu:/help,slack:/*`）。不在列表内的命令走"未授权"回复，不再 round-trip 到后端。作为"还没上 RBAC 时的灰度闸门"使用。
- **凭据热重载**：`SIGHUP` 触发 `provider.Reconcile(cfg)`。live transport 能重连的（feishu long-connection、slack socket mode、telegram long-poll、discord interactions server、qq onebot ws）先优雅断开再以新凭据连接；不能热换的（webhook 监听 port）打印明确的 `manual_restart_required` warning。

### 契约破坏

API 稳定期内自由破坏（见 project_api_stability_stage 内部约定）：
- `notify.Receiver.verifyAndRememberDelivery` 语义从 "reject unsigned if secret set" 扩展为 "reject unsigned | signature mismatch | timestamp out of window | duplicate delivery id | duplicate nonce"，各自带明确的 4xx 分类（`401 invalid_signature / 408 timestamp_out_of_window / 409 duplicate_delivery`）。
- `client.ControlDeliveryAck` 新增可选字段 `audit_event`（嵌套 JSON），后端侧如果消费需要对齐 schema。
- `core.RateLimiter.Allow(key string) bool` 签名替换为 `Allow(ctx, Scope) (Decision, error)`；所有内部调用点同步改写。
- 新增 env: `IM_BRIDGE_STATE_DIR`、`IM_SIGNATURE_SKEW_SECONDS`、`IM_BRIDGE_AUDIT_DIR`、`IM_AUDIT_ROTATE_SIZE_MB`、`IM_COMMAND_ALLOWLIST`、`IM_SANITIZE_EGRESS`、`IM_RATE_POLICY`（JSON-encoded override，可选）。`.env.example` 更新。

## Non-Goals

- 不引入 multi-tenant 路由或 multi-provider-per-process（change B）。
- 不新增 attachment / reaction / thread 一等对象（change A）。
- 不对 HMAC 算法或 `X-AgentForge-*` header 名称做改动（保持控制面兼容）。
- 不做完整的 secrets vault 集成，只暴露 `CredentialProvider` seam 供未来对接。
- 不在 bridge 侧做 project RBAC 精细校验（那是后端 `project-access-control` capability 的职责）；allowlist 只做粗粒度灰度。
- 不改 control-plane cursor/replay 语义；dedupe 只是把幂等真理从内存搬到磁盘。

## Capabilities

### New Capabilities
- `im-bridge-durable-state`：定义 bridge 侧持久化 dedupe + 限速 + nonce 状态存储契约，覆盖 schema、TTL、并发安全、跨副本/重启行为。
- `im-bridge-audit-trail`：定义结构化审计事件 schema、append-only JSONL 文件约束、可选控制面回传通道。
- `im-bridge-egress-sanitization`：定义出站文本净化策略、rendering profile 对 `TextLengthLimit`/`BroadcastMentionPolicy` 的新约束。

### Modified Capabilities
- `im-bridge-control-plane`：签名校验扩展时间戳窗口 + 持久化 dedupe 取代内存 map；明确 4xx 分类；ack 增补 `audit_event`。
- `im-rich-delivery`：`DeliveryEnvelope` metadata 约定新增 `sanitize_warnings` 字段（仅审计用途，不进入实际 IM payload）。
- `additional-im-platform-support`：所有 provider 必须声明 `RenderingProfile.TextLengthLimit` 的实际值而不是 `0=unlimited`；registration 时未声明视为配置错误。

## Impact

- **运行时** (`src-im-bridge/`)
  - 新增：`core/state/` 包（SQLite dedupe/rate/nonce store）、`audit/` 包（JSONL writer + 控制面 shipper）、`core/sanitize.go`、`core/ratelimit_policy.go`。
  - 修改：`core/ratelimit.go`（签名切换为 policy-based）、`core/engine.go`（allowlist + 命令维度限速埋点）、`notify/receiver.go`（时间戳窗口 + 持久化 dedupe + 审计埋点）、`client/control_plane.go`（ack 增补 `audit_event`）、`cmd/bridge/main.go`（wiring + SIGHUP 热重载钩子）、各 provider `Reconcile(cfg)` 方法（用于热重载）。
- **配置 / 文档**
  - `src-im-bridge/.env.example` 追加新 env。
  - `src-im-bridge/README.md` 新增 "Security & Ops" 章节。
  - `src-im-bridge/docs/platform-runbook.md` 补审计字段说明、时间戳窗口排障、SIGHUP 热重载清单。
- **后端可选对齐** (`src-go/`)
  - `internal/ws/im_control_handler.go` 若消费 `audit_event` 字段，则需对齐 schema；默认忽略即可保持向前兼容。
  - `internal/service/im_control_plane.go` 的 settlement history 可选追加 `audit_ref`。
- **测试**
  - `core/state/*_test.go`：dedupe TTL、并发安全、SQLite busy retry。
  - `audit/*_test.go`：JSONL schema、轮转、敏感字段 hash。
  - `core/sanitize_test.go`：各 platform 的 broadcast mention 规则 + 长度截断 + 零宽处理。
  - `notify/receiver_test.go` 新增 skew reject、重放 reject、policy-based 限速、allowlist gate 用例。
  - `cmd/bridge/main_test.go` 新增 SIGHUP 路径。
