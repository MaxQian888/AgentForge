## Why

`src-im-bridge` 现在已经能接收交互回调、绑定 reply target、发送 provider-native 通知，并通过控制面做注册、回放和进度续传，但仍有两个用户可见的断层没有补齐。第一，`/im/action` 到后端后大多还是占位式确认，任务分配、任务分解和审查按钮并没有稳定地驱动真实业务结果；第二，控制面和 bound-progress 队列目前仍把大多数异步投递压成纯文本，导致 rich card、structured payload 和 provider-native 更新在回放或异步通知场景里丢失保真度。

这两个缺口现在已经足够明确，继续只做 provider 覆盖或 reply-target 保留会开始产生“看起来支持、实际上不闭环”的假象。需要用一个新的 focused change，把交互动作的真实执行语义和 rich delivery 的可靠投递语义一起补齐，避免 IM Bridge 在核心任务流里继续停留在半闭环状态。

## What Changes

- 为共享 IM action 增加真实业务执行契约，覆盖任务分配并启动 Agent、任务分解、审查批准/要求修改等已暴露交互动作的后端执行与用户可见结果。
- 为 IM Bridge 增加 typed outbound delivery 契约，使 text、structured 和 provider-native payload 都能在控制面、compatibility `/im/send` 与 `/im/notify`、以及 bound progress/terminal update 路径中被保真投递或显式降级。
- 扩展控制面 delivery 模型与回放语义，使 replay-safe delivery 不再只支持纯文本 `content`，而是能携带 rich payload、reply-target 更新偏好和 operator-visible fallback metadata。
- 对齐 `src-im-bridge` 与 `src-go` 的动作/通知链路，让文档、配置样例和验证矩阵覆盖 action 闭环、rich replay、兼容 HTTP fallback 和平台差异化降级。

## Capabilities

### New Capabilities
- `im-action-execution`: 定义共享 IM action 如何映射到真实的任务、Agent、审查业务操作，以及成功、阻塞、失败时必须返回的用户可见结果。
- `im-rich-delivery`: 定义 IM Bridge 的 typed outbound delivery envelope，覆盖纯文本、structured payload、provider-native payload、fallback reason 和 replay-safe transport 语义。

### Modified Capabilities
- `im-bridge-control-plane`: 将控制面 delivery 从“签名文本投递”扩展为“签名且可回放的 typed delivery”，保证 rich payload 与 fallback metadata 在队列、回放和 ack 语义中保持一致。
- `im-platform-native-interactions`: 将平台交互要求从“规范化 callback”扩展为“规范化 callback 后必须驱动真实业务动作或返回显式终态失败”，避免继续停留在 placeholder acknowledgement。
- `im-bridge-progress-streaming`: 让长任务的 progress/terminal update 能通过 typed delivery 复用 rich/native/update path，而不是被强制压缩为新的纯文本消息。

## Impact

- Affected code: `src-go/internal/service/im_service.go`, `src-go/internal/service/im_control_plane.go`, `src-go/internal/model/im.go`, `src-go/internal/handler/im_handler.go`, `src-im-bridge/cmd/bridge`, `src-im-bridge/notify`, `src-im-bridge/client`, `src-im-bridge/core`, `src-im-bridge/commands`, 以及相关平台 adapter tests。
- Affected APIs: `/api/v1/im/action`, `/api/v1/im/send`, `/api/v1/im/notify`, `/api/v1/im/bridge/bind`, `/ws/im-bridge` 及其 delivery/ack payload。
- Affected systems: IM 交互按钮闭环、长任务进度回放、rich notification delivery、compatibility HTTP fallback、provider-native card/component/update path。
- Affected docs and ops: `src-im-bridge/README.md`, `.env.example`, smoke/manual verification docs，以及 action/rich delivery 相关排障说明。
