## 1. Task Command And Catalog Alignment

- [x] 1.1 对齐 `src-im-bridge/commands/task.go` 的 `/task` command catalog、usage、aliases 和 help 输出，使其与 proposal/spec 中的 canonical task surface 一致，并补齐 task identity 与 next-step guidance
- [x] 1.2 重构 task 响应构造逻辑，让 task status/create/move 等响应统一输出 canonical task summary，并在 card-capable provider 上生成 truthful 的 structured/native task payload
- [x] 1.3 为 task response 加入 provider-aware affordance gating，只在 runtime 真正 callback-ready 时暴露 callback-backed task actions，否则回退到 manual command guidance

## 2. Backend Task Lifecycle Action Execution

- [x] 2.1 在 `src-go/internal/service/im_action_execution.go` 中新增 canonical `transition-task` action（并兼容必要 alias），通过现有 task transition seam 执行真实状态流转
- [x] 2.2 扩展 task lifecycle IM action result shaping，使成功、blocked、failed 结果都返回 updated task identity、target status、和 reply-target-aware completion context
- [x] 2.3 更新 `src-go/internal/handler/im_control_handler.go`、binding helpers 与相关 DTO/tests，确保 IM-originated task lifecycle action 能稳定保留 task-scoped binding lineage

## 3. Bound Task Follow-Up Routing

- [x] 3.1 扩展 task transition 后的 follow-up 路径，使 IM-originated task action 可以按 `taskId` 复用现有 bound progress / terminal delivery seam
- [x] 3.2 对齐 `task_handler`、`task_workflow_service`、`im_control_plane` 之间的 payload 和 metadata，让 task/workflow verdict 通过 structured outcome 与 fallback reason 回到原 conversation
- [x] 3.3 为无 live bridge、binding 缺失、或 replay 恢复场景补齐 truthful blocked/retryable delivery 处理，避免静默换目标或重复 acceptance message

## 4. Provider-Aware Task Interaction

- [x] 4.1 更新 Feishu task interaction/rendering 路径，确保 task card callback 只在 callback-ready 时暴露并映射到 canonical backend task lifecycle action
- [x] 4.2 保持非 callback-ready 或 text-first provider 的 task affordance 降级真实，输出 task summary 与 manual guidance，而不是虚假按钮或平台特供命令
- [x] 4.3 对齐 task card action reference、provider metadata、和 rich-delivery fallback 说明，使 provider-native 与 structured follow-up 共享同一条 typed delivery truth

## 5. Verification

- [x] 5.1 为 `src-im-bridge` 补齐 `/task` command、task card affordance gating、以及 callback-backed task action 的 focused tests
- [x] 5.2 为 `src-go` 补齐 `transition-task` action execution、task-scoped binding、bound follow-up routing、以及 blocked delivery truth 的 focused tests
- [x] 5.3 运行 scoped verification（至少覆盖 `src-im-bridge` 与 `src-go` 的相关 Go tests），并 truthfully 记录仍未覆盖的 provider/runtime 边界
