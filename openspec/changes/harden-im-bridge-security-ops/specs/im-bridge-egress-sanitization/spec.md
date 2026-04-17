## ADDED Requirements

### Requirement: Bridge SHALL sanitize outbound text according to a provider-aware policy

IM Bridge SHALL 在 `DeliverText` 与 `deliverRenderedText` 的实际写出前调用统一的 `SanitizeEgress(profile, text)` 过程，按照 `IM_SANITIZE_EGRESS=strict|permissive|off`（默认 `strict`）的当前档位执行过滤。Sanitize 过程 MUST 返回净化后的文本以及结构化 warnings 列表（`broadcast_mention_stripped`、`text_truncated`、`zero_width_stripped` 等），warnings MUST 进入当次 delivery 的 `metadata.sanitize_warnings` 字段并触发一条审计 event。

#### Scenario: Strict mode strips broadcast mentions
- **WHEN** 出站文本包含 `@everyone` / `@here` / Slack `<!channel>` / Telegram `@channel` 之一，且 `IM_SANITIZE_EGRESS=strict`
- **THEN** 出站到目标 IM 的文本中该类 token 被替换为 `[广播已屏蔽]`
- **AND** warnings 中出现 `broadcast_mention_stripped`
- **AND** 审计 event 的 `metadata.sanitize_warnings` 包含同名 warning

#### Scenario: Strict mode truncates or segments oversized text by provider profile
- **WHEN** 出站文本长度超过 provider `RenderingProfile.TextLengthLimit`
- **THEN** 若 `RenderingProfile.SegmentOversized=true`（如 Telegram），Bridge 分段按序投递
- **AND** 若 `SegmentOversized=false`，Bridge 在末尾截断并附加 `…[已截断]` 提示
- **AND** warnings 中出现 `text_truncated` 或 `text_segmented`

#### Scenario: Permissive mode keeps mentions but still removes zero-width characters
- **WHEN** `IM_SANITIZE_EGRESS=permissive`
- **THEN** broadcast mention 保留原样投递
- **AND** `U+200B/200C/200D/FEFF` 仍被剔除
- **AND** warnings 仅包含 `zero_width_stripped` 相关条目

#### Scenario: Off mode bypasses sanitization entirely
- **WHEN** `IM_SANITIZE_EGRESS=off`
- **THEN** 文本透传到 provider send 路径，不进行任何改写
- **AND** 审计 event 的 `metadata.sanitize_mode` 明确为 `off`，便于回查为什么未净化

### Requirement: Rendering profile SHALL declare a truthful TextLengthLimit

所有 provider 的 `RenderingProfile` MUST 在 `MetadataForPlatform` 注册时声明一个 `TextLengthLimit > 0` 的值（以 UTF-8 字节数或字符数为单位，由 provider 自身定义并在注释里说明）。`selectProvider` 启动流程 MUST 拒绝 `TextLengthLimit == 0` 的 provider 配置，以避免 "无限长文本" 的实际上的限速漏洞。

#### Scenario: Provider registration with zero-length limit is rejected
- **WHEN** 某 provider 声明 `RenderingProfile.TextLengthLimit=0`
- **THEN** Bridge 启动阶段以明确错误拒绝该 provider 的注册
- **AND** 错误信息指向 provider id 以便快速定位

#### Scenario: Sanitizer respects declared limit per provider
- **WHEN** 一个 Feishu 和一个 Telegram provider 同时被注册（多 bridge 实例场景），各自声明 `TextLengthLimit=4500` 与 `TextLengthLimit=4096`
- **THEN** 超长文本分别按各自 limit 计算截断或分段
- **AND** 各 provider 的净化行为互不影响
