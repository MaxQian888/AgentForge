## Context

AgentForge 的 TS Bridge 已经把 `opencode` 纳入 runtime registry，并且 `src-bridge/src/handlers/execute.test.ts` 也覆盖了“OpenCode 请求会走 command runtime adapter”的路径；但当前实现本质上只是把 OpenCode 当成和 Codex 相同的泛化命令型 runtime。

仓库真相里的关键断点有三处：

- `src-bridge/src/handlers/command-runtime.ts` 假设命令型 runtime 会从 `stdin` 接收一份 Bridge 私有 JSON 请求，再从 `stdout` 输出逐行 JSON 事件。这个协议对仓库自定义 runner 成立，但并不是 OpenCode 官方对外承诺的自动化接口。
- `src-bridge/src/server.ts` 的 pause/cancel/resume 语义仍是 Bridge 本地控制：pause 直接 kill 当前进程并保存 `snapshot.request`，resume 再把同一份 execute 请求重新送回 `handleExecute(...)`。这对 OpenCode 会导致“重新跑一遍请求”而不是真正继续同一上游 session。
- `src-bridge/src/runtime/registry.ts` 对 `opencode` 的 readiness 只检查命令能否解析，无法判断 OpenCode server 是否可达、是否需要 basic auth、provider/model 是否真的可用，也无法提前暴露 session transport 层面的阻塞原因。

与此同时，OpenCode 官方当前文档明确提供三条自动化集成面：`opencode serve` HTTP API、`opencode run --format json` 和 `opencode acp`。对于 AgentForge 当前已经存在的 Hono HTTP bridge 来说，`opencode serve` 的会话 API、`/event` SSE、`/global/health` 和 `/session/:id/abort` 更适合作为长期稳定的 OpenCode runtime control plane。

## Goals / Non-Goals

**Goals:**
- 让 `opencode` runtime 通过 OpenCode 官方自动化接口真实执行，而不是继续依赖 Bridge 私有 stdin 协议
- 为 `opencode` 执行补齐真实的 session binding、cancel、pause、resume 和状态同步语义
- 让 runtime catalog/readiness diagnostics 能提前报告 OpenCode server reachability、认证、provider/model 可用性等真实阻塞点
- 保持 Go 侧继续走现有 `/bridge/*` canonical contract，不把本次 change 扩展成新的 Go/API/UI 功能波次
- 为 OpenCode transport、事件归一化、恢复流程和失败模式补齐 focused tests 与运维文档

**Non-Goals:**
- 不重写 `claude_code` 或 `codex` 的 adapter 实现
- 不在本次 change 中扩展新的前端设置面板或新的 Go REST route
- 不把所有 OpenCode 自动化入口都做成并列实现；本次只定义并落地 Bridge 的 canonical 集成方式
- 不尝试在本次 change 中建设完整的多实例 OpenCode fleet 管理或远程租户编排

## Decisions

### 1. `opencode serve` HTTP API 作为 Bridge 的 canonical OpenCode transport

Bridge 对 OpenCode 的正式集成改为基于 `opencode serve` 暴露的 HTTP/OpenAPI 接口，至少覆盖：

- `/global/health` 用于 readiness 和版本探针
- `/session` / `/session/:id` 用于创建和恢复 session
- `/session/:id/message` 或 `/session/:id/prompt_async` 用于提交执行请求
- `/session/:id/abort` 用于 cancel/pause
- `/event` 或与 session 相关的事件流用于获取执行中的输出和状态变化

这样选的原因：

- 当前 TS Bridge 自己就是 HTTP server，接 OpenCode server 比继续包装一个 Bridge 私有 stdin 协议更自然
- OpenCode 官方文档已经把 server API 作为 programmatic interaction 的主入口，pause/cancel/resume 语义也更完整
- `opencode run --format json` 更适合脚本或一次性 automation，`opencode acp` 更适合编辑器/IDE 对接；二者都不如 HTTP control plane 便于 Bridge 维持长生命周期任务

备选方案：

- 继续保留当前泛化命令协议。否决，因为这不是 OpenCode 官方承诺的接口，resume/cancel 语义也不真实。
- 以 `opencode run --format json --attach ...` 作为主实现。否决，因为虽然它比私有 stdin 协议更真实，但对于 Bridge 这种长生命周期服务来说，session 控制和 abort/reconcile 仍不如直接用 server API 明确。
- 以 `opencode acp` 作为主实现。暂不采用，因为 ACP 更偏 editor client 场景，当前 Bridge 侧已有成熟 HTTP 契约，优先降低架构摩擦。

### 2. 为 OpenCode runtime 引入上游 session binding，而不是只保存本地 `snapshot.request`

Bridge 现有 `SessionManager` 只保存 Bridge 自己的 `SessionSnapshot`。对 OpenCode 来说，这不足以表达“继续同一个上游会话”。本次设计要求新增 OpenCode continuity metadata，至少记录：

- 绑定的 OpenCode session ID
- 最近一次驱动的 message / prompt cursor（如果上游 API 区分）
- 当前 transport endpoint 与必要的 auth context 标识
- 最后一次已确认同步的上游状态时间戳

resume 时，Bridge 必须基于这份 binding 继续原有 OpenCode session，而不是把原始 prompt 重新当成一条新 execute 请求再发一遍。这样才能避免重复改动工作区、重复工具调用、重复成本计费和错误的“恢复成功”假象。

备选方案：

- 继续只保存原始 execute payload。否决，因为这只是重放，不是恢复。
- 把 OpenCode session ID 只放在内存里。否决，因为 Bridge 重启、pause 后 resume、诊断导出都会丢失真正的 continuity identity。

