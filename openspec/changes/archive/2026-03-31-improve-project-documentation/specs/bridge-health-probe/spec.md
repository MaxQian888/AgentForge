## ADDED Requirements

### Requirement: Bridge 测试矩阵文档
TESTING.md SHALL 新增 Bridge 测试矩阵章节，以表格形式展示：Bridge 类型 × 测试维度（单元测试、集成测试、E2E 测试），标注每种组合的覆盖状态和运行命令。

#### Scenario: 查看当前 Bridge 测试覆盖情况
- **WHEN** 开发者需要了解哪些 Bridge 已有测试覆盖
- **THEN** 测试矩阵表格清晰展示各 Bridge 的测试状态和运行命令

### Requirement: 覆盖率提升指南
TESTING.md SHALL 新增覆盖率提升指南，包含：当前各模块覆盖率数据、未覆盖场景识别方法、测试编写建议（优先级排序）。

#### Scenario: 为低覆盖模块添加测试
- **WHEN** 开发者需要提高某个模块的测试覆盖率
- **THEN** 指南提供优先级排序和测试编写模板
