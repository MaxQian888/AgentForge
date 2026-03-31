## 1. Canonical Command Catalog

- [x] 1.1 在 src-im-bridge/commands 中引入统一的 command catalog 或等价元数据层，描述命令族、子命令、alias、usage 和说明。
- [x] 1.2 将现有 task、agent、review、sprint、cost、help 的注册与 usage 输出迁移到 catalog 驱动的实现，保持现有行为兼容。
- [x] 1.3 让 /help 与 mention fallback 的命令建议从同一份 catalog 生成，消除静态帮助文本与实际 handler 的漂移。

## 2. AgentForge Client Surface Expansion

- [x] 2.1 在 src-im-bridge/client/agentforge.go 中补齐 agents 的 get、pause、resume、kill 与 tasks 的状态流转 client methods 和测试模型。
- [x] 2.2 补齐项目 queue 的 list、cancel，以及 project team 或 member summary 所需的 client methods。
- [x] 2.3 补齐项目 memory search 与 lightweight note store 所需的 client methods，并约定 IM 侧默认 scope 或 category 规则。

## 3. Operator Command Implementation

- [x] 3.1 扩展 /agent 命令族，补齐 canonical 的 status、pause、resume、kill，并保留 /agent list 兼容 alias。
- [x] 3.2 扩展 /task 命令族，补齐 canonical 的状态流转子命令，并保留兼容写法而不破坏 create、list、status、assign、decompose。
- [x] 3.3 新增 /queue list 或 cancel、/team list，以及 /memory search 或 note 命令处理与 IM 友好回复格式。
- [x] 3.4 统一各命令的成功、失败、not found、conflict 和 blocked 回复语义，使其与 backend API 结果一致且适合 IM 阅读。

## 4. Docs And Smoke Alignment

- [x] 4.1 更新 docs/PRD.md、src-im-bridge/README.md 与 src-im-bridge/docs/platform-runbook.md 中的 IM 命令矩阵，使其和 canonical catalog 一致。
- [x] 4.2 刷新 src-im-bridge/scripts/smoke 下的代表性平台 fixtures 或说明，覆盖新增命令族与 alias 口径。
- [x] 4.3 对齐 help 文本、README 示例和 runbook 验证步骤，确保 review、sprint、agent control、queue、team、memory 不再出现各写各的情况。

## 5. Verification

- [x] 5.1 为新增或变更的 commands 与 client methods 补齐单元测试，覆盖成功路径、usage 错误、404、409 和 alias 兼容。
- [x] 5.2 运行 src-im-bridge 的 focused go test 套件，至少覆盖 commands、client、core 和受影响的平台入口。
- [x] 5.3 用更新后的 smoke fixtures 或 runbook 步骤验证至少一条 slash command、一个兼容 alias 和一条 mention fallback 建议路径。
