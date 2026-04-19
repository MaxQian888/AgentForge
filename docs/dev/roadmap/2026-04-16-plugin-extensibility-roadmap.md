---
title: AgentForge 插件可扩展性 Roadmap
date: 2026-04-16
status: approved
owner: Max Qian
tracks: 6
---

# AgentForge 插件可扩展性 Roadmap

## 为什么做

AgentForge 已完成从"3 角色编码团队"到"通用 DAG 工作流引擎"的重构（Phase 1-5 合入）。
但当前插件系统只能扩展"外挂能力"（调用外部工具、跑外部脚本），**无法扩展工作流引擎本体**：
新节点类型、Function 实现、执行拦截点、前端自定义 UI、Skill 中心化、开发者脚手架全部缺位。

**目标**：让第三方（以及未来的内部业务线）仅通过插件就能把 AgentForge 压成领域专用平台
（电商发品、会计月结、内容生产、客服分流等），**无需改动核心代码**。

## 本 roadmap 不做什么

- 不保证向后兼容：项目处内部测试，所有契约随时 breaking
- 不引入新语言/新 runtime：只在现有 Go/TS/WASM/MCP 边界内扩展
- 不做跨插件编排（pub/sub、RPC）：插件仍通过工作流引擎协作
- 不引入插件市场商业化（定价/许可/DRM）：marketplace 结构已存在，商业层另议

## Tracks

| ID | 名称 | 目标 | 影响栈 | 成功标准（粗） |
|----|------|------|--------|----------------|
| **A** | DAG 节点类型注册表 | `dag_workflow_service.go` 的 switch/case 替换为 `NodeTypeRegistry`；13 个内置节点类型全部注册；支持插件在 bootstrap 时注册自定义 `node.type`；同步下线 `TeamWorkflowAdapter` 和老 team 策略。 | Go core | (1) registry 接口稳定；(2) 内置类型全迁移且测试通过；(3) TeamService 相关代码删除；(4) 示例自定义节点可跑通。 |
| **B** | Function handler 注册表 | `function` 节点从"内联 executor"改为"按 id 查 registry"；插件可注册具名函数（输入/输出 JSON Schema + 实现）。 | Go core | (1) `FunctionRegistry` 接口；(2) 一组内置函数迁入注册表；(3) WASM/MCP 插件都能贡献 function。 |
| **C** | 执行 Hook 系统 | 暴露 `OnNodeStart / OnNodeComplete / OnNodeError / OnAsyncCallback` 四个拦截点；插件可注册 hook 做审计、计费、合规、观测。 | Go core | (1) Hook 链式组合、可短路；(2) hook 失败不拖垮 execution；(3) 至少一个参考插件（审计日志）。 |
| **D1** | 前端扩展槽 | 插件可贡献：自定义节点配置表单、节点结果可视化、Dashboard 小部件。前端通过动态加载机制读取插件清单并渲染。 | Next 前端 + Go manifest 扩展 | (1) 扩展点注册表；(2) 动态加载+沙箱化；(3) 至少一个参考前端插件。 |
| **D2**（并行） | Skill registry | Skill 从 "RoleManifest 里的 path 字符串" 升级为一等实体：独立注册表、版本、依赖、发现 API。 | Go core + RoleManifest schema | (1) Skill 模型+表+REST；(2) RoleManifest 迁移为按 id 引用；(3) Marketplace `skill` item 通过 registry 安装。 |
| **D3** | 脚手架 `pnpm create-plugin` | 兑现 `docs/guides/plugin-development.md` 已承诺但未实现的 CLI；覆盖 tool / review / workflow / integration / frontend（D1 落地后）。 | scripts/ + TS CLI | (1) 5 种模板可跑；(2) 生成物直接通过 `plugin install` 落库；(3) 文档同步更新。 |

## 顺序与并行

```
主线（串行，每条合入才开下一条）：
  A → B → C → D1 → D3

并行支线：
  D2（与主线解耦，任何时候可启动/合入）
```

**主线串行的理由**：A 提供的 `NodeTypeRegistry` 基础设施会被 B 复用；C 的 hook 需要覆盖 A 注册的自定义类型才有意义；D1 需要 A/B 的自定义节点存在才有渲染对象；D3 的模板需要 A/B/C/D1 都定型再写。

## 横切议题（所有 track 共享的约定）

### 契约风格

- 所有新增接口标注 `// experimental: pre-1.0, may change without notice`
- 不提供迁移 shim、不写 deprecation warning、不引入 feature flag
  （项目处内部测试阶段，破坏性更改自由）
- 每条 track 合入时，允许直接删除老代码路径

### 权限模型

- 所有注册表（NodeType / Function / Hook / Frontend slot）必须继承插件的 `capabilities` 声明
  ——插件注册的条目只能用 manifest 里已声明的能力，registry 在注册时校验
- 未声明却调用 = registry 拒绝注册并记入 plugin_events

### 观测

- 所有注册表写入 → 一条 `plugin_events` 审计记录（`event_type: registry_entry_added / registry_entry_removed`）
- 所有 registry 查找失败 → 结构化错误（`code: REGISTRY_MISS`）
- 自定义节点执行时长、失败率走现有 `workflow_node_execution` 指标通道，不另起

### 测试策略

每条 track 必须包含：
1. Go/TS 单元测试覆盖 registry CRUD 与错误路径
2. 一个 end-to-end 参考插件（真实可运行，不是 mock）
3. 迁移后的回归测试——原有 workflow 用例必须全绿

### 文档

- 每条 track 合入时，`docs/guides/plugin-*.md` 同步更新
- D3 合入时重写整个 `plugin-development.md`，统一术语

## 工作方式

每条 track 独立走完：

```
Brainstorm → Spec → Plan → Implement → Commit → 回到 Brainstorm 下一条
```

A 合入之前不启动 B 的 brainstorm；D2 可独立于这条主线启动。

## 合入验收

每条 track 合入 master 需满足：

- CI 全绿（`pnpm lint`、`pnpm test`、`pnpm exec tsc --noEmit`、`cd src-go && go test ./...`）
- 参考插件可跑通
- 本 roadmap 对应条目标记 `[Done]`
- MEMORY 更新：完成态 + 合约决策

## 进度跟踪

| Track | Spec | Plan | Impl | Merged |
|-------|------|------|------|--------|
| A  | ☐ | ☐ | ☐ | ☐ |
| B  | ☐ | ☐ | ☐ | ☐ |
| C  | ☐ | ☐ | ☐ | ☐ |
| D1 | ☐ | ☐ | ☐ | ☐ |
| D2 | ☐ | ☐ | ☐ | ☐ |
| D3 | ☐ | ☐ | ☐ | ☐ |

每完成一个里程碑，打勾并附链接到对应 spec/plan/PR。
