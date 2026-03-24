## Why

PRD 和 `PLUGIN_SYSTEM_DESIGN.md` 把 AgentForge 的 IM 能力定位成“国内外多平台原生支持”，但当前仓库里的 IM Bridge 仍主要停留在“共享命令 + 最小通知回退”层：Slack、Discord、Telegram、飞书、钉钉都已经有 live transport 或 control-plane 基础，却还没有把各平台官方支持的线程、延迟回复、消息编辑、按钮回调、卡片更新、模态交互等特色能力收敛成稳定契约。现在 transport、reply-target、Bridge 注册和异步回传基础都已具备，正是把“能连上平台”补成“在每个平台里像原生应用一样可用”的窗口期。

## What Changes

- 为当前已支持的五个平台建立一份权威能力矩阵，明确 Slack、Discord、Telegram、飞书、钉钉在命令入口、交互组件、延迟确认、消息编辑、线程/话题回复、结构化消息和回调时限上的官方支持边界。
- 把 IM Bridge 的能力模型从 `SupportsRichMessages` 这类粗粒度布尔位扩展为可驱动真实策略选择的能力声明，使后端和 Bridge 能根据平台特性选择 reply、edit、follow-up、thread reply、session webhook 或纯文本降级。
- 为每个平台补齐原生交互闭环，而不再只依赖最低公共分母：
  - Slack：Block Kit、线程内回传、`response_url`/交互回调、模态入口和后续更新策略。
  - Discord：3 秒内 defer、follow-up、原始响应编辑，以及组件/命令回调的统一 reply-target 恢复。
  - Telegram：inline keyboard、callback query、消息编辑/替换、topic/thread 感知和低噪音进度更新。
  - 飞书：新版卡片回调、3 秒即时响应、30 分钟延时更新 token、卡片按钮动作与文本/卡片双轨输出。
  - 钉钉：Stream + session webhook 回复、ActionCard/交互降级、群聊与会话回复目标的稳定恢复。
- 收敛 `src-im-bridge`、Go 后端 IM 模型和 OpenSpec 规格，使“平台特色能力”成为后续继续扩展企微/LINE/QQ/微信时可复用的实现模板，而不是散落在 adapter 内部的特例判断。
- 为仍未进入当前 active adapter 集合、但已在 PRD/模型层出现的平台（如 `wecom`）列出缺口和后续接入前提，避免路线图遗漏。

## Capabilities

### New Capabilities
- `im-platform-native-interactions`: 为已支持 IM 平台定义原生命令、交互组件、消息更新与平台特色降级策略，使同一条 AgentForge 流程在不同平台上都能采用用户预期的交互方式。

### Modified Capabilities
- `additional-im-platform-support`: 从“平台可启动、共享命令可运行、能力感知回退可用”升级为“平台能力矩阵可声明、reply target 可表达原生交互上下文、通知和进度更新必须遵循各平台特色策略”。

## Impact

- Affected code: `src-im-bridge/core/*`, `src-im-bridge/platform/*`, `src-im-bridge/cmd/bridge/*`, `src-im-bridge/notify/*`, `src-im-bridge/client/*`, `src-go/internal/model/im.go`, `src-go/internal/service/im_*`, 以及相关测试。
- Affected docs/specs: `docs/PRD.md`, `docs/part/PLUGIN_SYSTEM_DESIGN.md`, `src-im-bridge/README.md`, `src-im-bridge/docs/platform-runbook.md`, `openspec/specs/additional-im-platform-support/spec.md`。
- Affected integrations: Slack Developer Docs, Discord Interactions, Telegram Bot API, Feishu/Lark 卡片与回调体系，以及钉钉开放平台 Stream / 机器人交互语义。
- Operational impact: 需要补充平台级 capability 验证、按钮/卡片回调 smoke tests、消息编辑与 follow-up 限制验证，以及针对未来 `wecom` 接入的显式未完成边界。
