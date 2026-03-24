## Context

`docs/PRD.md` 和 `docs/part/PLUGIN_SYSTEM_DESIGN.md` 都把 MCP 定位为 AgentForge 工具扩展的标准交互层，且明确了 `ToolPlugin -> MCP -> TS Bridge -> Go Orchestrator` 这一宿主分工。当前代码已经具备最小运行闭环：`src-bridge/src/mcp/client-hub.ts` 能连接 MCP server、列出工具并读取资源，`src-bridge/src/plugins/tool-plugin-manager.ts` 能注册/激活工具插件，Go 侧 `src-go/internal/service/plugin_service.go` 能持久化基础生命周期并通过 `POST /internal/plugins/runtime-state` 接收 Bridge 运行态同步。

但这条链路仍然停留在“插件能挂起来”的阶段。当前缺口主要有四类：

- Go 控制面没有统一的 MCP 交互代理，只能做注册、激活、健康检查和重启，无法代表操作员执行能力发现、资源读取、提示词查看或工具试调用。
- TS Bridge 的 MCP 运行态只保留 `discovered_tools` 和粗粒度健康状态，没有最近刷新时间、资源/提示词计数、最近一次交互摘要、失败分类等运维关键信息。
- 注册表中的 `PluginRuntimeMetadata` 仍偏向 Go/WASM 场景，无法沉淀 ToolPlugin 的 MCP 交互快照，导致 Go 很难成为真正权威的操作视图。
- 目前没有一组明确的审计与错误语义，区分“插件未激活”“能力发现失败”“工具调用参数不合法”“MCP server 已断连”等 operator-facing 结果。

这次设计要补的是 MCP 交互控制面，而不是整个插件生态重做。目标是在不改变现有宿主边界的前提下，把 MCP 插件从“可挂载”推进到“可观察、可验证、可诊断、可试操作”。

## Goals / Non-Goals

**Goals:**

- 保持 `Go 负责控制面 / TS Bridge 负责 MCP 执行` 的分层，不让前端或其他调用方直接绕开 Go 操作 Bridge。
- 为 ToolPlugin 增加完整的 MCP 能力发现面，至少覆盖工具、资源、提示词三类原语，以及按需刷新。
- 提供受控的 operator-facing MCP 交互 API，覆盖工具试调用、资源读取、提示词查看，并统一返回结构化结果与错误。
- 把 MCP 交互摘要同步回注册表和事件审计，让插件管理与后续自动化能力都能复用同一份运行态事实。
- 为这条能力补齐 focused tests 和文档，使仓库记录的 MCP 实际能力与文档口径一致。

**Non-Goals:**

- 不在这次变更里实现 WorkflowPlugin/ReviewPlugin 的执行引擎。
- 不在这次变更里补完整的 npm/git/catalog 安装链路或插件签名审核体系。
- 不把前端直接改成 Bridge 原生客户端；前端继续只经由 Go API 访问插件控制面。
- 不引入通用 JSON-RPC 透传接口，避免把 Go 控制面退化成无约束代理。

## Decisions

### 1. 保持 Go 为唯一 operator-facing MCP 控制面

Go 侧新增一组面向 ToolPlugin 的受控 API，由 `src-go/internal/bridge/client.go` 转发到 TS Bridge 的 typed MCP routes。前端、CLI 或未来 IM/Workflow 自动化一律通过 Go 调用这些接口，不直接连接 TS Bridge。

这样做的原因是：

- 认证、授权、审计、注册表同步已经都以 Go 为中心，继续保持单一权威面最稳妥。
- TS Bridge 仍然专注做 MCP client/runtime，不引入额外的产品级鉴权职责。
- 现有 `/internal/plugins/runtime-state` 已证明 Go<-TS 的状态回传通道成立，扩展 typed operator 操作比新增直连模式成本更低。

备选方案：

- 前端直接调用 TS Bridge：会绕开 Go 的鉴权和审计，也会制造第二套运行态事实。
- 把 MCP client 全部搬到 Go：会重复实现现有 TS MCP 能力，并削弱 `MCP First` 与 TS Bridge 的职责边界。

### 2. 用显式 typed operations 暴露 MCP 原语，而不是做通用透传

这次只开放明确的几类操作：

