## 1. 平台 readiness 与 capability truth 收紧

- [x] 1.1 在 `src-im-bridge` 的 provider descriptor / `core.PlatformMetadata` / control-plane registration seam 中新增中国平台 readiness tier 与 parity truth 字段，并保持现有 `IM_PLATFORM`、transport、capability booleans 兼容。
- [x] 1.2 让 Feishu、DingTalk、WeCom、QQ、QQ Bot 的 capability matrix、health metadata、registration metadata 基于真实 adapter 行为输出，不再把 richer/send/update 能力压扁成统一布尔值。
- [x] 1.3 为中国平台 metadata truth 增加 focused tests，校验 readiness tier、native surfaces、mutable-update truth 与 reply-target 能力声明一致。

## 2. 中国平台 native interaction / reply-target 生命周期对齐

- [x] 2.1 收紧 Feishu callback / delayed update 路径，确保 3 秒内即时响应、后续延时更新顺序、token 窗口与 fallback reason 合同 truthfully 生效。
- [x] 2.2 收紧 DingTalk 与 WeCom 的 native callback、reply-target 恢复、richer send 与 update fallback 行为，明确 native-send-only 与 mutable-update-unavailable 的边界。
- [x] 2.3 收紧 QQ 与 QQ Bot 的 text-first / markdown-first reply-target 与 action completion 语义，防止 richer request 被误表述为完整 rich-card lifecycle。

## 3. 共享 delivery / replay / action completion 一致性

- [x] 3.1 调整 `core/delivery`、`core/reply_strategy`、`notify/receiver` 与相关 provider sender/updater，使 direct notify、compat HTTP、control-plane replay、`/im/action` completion 复用同一套中国平台 rendering / fallback 判定。
- [x] 3.2 统一中国平台的 downgrade reason 与 delivery metadata，确保 richer fallback、update 不可用、reply-target 丢失等结果在 ack / replay / operator diagnostics 中保持一致。
- [x] 3.3 增加跨 transport focused tests，覆盖 Feishu delayed update、DingTalk ActionCard fallback、WeCom richer send downgrade、QQ text-first fallback、QQ Bot markdown-first fallback。

## 4. 文档、runbook 与验证矩阵同步

- [x] 4.1 更新 `src-im-bridge/README.md` 与 `src-im-bridge/docs/platform-runbook.md`，把中国平台矩阵改成 readiness tier / callback / native send / update truth 导向，而不是平铺式“已支持”。
- [x] 4.2 补齐中国平台 focused verification matrix 与 smoke 指引，明确每个平台的 stub/live、callback、reply-target、richer fallback 检查点。
- [x] 4.3 运行并记录最小可信验证（至少包含中国平台相关 package tests 与必要的 focused contract tests），确保 change 进入 apply 时有可复用的验证入口。
