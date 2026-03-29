## 1. 前端平台类型与共享组件

- [x] 1.1 扩展 `lib/stores/im-store.ts` 的 `IMPlatform` 类型，增加 `"wecom" | "qq" | "qqbot"`；`IMChannel` 增加 `platformConfig: Record<string, string>` 字段
- [x] 1.2 创建 `components/shared/platform-badge.tsx`（PlatformBadge 组件），定义 `PLATFORM_DEFINITIONS` 常量包含 8 平台的 label、icon、配置字段 schema
- [x] 1.3 创建 `components/shared/event-badge-list.tsx`（EventBadgeList 组件），渲染事件标签列表
- [x] 1.4 更新 `components/im/im-channel-config.tsx`：使用 `PLATFORM_DEFINITIONS` 驱动平台下拉列表，条件渲染平台特有配置字段（WeCom: corpId/agentId/callbackToken, QQ: onebot endpoint/access token, QQ Bot: appId/appSecret/webhookUrl）
- [x] 1.5 更新 `components/im/im-bridge-health.tsx`：使用 PlatformBadge 组件，增加 provider 能力摘要展示
- [x] 1.6 更新 `components/im/im-message-history.tsx`：使用 PlatformBadge，增加 downgrade_reason 列、payload 预览抽屉、失败重试按钮
- [x] 1.7 更新 `app/(dashboard)/im/page.tsx`：适配新组件 props 变化
- [x] 1.8 扩展 `messages/` i18n 文件，补齐 WeCom/QQ/QQ Bot 平台名称、新事件类型、降级原因等翻译 key

## 2. 前端事件类型动态加载

- [x] 2.1 在 `lib/stores/im-store.ts` 增加 `fetchEventTypes()` action，调用 `GET /api/v1/im/event-types`
- [x] 2.2 更新 `IMChannelConfig` 事件订阅区域，从 store 动态加载事件类型列表替代硬编码数组

## 3. 后端 delivery 扩展

- [x] 3.1 在 `src-go/internal/model/im.go` 的 delivery 相关结构体增加 `DowngradeReason string` 字段
- [x] 3.2 更新 `src-go/internal/handler/im_control_handler.go` 的 `AckDelivery()`，接收并持久化 `downgrade_reason`
- [x] 3.3 更新 `ListDeliveries()` 响应，包含 `downgrade_reason` 字段
- [x] 3.4 新增 `POST /im/deliveries/:id/retry` 端点，re-enqueue failed/timeout delivery，返回 409 对非失败状态
- [x] 3.5 在 `src-go/internal/server/routes.go` 注册 retry 路由

## 4. 后端事件类型端点

- [x] 4.1 在 `im_control_handler.go` 新增 `ListEventTypes()` handler，返回规范事件类型列表
- [x] 4.2 在 routes.go 注册 `GET /im/event-types` 路由（protected）

## 5. DingTalk ActionCard 支持

- [x] 5.1 在 `src-im-bridge/platform/dingtalk/live.go` 实现 `SendActionCard()` 方法，通过 DingTalk OpenAPI 发送 ActionCard
- [x] 5.2 更新 DingTalk rendering profile（`platform_metadata.go` 或 `rendering_profile.go`），声明 `card: true, cardUpdate: false`
- [x] 5.3 更新 DingTalk `NormalizeAction()` 处理 ActionCard callback，归一化为 `IMActionRequest`
- [x] 5.4 ActionCard 发送失败时降级为纯文本，设置 `X-IM-Downgrade-Reason: actioncard_send_failed`

## 6. Bridge 降级上报统一

- [x] 6.1 在 `src-im-bridge/core/delivery.go` 或各平台 `live.go` 的投递方法中，统一在 structured → text fallback 时设置 `X-IM-Downgrade-Reason` header
- [x] 6.2 更新 `src-im-bridge/client/control_plane.go` 的 ack 方法，携带 `downgrade_reason` 字段
- [x] 6.3 验证所有 8 平台的 fallback 路径都正确上报降级原因

## 7. Review 子命令集成

- [x] 7.1 在 `src-im-bridge/commands/review.go` 补齐 `/review deep <taskId>` 子命令处理，调用 `POST /api/v1/reviews`
- [x] 7.2 补齐 `/review approve <reviewId>` 子命令，调用 `POST /api/v1/reviews/:id/decide`
- [x] 7.3 补齐 `/review request-changes <reviewId> [reason]` 子命令，调用 decide 端点并传递 reason
- [x] 7.4 更新 `/review help` 输出，包含新子命令说明

## 8. Bridge provider capability 补齐

- [x] 8.1 更新 WeCom provider 的 `PlatformMetadata()` / capability descriptor，声明完整能力矩阵
- [x] 8.2 更新 QQ provider 的 capability descriptor
- [x] 8.3 更新 QQ Bot provider 的 capability descriptor
- [x] 8.4 验证 bridge 注册时上报的 capabilities 包含完整字段

## 9. 测试

- [x] 9.1 前端：为 IMChannelConfig 新增 WeCom/QQ/QQ Bot 平台选择和配置字段的单元测试
- [x] 9.2 前端：为 PlatformBadge 和 EventBadgeList 组件编写测试
- [x] 9.3 前端：为 IMMessageHistory 的降级列、重试按钮编写测试
- [x] 9.4 后端：为 AckDelivery 的 downgrade_reason 持久化编写测试
- [x] 9.5 后端：为 delivery retry 端点编写测试
- [x] 9.6 Bridge：为 DingTalk ActionCard 发送和降级路径编写 stub 测试
- [x] 9.7 Bridge：为 review 子命令解析和 API 调用编写测试