### 3. OpenCode 事件采集与控制面解耦：HTTP control plane + session-aware event normalization

OpenCode integration 需要拆成两层：

- Control plane：建 session、发 prompt、abort、query status、校验 health
- Event plane：消费 OpenCode 官方事件流或消息结果，把上游内容映射到 Bridge 现有 `output` / `tool_call` / `tool_result` / `cost_update` / `error` / `status_change`

Bridge 不需要把 OpenCode 的原生事件原样泄漏给 Go；Go 依旧只消费 canonical `AgentEvent`。但 Bridge 内部必须保留一层 OpenCode-specific normalizer，负责把 session/message event 转成当前统一事件模型，并在事件流断开时做状态 reconcile，而不是直接把断流当完成。

备选方案：

- 只靠 synchronous message API，不接事件流。否决，因为这会退化成“等任务全部跑完才拿结果”，丢失实时输出与 tool activity。
- 直接把 OpenCode 原生 event payload 透传给 Go。否决，因为这会破坏 Bridge 统一事件契约。

### 4. Pause 与 Cancel 语义需要区分“停止当前生成”与“丢弃恢复能力”

对 OpenCode 来说，pause 与 cancel 不能再都退化成“本地 kill 进程”：

- `cancel`: 终止当前上游 session 的执行并清理 Bridge resumable binding，不再允许后续 resume
- `pause`: 终止当前生成，但保留 resumable binding，使后续 `/bridge/resume` 可以继续同一个 OpenCode session
- `resume`: 使用保存的上游 session identity 继续执行，而不是新建 session 或重放完整 prompt

如果 OpenCode 上游没有“原地暂停继续生成”的一等原语，则 Bridge 的 truthful 语义应当是“abort current run, keep session continuity, later continue within same session”。这比伪造一个假 pause 更符合用户真实预期。

备选方案：

- 把 pause 和 cancel 都映射成同一个上游 abort。否决，因为 resume 语义会失真。
- 把 pause 实现成仅 Bridge 本地状态切换，不触碰上游。否决，因为上游仍会继续跑，状态会漂移。

### 5. Runtime diagnostics 升级为真实 transport readiness，而不是只看命令存在

`bridge-agent-runtime-registry` 里对 `opencode` 的 readiness 判断要从“`which opencode` 能找到命令”升级为一组可操作诊断：

- server URL 是否配置或可解析到默认值
- `/global/health` 是否可达
- 是否需要 basic auth，当前凭据是否有效
- provider/default model 是否能从上游 provider/config API 解析
- 当前 runtime 是否具备执行与 resume 所需的会话接口

这些 diagnostics 一方面服务 runtime catalog，另一方面也服务 execute 前校验，避免 catalog 说可用、execute 时才发现 server 不通或 model 不存在。

备选方案：

- catalog 继续只做静态命令检查，剩下错误留给 execute。否决，因为这无法支持“功能完整”的事前诊断。

## Risks / Trade-offs

- [Risk] OpenCode server 的事件 payload 可能比当前文档描述更细或未来调整字段名 -> Mitigation: 以官方 `/doc` OpenAPI 和 focused fixture tests 作为实现真相源，normalizer 只依赖稳定字段并对未知事件做可观测降级
- [Risk] 事件流断连会导致 Bridge 提前把任务判成完成或失败 -> Mitigation: 断流后必须做 session/message 状态 reconcile，再决定 terminal status
- [Risk] 当前用户环境可能只有 `opencode` CLI，没有长期运行的 server -> Mitigation: diagnostics 明确提示 server 前置要求，并保留是否需要 managed bootstrap 的开放问题，不在本次设计里模糊化为“看起来能跑”
- [Risk] pause 在 OpenCode 上可能只能语义化为 “abort current response but keep session” -> Mitigation: 在 spec 和文档里明确这一语义，避免误称为底层原生暂停
- [Risk] `SessionSnapshot` 增加 OpenCode continuity metadata 后，旧快照或其他 runtime 路径可能出现兼容性问题 -> Mitigation: 采用可选字段扩展，不破坏 Claude/Codex 现有 snapshot 读取路径

## Migration Plan

1. 为 Bridge 增加 OpenCode transport client、配置解析和 basic auth 支持，先让 readiness/health 可以独立验证
2. 扩展 OpenCode runtime continuity metadata，并让 pause/resume/cancel 走新的上游 session control path
3. 用新的 OpenCode adapter 替换当前 registry 中的泛化 command adapter 接入，同时保留 Codex 继续走现有 command path
4. 补齐 runtime registry、OpenCode adapter、server route 的 focused tests，再更新 README 与本地验证步骤
5. scoped verification 通过后，再决定是否需要单独追加“Bridge managed OpenCode server bootstrap” change

回滚策略：

- 如果 OpenCode official transport 集成在实现期出现不可接受的不确定性，可以先保留现有 `opencode` runtime 注册但将其显式标记为 unavailable，并通过 diagnostics 说明原因，避免继续提供一个看似可用但实际不真实的 runtime
- 新增的 OpenCode continuity metadata 采用可选字段扩展，必要时可回滚到仅本地 snapshot 而不影响 Claude/Codex 路径

## Open Questions

- 本次是否需要 Bridge 负责自动拉起 `opencode serve`，还是先要求显式提供 `OPENCODE_SERVER_URL` 并把 managed bootstrap 留到后续 focused change
- OpenCode `/event` 与 session/message 事件里哪些字段在当前版本最稳定，是否需要在实现前先从 `/doc` 生成 typed client 或 fixture
- Go 侧是否需要看到额外的 resumability metadata，还是仅 Bridge 内部持有上游 session binding 即可
