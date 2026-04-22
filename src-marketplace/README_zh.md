# AgentForge Marketplace

AgentForge 的独立 Go 微服务，负责插件、技能与角色的发布、发现、安装与评分。

## 技术栈

- **语言**: Go 1.25+
- **Web 框架**: Echo v4
- **数据库**: PostgreSQL
- **ORM**: GORM
- **迁移**: golang-migrate

## 目录结构

```
cmd/
  server/           # 服务入口
internal/
  handler/          # HTTP 处理器（items、versions、reviews、admin）
  service/          # 业务逻辑层
  repository/       # 数据访问层
  model/            # 领域模型
  config/           # 配置管理
  i18n/             # 国际化
migrations/         # 数据库迁移脚本
```

## 快速开始

```bash
# 直接运行（默认端口 7781）
go run ./cmd/server

# 运行测试
go test ./...

# 构建
go build ./cmd/server
```

## 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `SERVER_PORT` | 服务端口 | `7781` |
| `POSTGRES_URL` | PostgreSQL 连接字符串 | - |
| `JWT_SECRET` | JWT 签名密钥 | - |

## 核心概念

- **Item**:  marketplace 中的可发布实体，类型包括 `plugin`（插件）、`skill`（技能）、`role`（角色）。
- **Version**:  每个 Item 的版本历史，支持语义化版本号。
- **Review**:  用户对 Item 的评分与评论。
- **Consumption**: 主后端通过 `/api/v1/marketplace/install` 与 `/api/v1/marketplace/consumption` 桥接安装与消费状态。

## 安装流向

Marketplace 安装会物化到现有消费接缝：

- **插件** → 插件控制平面
- **角色** → 仓库本地角色存储
- **技能** → 权威角色技能目录

本地侧载在 marketplace 工作空间中目前复用插件本地安装接缝。不支持的角色/技能侧载流程保持显式拦截，而非假装成功。

## 前端集成

- 前端 Store: `lib/stores/marketplace-store.ts`
- 前端页面: `app/(dashboard)/marketplace/page.tsx`
- 前端组件: `components/marketplace/`
