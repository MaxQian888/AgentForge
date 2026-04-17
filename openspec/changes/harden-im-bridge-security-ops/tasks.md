## 1. Durable state foundation (`core/state` 包)

- [x] 1.1 在 `src-im-bridge/core/state/` 新建 package，引入 `modernc.org/sqlite`（纯 Go），schema 与 design.md Decision 1 对齐。
- [x] 1.2 实现 `DedupeStore.Seen(id, surface, ttl) (bool, error)`、`NonceStore.Consume(nonce, scope, ttl) (bool, error)`、`RateStore.Record(scopeKey, policyId, ts) error` + `Count(scopeKey, policyId, since) (int, error)`。
- [x] 1.3 后台 cleanup goroutine：每 30s `DELETE WHERE expires_at < now` / `DELETE WHERE occurred_at < cutoff`。
- [x] 1.4 启用 WAL + `busy_timeout=5s` + 合理 `journal_size_limit`；compile-time 校验纯 Go 构建通过。
- [x] 1.5 Focused tests：并发 Seen/Seen 同 id（单一 true）、TTL 过期复用、清理 goroutine 命中、SQLite busy 重试计数、重启后 state 仍生效（新开 DB 指针）。

## 2. Signature timestamp window + durable dedupe 接入

- [x] 2.1 `notify/receiver.go`：`verifyAndRememberDelivery` 改为：先 HMAC → 再 `IM_SIGNATURE_SKEW_SECONDS` skew 校验 → 再查 durable dedupe。
- [x] 2.2 错误分类：`401 invalid_signature` / `408 timestamp_out_of_window` / `409 duplicate_delivery`；response body 带 `{"error":"...","retryable":bool}`。
- [x] 2.3 `cmd/bridge/main.go`：启动时注入 `DedupeStore` 实例，`Receiver.SetDedupeStore(...)`。
- [x] 2.4 Focused tests：`notify/receiver_test.go` 新增 skew reject / within-window accept / duplicate delivery id / 重启后 dedupe 仍拦截 用例。
- [x] 2.5 内存 `processed map` 保留为 L1 快缓存（仅当前请求周期减少 DB 查询），但真理在 SQLite；移除依赖注释。

## 3. Audit package (`audit/`)

- [x] 3.1 新建 `src-im-bridge/audit/` 包：`Writer` 接口、`FileWriter`（append JSONL，mutex 保护 `os.File`）、`RotatingWriter` 叠加轮转（size+age）。
- [x] 3.2 事件 schema 用 `audit.Event` struct，`MarshalJSON` 输出 v1 schema。`Salt` 从 `IM_AUDIT_HASH_SALT` 读；空则从 state.db `settings(key='audit_salt')` 读或生成。
- [x] 3.3 `audit.Emit(event)` 同步写入磁盘（非阻塞 buffered 可选但第一版直写，保证崩溃时 event 已落盘）。
- [x] 3.4 在 `notify/receiver.go` 四个入口埋点：`/im/send`、`/im/notify`、`/im/action`、inbound callback。
- [x] 3.5 在 `core/delivery.go` 的 `executeRenderingPlan` 埋点：delivered / downgraded / fallback_reason（receipt.FallbackReason 已被 receiver-level emit 捕获；再做更深 hook 见 §6）。
- [ ] 3.6 在 `core/engine.go` 命令路由前后埋点：allowlist reject / rate limit reject / command executed（随 §4/§6 实施）。
- [x] 3.7 Focused tests：`audit/writer_test.go` 覆盖并发写、轮转触发、保留期清理；`audit/schema_test.go` 锁 schema v1 字段（合并到 writer_test.go）。
- [ ] 3.8 Optional：`client.ControlDeliveryAck.AuditEvent` 字段 + `IM_AUDIT_SHIP_VIA_CONTROL_PLANE` 开关（默认 false）—— 后端消费需配合，本 change 暂不实施。

## 4. Multi-dimensional rate limit policy

- [x] 4.1 `core/ratelimit_policy.go`：定义 `RateDimension`、`RateLimitPolicy`、`Scope`、`Decision` 类型。
- [x] 4.2 替换 `core/ratelimit.go` 的签名：`Allow(ctx, scope) (Decision, error)`；内部按每条 policy 计算 composite key（拼 `policy.Dimensions` 取出 scope 对应值 → sha256 → hex）。
- [x] 4.3 默认 policy 集合（design Decision 4）注册为 `DefaultPolicies()`；`IM_RATE_POLICY` JSON 覆盖解析。
- [x] 4.4 `core/action_class.go` 增加 `ActionClassForCommand(command) RateActionClass` map，覆盖 design 里列出的所有命令。
- [x] 4.5 `core/engine.go`：命令解析后、handler 执行前调用 `rateLimiter.Allow(ctx, Scope{...})`；被拒 → 回 `Decision.RetryAfterSec` 信息 + 审计 emit（engine 侧 emit 留给 §6 整合）。
- [x] 4.6 Focused tests：`core/ratelimit_test.go`——多 policy 并存、命中顺序、retry_after 准确、并发 Allow 单一通过、legacy 兼容层。

