# database-schema-docs Specification

## Purpose
Define the database and Redis documentation contract for AgentForge so developers can inspect PostgreSQL relationships, table/index ownership, and Redis key and TTL strategy from repository documentation instead of scattered implementation details.
## Requirements
### Requirement: PostgreSQL 表结构文档
系统 SHALL 在 `docs/schema/postgres.md` 中提供 PostgreSQL 数据库的完整表结构文档，包含：ER 关系图（Mermaid 格式）、每张表的字段列表（名称、类型、约束、默认值、说明）、索引列表、外键关系。

#### Scenario: 开发者理解用户与项目的关系
- **WHEN** 开发者打开 `docs/schema/postgres.md`
- **THEN** 文档包含 ER 图展示 users、projects、tasks、worktrees 等核心表之间的关系，以及每张表的完整字段说明

#### Scenario: 开发者查询索引定义
- **WHEN** 开发者需要优化查询性能
- **THEN** 文档列出所有索引定义，包含索引名、关联表、字段组合、索引类型

### Requirement: Redis 缓存策略文档
系统 SHALL 在 `docs/schema/redis.md` 中提供 Redis 的数据结构文档，包含：键名格式规范、数据类型、TTL 策略、用途说明。覆盖 token 黑名单、refresh token 存储、会话缓存等。

#### Scenario: 开发者理解 token 黑名单机制
- **WHEN** 开发者打开 `docs/schema/redis.md`
- **THEN** 文档包含 token 黑名单的键名格式（如 `token:blacklist:{jti}`）、TTL 策略（与 token 过期时间一致）、数据结构（SET 或 STRING）

#### Scenario: 开发者排查缓存一致性问题
- **WHEN** 开发者需要理解 Redis 中存储的会话数据
- **THEN** 文档列出所有 Redis key pattern 及其用途、TTL 和清理策略
