# im-bridge-audit-trail Specification

## Purpose
Define the structured audit logging contract for AgentForge IM Bridge, covering event emission for every delivery surface, PII-safe hashing of identifiers, bounded log rotation and retention, and optional mirroring to the backend via control-plane ack.
## Requirements
### Requirement: Bridge SHALL emit structured audit events for every delivery surface

IM Bridge SHALL 对 `/im/send`、`/im/notify`、`/im/action`、inbound provider callback 与 control-plane delivery 的每一次处理（无论成功、被拒还是降级）发射一条结构化审计 event。事件 MUST 写入 `${IM_BRIDGE_AUDIT_DIR}/audit.jsonl`（默认 `.agentforge/audit/`），每行一个 JSON 对象，schema 版本字段 `v` 必须存在。事件 MUST 包含：`ts`、`direction`、`surface`、`deliveryId`、`platform`、`bridgeId`、`chatIdHash`、`userIdHash`、`action`、`status`、`deliveryMethod`、`fallbackReason`、`latencyMs`、`signatureSource`。

#### Scenario: Successful outbound delivery produces an audit event
- **WHEN** Bridge 成功处理一条 `/im/send` 请求，投递到 Feishu 群聊
- **THEN** `audit.jsonl` 追加一条 event，`direction=egress`、`surface=/im/send`、`status=delivered`、`platform=feishu`
- **AND** `deliveryMethod` 反映实际投递方式（如 `reply`、`send`、`deferred_card_update`），`fallbackReason` 若无降级则为空

#### Scenario: Rejected inbound request is still audited
- **WHEN** 一条 `/im/notify` 请求签名校验失败
- **THEN** `audit.jsonl` 追加一条 event，`direction=ingress`、`status=rejected`、`metadata` 含明确的拒绝原因（`invalid_signature`、`timestamp_out_of_window`、`duplicate_delivery` 之一）
- **AND** `latencyMs` 为入站到拒绝决定之间的耗时，`signatureSource` 保留 `shared_secret` 或 `unsigned`

#### Scenario: Rate-limited command captures policy context
- **WHEN** 用户在短时间内触发 `/task create` 超过 `write-action` policy 上限
- **THEN** 审计 event 的 `status=rate_limited`、`metadata.rate_policy` 指明被拒的 policy id（如 `write-action`）
- **AND** 后续重试成功时另发一条 `status=delivered` event，两者的 `action` 字段一致

### Requirement: Audit log SHALL hash user and chat identifiers

IM Bridge SHALL 对 `chatId`、`userId` 等 PII 字段统一做 `HMAC-SHA256(salt, raw)[:16]` hash 后再写入 audit event。`salt` 由 `IM_AUDIT_HASH_SALT` 提供；若未设置，Bridge MUST 在首次启动时生成随机 salt 并持久化到本地状态存储，后续启动复用同一 salt 以保证跨日期查询一致性。原始 PII MUST NOT 出现在任何 audit event 字段中。

#### Scenario: Salt is generated and reused across restarts
- **WHEN** 首次启动且 `IM_AUDIT_HASH_SALT` 未设置
- **THEN** Bridge 生成一个 256-bit salt 写入 `state.db.settings(key='audit_salt')`
- **AND** 重启后同一 `chatId` 产生相同的 `chatIdHash`

#### Scenario: Explicit salt override is honored
- **WHEN** 运营显式设置 `IM_AUDIT_HASH_SALT`，且与持久化的 salt 不同
- **THEN** Bridge 使用 env 值作为当前运行期 salt，并在启动 log 明确标注 `audit_salt_source=env`
- **AND** 审计流记录 `direction=internal, action=audit_salt_override`

### Requirement: Audit files SHALL rotate and retain bounded history

IM Bridge SHALL 按 `IM_AUDIT_ROTATE_SIZE_MB`（默认 128）或每日零点 UTC 触发 audit 文件滚动，滚动后的文件命名为 `audit.YYYY-MM-DD-HHMM.jsonl`。Bridge SHALL 按 `IM_AUDIT_RETAIN_DAYS`（默认 14）清理过期 rotated 文件。当前 `audit.jsonl` 文件 MUST NOT 被清理逻辑误删。轮转失败（磁盘满、权限拒绝）MUST 触发审计自身的 `status=audit_rotate_failed` event 并继续写当前文件以避免静默丢失。

#### Scenario: Size-based rotation preserves all events
- **WHEN** `audit.jsonl` 大小达到 128MB
- **THEN** Bridge 关闭当前文件并以 `audit.<timestamp>.jsonl` 保存，随后打开新的 `audit.jsonl` 继续追加
- **AND** 滚动过程中到达的 event 要么落在旧文件尾要么落在新文件头，不丢失

#### Scenario: Retention cleans old files but not current file
- **WHEN** `IM_AUDIT_RETAIN_DAYS=14`，磁盘上有 16 天前的 rotated 文件与今日 `audit.jsonl`
- **THEN** 清理周期删除 14 天外的 rotated 文件
- **AND** 保留最近 14 天的 rotated 文件和当前 `audit.jsonl`

### Requirement: Audit MAY be mirrored to backend through control-plane ack

IM Bridge MAY 通过 `client.ControlDeliveryAck.AuditEvent` 字段把对应 delivery 的审计 event 一并回传给后端。该行为受 `IM_AUDIT_SHIP_VIA_CONTROL_PLANE=true|false` 控制（默认 false，仅本地 JSONL）。后端若消费该字段 MUST 做 forward-compatible 处理，即允许字段缺失/未知。Bridge MUST NOT 假设后端已经消费成功，本地 JSONL 始终是 audit 的第一真理源。

#### Scenario: Ack carries audit event when shipping is enabled
- **WHEN** `IM_AUDIT_SHIP_VIA_CONTROL_PLANE=true` 且 Bridge 处理一条 control-plane 下发的 delivery
- **THEN** 回传的 `ControlDeliveryAck` 额外携带当次 audit event 的 JSON 对象
- **AND** 该 event 已同步写入本地 `audit.jsonl`

#### Scenario: Backend ignores unknown AuditEvent field
- **WHEN** 后端尚未实现 `AuditEvent` 字段消费
- **THEN** 后端依然按既有 ack schema 正常更新 settlement，忽略额外字段
- **AND** Bridge 不会因为后端忽略而重试或报错

