## ADDED Requirements

### Requirement: IM Bridge 端到端测试流程文档
TESTING.md SHALL 新增 IM Bridge 端到端测试流程章节，包含：测试环境准备（Mock 平台 webhook）、消息收发验证、平台配置测试、Payload 格式验证、错误场景覆盖。

#### Scenario: 验证飞书消息收发
- **WHEN** 开发者需要测试飞书集成
- **THEN** 文档提供 Mock webhook 配置方法和消息发送验证步骤

#### Scenario: 验证多平台 Payload 格式
- **WHEN** 开发者修改了 IM Bridge 的消息格式
- **THEN** 文档提供各平台 Payload 格式的测试用例和验证方法