- 刷新并读取插件的工具/资源/提示词清单
- 读取单个资源
- 预览单个提示词定义或渲染结果
- 试调用指定工具

不提供“任意 JSON-RPC 方法透传”接口。原因是显式 API 更利于做权限校验、错误分类、输入验证、审计留痕和后续前端展示；同时避免把任意 MCP 方法暴露成难以维护的黑盒通道。

备选方案：

- 透传 JSON-RPC：短期开发快，但会让 Go 很难做稳定契约、结果摘要和安全边界。

### 3. 让 TS Bridge 维护“可同步的 MCP 交互快照”，Go 只持久化摘要

TS Bridge 作为 MCP 会话持有者，负责维护每个 ToolPlugin 的运行态快照，包括：

- transport 类型与连接状态
- 最近一次成功发现时间
- 工具、资源、提示词的计数与标识摘要
- 最近一次 operator-triggered 交互的类型、时间、结果状态与错误摘要

Go 注册表只持久化必要摘要，而不是完整保存任意资源内容或工具返回体。完整交互结果通过实时 API 返回给调用方，同时将摘要写入审计事件。

这样做的原因是：

- 大多数 MCP 结果体可能很大，也可能含敏感上下文，不适合直接沉淀到注册表。
- 注册表更适合记录“看板级”与“运维级”事实，而不是做结果仓库。

备选方案：

- 完整持久化所有 MCP 响应：审计和存储成本过高，也增加隐私泄露风险。

### 4. 把 operator-triggered 交互视为独立审计事件

人工触发的工具试调用、资源读取、提示词查看，都写成明确的插件事件，并附带：

- 交互类型
- 插件标识
- 目标工具/资源/提示词标识
- 成功/失败状态
- 截断后的结果摘要或错误摘要

这让后续插件面板、自动化恢复、故障回顾都能基于同一条事件流工作，而不是依赖临时日志。

备选方案：

- 仅更新最新状态、不落事件：无法回看历史，也不利于定位间歇性故障。

### 5. 错误模型按交互阶段分层返回

MCP 交互错误分成四类：

- 控制面前置失败：插件不存在、未启用、未激活、宿主不匹配
- 发现/连接失败：MCP server 无法连接、刷新失败、能力列表获取失败
- 输入验证失败：工具参数不满足 schema、缺少资源 URI、提示词参数不完整
- 运行时失败：工具执行报错、资源读取失败、server 断连

Go 与 TS 都使用结构化错误码和人类可读摘要，避免把裸异常文本直接当产品契约。

## Risks / Trade-offs

- [交互快照可能过期] → 通过 `last_discovery_at` 和显式 refresh API 让调用方知道当前快照是否新鲜，并允许按需刷新。
- [工具试调用可能触发真实副作用] → 只允许认证后的 operator-facing API 触发，执行前校验插件状态与参数结构，并把交互全量记入审计事件。
- [运行态元数据膨胀] → 注册表只保存摘要字段，详细内容留在 API 响应与事件摘要里，并对可变长文本做截断。
- [Go/TS 契约变复杂] → 把新能力收敛到 MCP 专用 typed DTO，而不是复用模糊的 `map[string]any`。
- [与现有插件管理变更并行时可能产生边界重叠] → 本次只补控制面与运行态契约，不改插件市场/面板的视觉与交互框架。

## Migration Plan

1. 先扩展 TS Bridge MCP hub 与 tool-plugin manager，补齐 prompts/resources discovery、interaction snapshot、typed MCP routes 和 reporter 载荷。
2. 再扩展 Go bridge client、plugin service、handler/routes 和 registry model，接入新操作与运行态摘要同步。
3. 最后补 focused tests、文档和必要的 operator-facing API 说明。

回滚策略：

- 这次设计以增量接口和增量元数据为主，旧的插件注册/启用/激活流程保持兼容。
- 如需回滚，可先停用新增 MCP interaction routes，再忽略新增 runtime metadata 字段，不影响现有基础生命周期能力。

## Open Questions

- 当前切片默认“提示词查看”至少支持发现与读取元数据；是否需要在同一轮里支持完整 prompt render，将以 MCP SDK 当前能力和输入参数模型为准。
- 若部分 MCP server 返回超大资源或工具结果，实现时需要确定统一截断上限；这不会阻塞规格编写，但应在 apply 阶段固化常量。
