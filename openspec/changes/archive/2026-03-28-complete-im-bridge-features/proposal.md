## Why

IM Bridge 的 8 个平台适配器（后端 + Bridge 进程）已基本就绪，但前端 UI 仅覆盖 5 个平台（feishu/dingtalk/slack/telegram/discord），缺少 WeCom、QQ、QQ Bot 的配置与展示支持。同时，若干跨层联动能力（事件订阅扩展、DingTalk ActionCard 交互、review 子命令集成、native 降级上报）尚未打通。需要在前后端同步补齐，使 IM Bridge 达到 PRD 定义的功能完整状态。

## What Changes

- **前端平台补齐**：`IMPlatform` 类型扩展至 8 平台；`IMChannelConfig` / `IMBridgeHealth` / `IMMessageHistory` 组件适配 WeCom / QQ / QQ Bot 的平台标签、图标、配置字段
- **事件订阅扩展**：新增 `sprint.started` / `sprint.completed` / `review.requested` / `workflow.failed` 等事件类型，前后端同步
- **DingTalk ActionCard 支持**：Bridge 端 DingTalk 适配器补齐 ActionCard 渲染路径，后端 rendering profile 增加 DingTalk card 能力声明
- **Review 命令集成**：Bridge 命令引擎补齐 `/review deep` / `/review approve` / `/review request-changes` 子命令，调用后端 review handler
- **Native 降级上报统一**：所有平台适配器在 structured → text fallback 时统一上报 `X-IM-Downgrade-Reason` 元数据，后端 delivery 记录持久化该字段
- **前端投递详情增强**：`IMMessageHistory` 增加降级原因列、payload 预览抽屉、重试按钮
- **组件复用**：提取共享的 `PlatformBadge` / `EventBadgeList` 组件，IM 页面与 Settings / Notification 区域复用

## Capabilities

### New Capabilities

- `im-frontend-full-platform-coverage`: 前端 IM 管理界面支持全部 8 个平台的配置、状态展示和投递历史
- `im-delivery-diagnostics`: 投递降级上报、payload 预览、重试能力

### Modified Capabilities

- `im-platform-native-interactions`: 增加 DingTalk ActionCard 渲染路径与能力声明
- `im-provider-rendering-profiles`: DingTalk rendering profile 升级，补齐 card/update 能力
- `im-rich-delivery`: delivery 记录增加 downgrade_reason 字段，支持前端展示
- `bridge-provider-support`: provider descriptor 声明补齐 WeCom/QQ/QQ Bot 完整能力矩阵

## Impact

- **前端**：`lib/stores/im-store.ts`、`components/im/*`、`app/(dashboard)/im/page.tsx`、`components/shared/` (新增复用组件)
- **后端 Go**：`src-go/internal/handler/im_handler.go`、`im_control_handler.go`、`src-go/internal/model/im.go`（delivery 模型扩展）
- **Bridge (Go)**：`src-im-bridge/platform/dingtalk/`、`src-im-bridge/commands/review.go`、各平台 `live.go` 降级上报
- **i18n**：`messages/` 下的 IM 相关翻译 key 扩展
- **API 契约**：无 breaking change，均为增量字段/端点
