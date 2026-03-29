## Why

当前 `app/(dashboard)/settings/page.tsx` 已经把多个设置区块堆到同一页，但前端工作区体验仍然不完整：页面缺少统一 dirty/reset 流程、字段级校验与失败反馈，operator diagnostics 也还没有把当前保存值、默认回退值和阻塞态区分清楚。现在补齐这条前端闭环，可以让已归档的 project settings contract 真正落到可用的设置工作区，而不是停留在“能提交 JSON，但不够可信和可操作”的半成品状态。

## What Changes

- 将项目设置页从“多卡片表单”补齐为完整的前端 settings workspace，增加统一的 dirty state、discard/reset、save pending、save success 与 save failure 反馈。
- 为 settings 表单增加前端可见的字段校验和错误呈现，覆盖预算阈值、review policy 组合、runtime/provider 选择、webhook 配置等关键输入，而不是只在提交后静默失败。
- 重做 operator summary / diagnostics 区块，让它明确反映当前选中值、服务器回退值、runtime 阻塞诊断、review posture 与 webhook readiness，而不是只重复静态 badge。
- 补齐针对 settings 页的 focused tests，覆盖 legacy fallback、dirty/reset、invalid input feedback、save payload 与 diagnostics rendering。

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `project-settings-control-plane`: 前端 settings workspace 必须提供统一编辑生命周期、可操作的校验反馈，以及基于当前设置状态的可信摘要/诊断，而不只是基础字段展示与保存。

## Impact

- Frontend: `app/(dashboard)/settings/page.tsx`, settings-related child components, `messages/*`, shared form/feedback UI.
- Client state: `lib/stores/project-store.ts` settings normalization and settings save error propagation.
- Tests: `app/(dashboard)/settings/page.test.tsx` and any focused settings workspace test helpers.