## 5. Egress sanitization

- [x] 5.1 `core/sanitize.go`：`SanitizeEgress(profile, mode, text) SanitizeResult` 实现 Decision 5 的规则 + 三档模式。
- [x] 5.2 复用现有 `RenderingProfile.MaxTextLength` / `SupportsSegments` 字段（已满足需求）；无需新增字段。
- [x] 5.3 `core/reply_strategy.go` 的 `DeliverText` 在写出前调用 `SanitizeEgress`；warnings 追加到 `ReplyPlan.FallbackReason`；多段在 send 路径按段投递。
- [x] 5.4 Focused tests：`core/sanitize_test.go`——broadcast mention 样例、长度分段、零宽清理、控制字符、多字节边界、`off/strict/permissive` 三档切换。
- [ ] 5.5 Deferred：`MaxTextLength>0` 的启动校验与 email 的"无上限"语义冲突（email 合法地声明 `MaxTextLength=0`）。sanitize 把 `0` 视作 unbounded，语义正确但与 spec 文字不完全一致；后续与 provider catalog change 一起调整。

## 6. Command allowlist gate

- [x] 6.1 `core/allowlist.go`：解析 `IM_COMMAND_ALLOWLIST`（逗号分隔，`platform:command` 格式，`!` 前缀为 deny，`*`/`/*` 通配），构建 matcher。
- [x] 6.2 `core/engine.go`：命令路由入口在 rate limit 之前先走 allowlist；未授权 → 固定中文文案回复（审计 emit 在 §3.6 补 engine hooks 时统一加）。
- [x] 6.3 Focused tests：`core/allowlist_test.go` 新增 allowlist 用例（通配符、黑名单优先、空值 fallthrough、platform 维度切分、大小写、畸形条目）。

## 7. Credential hot reload

- [x] 7.1 `core/platform.go` 新增可选接口 `HotReloader`；定义 `ReconcileConfig` / `ReconcileResult`。
- [x] 7.2 `cmd/bridge/main.go`：SIGHUP handler（Unix only；Windows 明确 no-op + log 提示），收到信号 → 重新 `loadConfig()` → 调 `platform.Reconcile()`；unreconcilable → `manual_restart_required` warning。
- [ ] 7.3 Deferred：`platform/feishu`、`platform/slack`、`platform/telegram`、`platform/qq` 实现 `HotReloader` 的具体重连逻辑（每 provider 需独立改造，属大头工程，留给后续 change）。
- [x] 7.4 不实现 HotReloader 的 provider 在 Reconcile 调用路径里命中 `manual_restart_required` warning 分支，不 panic。
- [ ] 7.5 Deferred：provider-specific hot-reload tests 与 §7.3 同步推进。

## 8. Config & documentation

- [x] 8.1 `src-im-bridge/.env.example`：追加所有新 env（`IM_BRIDGE_STATE_DIR`、`IM_DISABLE_DURABLE_STATE`、`IM_SIGNATURE_SKEW_SECONDS`、`IM_BRIDGE_AUDIT_DIR`、`IM_AUDIT_ROTATE_SIZE_MB`、`IM_AUDIT_RETAIN_DAYS`、`IM_AUDIT_HASH_SALT`、`IM_AUDIT_SHIP_VIA_CONTROL_PLANE`、`IM_DISABLE_AUDIT`、`IM_COMMAND_ALLOWLIST`、`IM_SANITIZE_EGRESS`、`IM_RATE_POLICY`）+ 注释行默认值。
- [x] 8.2 `src-im-bridge/README.md` 新增 "Security & Ops Hardening" 章节，覆盖 durable state / signed contract / audit / rate / sanitize / allowlist / hot reload 的运维视角。
- [x] 8.3 `src-im-bridge/docs/platform-runbook.md` 追加 "Security & ops troubleshooting"：timestamp 失配排障、duplicate 行为、rate limit 诊断、audit 文件摄取、SIGHUP 热重载、hardening disable 开关总表。

## 9. Verification and rollout evidence

- [x] 9.1 运行 `go test ./... -count=1`，全部 17 个 package 绿；新增测试覆盖 state / audit / ratelimit policy / allowlist / sanitize / 安全 receiver 路径。
- [ ] 9.2 Deferred：`scripts/smoke/security-smoke.ps1` 脚本（可作为后续补强；go test 已覆盖路径语义）。
- [ ] 9.3 Deferred：`pnpm dev:all` 全栈 smoke（需真实 backend 同步 `audit_event` schema 才有端到端可观察价值）。
- [ ] 9.4 Deferred：`docs/part/TECHNICAL_CHALLENGES.md` 交叉引用 `im-bridge-audit-trail` 与 `im-bridge-durable-state`（文档同步通常在 change archive 时合并）。
