## 1. Feishu Callback Contract

- [x] 1.1 拆分并收紧 Feishu send/reply、callback acknowledgement、deferred update 的 payload 构造，确保各路径使用各自文档支持的 interactive/card body 结构
- [x] 1.2 统一 Feishu callback-ready 判定逻辑，只在长连接或 webhook callback intake 真实可用时暴露 callback-backed affordance
- [x] 1.3 对齐 callback token、message identity、fallback reason 的保留与回退行为，确保同步响应与后续 delayed update 使用同一条真实 reply-target 合约

## 2. Feishu Rendering And Help Surface

- [x] 2.1 更新 Feishu renderer/builders，使结构化 operator 响应只使用 provider-safe 的 interactive card 元素与 markdown/text 槽位
- [x] 2.2 修复 `/help` 在飞书下的 markdown/card 渲染与快捷按钮 gating，确保 callback-ready 与 callback-missing 两条路径都输出可读且真实的指导
- [x] 2.3 对齐 native text/help card helper 与渲染元数据，让 Feishu richer text、structured help、以及 callback update 选择兼容的卡片结构

## 3. Coverage, Docs, And Verification

- [x] 3.1 补齐 Feishu 单元测试，覆盖 interactive envelope、callback response body、readiness gating、delayed-update compatibility、以及 `/help` 渲染回归
- [x] 3.2 在 `src-im-bridge` 运行 `go test ./... -count=1`，修复新规格收紧后暴露的飞书回归
- [x] 3.3 运行针对 Feishu 的 stub 或 callback smoke，验证 `/help` 与 callback-backed card flow 的端到端契约
- [x] 3.4 如行为或配置说明发生变化，更新 `src-im-bridge` README / runbook 中的飞书 callback 配置与 `/help` fallback 说明
