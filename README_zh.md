# React + Go Quick Starter

一个现代化的全栈启动模板，结合了用于 Web 应用的 **Next.js 16** 与 **React 19**、**Go（Echo）** 后端（PostgreSQL + Redis），以及用于跨平台桌面应用的 **Tauri 2.9**。使用 TypeScript、Tailwind CSS v4 和 shadcn/ui 组件构建。

[English Documentation](./README.md)

## 特性

- ⚡️ **Next.js 16** 配合 App Router 和 React 19
- 🦫 **Go（Echo v4）** 后端，内置 JWT 认证、PostgreSQL、Redis 和 WebSocket
- 🖥️ **Tauri 2.9** 用于原生桌面应用（Windows、macOS、Linux）
- 🎨 **Tailwind CSS v4** 支持 CSS 变量和暗色模式
- 🧩 **shadcn/ui** 组件库，基于 Radix UI 原语
- 📦 **Zustand** 轻量级状态管理
- 🔤 **Geist 字体** 通过 next/font 优化
- 🎯 **TypeScript** 提供类型安全
- 🎭 **Lucide Icons** 精美的图标库
- 🗄️ **PostgreSQL** 数据库，支持自动迁移
- 🔴 **Redis** 用于令牌缓存和会话管理
- 📱 双重部署：从同一代码库部署 Web 应用或桌面应用

## 前置要求

### Web 开发所需

- **Node.js** 20.x 或更高版本（[下载](https://nodejs.org/)）
- **pnpm** 8.x 或更高版本（推荐）

  ```bash
  npm install -g pnpm
  ```

### 后端开发所需（额外要求）

- **Go** 1.22 或更高版本（[下载](https://go.dev/dl/)）
- **Docker**（通过 Docker Compose 运行 PostgreSQL + Redis）

  ```bash
  # 验证安装
  go version
  docker compose version
  ```

### 桌面开发所需（额外要求）

- **Rust** 1.77.2 或更高版本（[安装](https://www.rust-lang.org/tools/install)）

  ```bash
  # 验证安装
  rustc --version
  cargo --version
  ```

- **系统依赖**（因操作系统而异）：
  - **Windows**：Microsoft Visual Studio C++ 生成工具
  - **macOS**：Xcode 命令行工具
  - **Linux**：参见 [Tauri 前置要求](https://tauri.app/v1/guides/getting-started/prerequisites)

## 安装

1. **克隆仓库**

   ```bash
   git clone <your-repo-url>
   cd react-go-quick-starter
   ```

2. **安装前端依赖**

   ```bash
   pnpm install
   ```

3. **配置后端**

   ```bash
   cp src-go/.env.example src-go/.env
   # 按需编辑 src-go/.env
   ```

4. **启动后端依赖（PostgreSQL + Redis）**

   ```bash
   docker compose up -d
   ```

5. **验证安装**

   ```bash
   pnpm dev                          # 前端：http://localhost:3000
   cd src-go && go run ./cmd/server  # 后端：http://localhost:7777
   ```

## 开发

### 前端（Next.js）

```bash
pnpm dev      # 启动开发服务器，地址 http://localhost:3000
pnpm lint     # 运行 ESLint
pnpm test     # 运行 Jest 测试
```

### 后端（Go Echo）

```bash
# 先启动依赖服务
docker compose up -d

# 运行后端
cd src-go && go run ./cmd/server

# 或使用 Make
cd src-go && make run
cd src-go && make test
cd src-go && make build
```

后端启动于 `http://localhost:7777`，启动时自动执行数据库迁移。

### 桌面应用（Tauri + Go Sidecar）

```bash
# 为当前平台编译 Go 后端并启动 Tauri 开发模式
pnpm tauri:dev

# 分步执行：
pnpm build:backend:dev   # 仅为当前平台编译 Go sidecar
pnpm tauri dev           # 启动 Tauri 桌面应用
```

## 可用脚本

### 前端脚本

| 命令 | 描述 |
| --- | --- |
| `pnpm dev` | 在 3000 端口启动 Next.js 开发服务器 |
| `pnpm build` | 构建生产环境的 Next.js 应用（输出到 `out/`） |
| `pnpm start` | 启动 Next.js 生产服务器 |
| `pnpm lint` | 运行 ESLint |
| `pnpm test` | 运行 Jest 测试 |
| `pnpm test:watch` | 以监听模式运行测试 |
| `pnpm test:coverage` | 运行测试并生成覆盖率报告 |

### 后端脚本

| 命令 | 描述 |
| --- | --- |
| `pnpm build:backend` | 为所有平台交叉编译 Go sidecar |
| `pnpm build:backend:dev` | 仅为当前平台编译 Go sidecar（快速） |

### Tauri（桌面）脚本

| 命令 | 描述 |
| --- | --- |
| `pnpm tauri:dev` | 编译 Go sidecar + 启动 Tauri 开发模式 |
| `pnpm tauri:build` | 完整生产构建（Go + Next.js + Tauri 安装包） |
| `pnpm tauri info` | 显示 Tauri 环境信息 |

### 添加 UI 组件（shadcn/ui）

```bash
pnpm dlx shadcn@latest add button
pnpm dlx shadcn@latest add card dialog
```

## 项目结构

```text
react-go-quick-starter/
├── app/                      # Next.js App Router
│   ├── layout.tsx           # 根布局，包含字体和元数据
│   ├── page.tsx             # 主着陆页
│   ├── globals.css          # 全局样式和 Tailwind 配置
│   └── favicon.ico          # 应用图标
├── components/              # React 组件
│   └── ui/                  # shadcn/ui 组件（Button 等）
├── lib/                     # 工具函数
│   └── utils.ts            # 辅助函数（cn 等）
├── __tests__/               # Jest 测试（React Testing Library）
├── public/                  # 静态资源
├── scripts/
│   └── build-backend.sh    # 为 Tauri 交叉编译 Go sidecar
├── src-go/                  # Go Echo 后端
│   ├── cmd/server/          # 主入口
│   ├── internal/
│   │   ├── config/          # 从环境变量 / .env 文件加载配置
│   │   ├── handler/         # HTTP 处理器（auth、health、ws）
│   │   ├── middleware/       # JWT 中间件
│   │   ├── model/           # 领域模型（User）
│   │   ├── repository/      # 数据库 + 缓存访问层
│   │   ├── server/          # Echo 初始化和路由注册
│   │   ├── service/         # 业务逻辑（AuthService）
│   │   └── version/         # 构建版本信息
│   ├── migrations/          # 内嵌 SQL 迁移文件
│   ├── pkg/database/        # PostgreSQL + Redis 客户端
│   ├── Dockerfile           # 多阶段 Docker 镜像
│   ├── Makefile             # 构建、测试、Lint 快捷命令
│   ├── .env.example         # 环境变量模板
│   ├── go.mod
│   └── go.sum
├── src-tauri/              # Tauri 桌面应用
│   ├── binaries/           # 编译后的 Go sidecar 二进制文件
│   ├── src/
│   │   ├── main.rs         # Rust 主入口点
│   │   └── lib.rs          # Rust 库代码
│   ├── icons/              # 桌面应用图标
│   └── tauri.conf.json     # Tauri 配置
├── docker-compose.yml       # 本地开发用 PostgreSQL + Redis
├── components.json          # shadcn/ui 配置
├── next.config.ts          # Next.js 配置
├── tailwind.config.ts      # Tailwind CSS 配置
├── tsconfig.json           # TypeScript 配置
└── package.json            # Node.js 依赖和脚本
```

## API 接口

Go 后端在 `http://localhost:7777` 暴露以下接口：

| 方法 | 路径 | 鉴权 | 描述 |
| --- | --- | --- | --- |
| `GET` | `/health` | — | 健康检查（含版本信息） |
| `GET` | `/api/v1/health` | — | 版本化健康检查 |
| `POST` | `/api/v1/auth/register` | — | 注册新用户 |
| `POST` | `/api/v1/auth/login` | — | 登录，返回 JWT 令牌 |
| `POST` | `/api/v1/auth/refresh` | — | 刷新访问令牌 |
| `POST` | `/api/v1/auth/logout` | JWT | 登出并撤销令牌 |
| `GET` | `/api/v1/users/me` | JWT | 获取当前用户信息 |
| `GET` | `/ws` | — | WebSocket 连接 |

## 配置

### 后端环境变量（`src-go/.env`）

将 `src-go/.env.example` 复制为 `src-go/.env`：

```env
PORT=7777
POSTGRES_URL=postgres://dev:dev@localhost:5432/appdb?sslmode=disable
REDIS_URL=redis://localhost:6379
JWT_SECRET=change-me-in-production-use-at-least-32-chars
JWT_ACCESS_TTL=15m
JWT_REFRESH_TTL=168h
ALLOW_ORIGINS=http://localhost:3000,tauri://localhost
ENV=development
```

**重要提示**：

- 生产环境必须设置 `JWT_SECRET`（最少 32 个字符）。开发环境会使用默认值并输出警告。
- 切勿将 `src-go/.env` 提交到版本控制。

### 前端环境变量（`.env.local`）

```env
NEXT_PUBLIC_API_URL=http://localhost:7777
NEXT_PUBLIC_APP_NAME=React Go Quick Starter
```

### Tauri 配置

编辑 `src-tauri/tauri.conf.json` 以自定义桌面应用：

```json
{
  "productName": "react-go-quick-starter",
  "version": "0.1.0",
  "identifier": "com.reactgoquickstarter.desktop",
  "build": {
    "frontendDist": "../out",
    "devUrl": "http://localhost:3000"
  }
}
```

### 路径别名

在 `components.json` 和 `tsconfig.json` 中配置：

```typescript
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"
```

可用别名：`@/components`、`@/lib`、`@/ui`、`@/hooks`、`@/utils`

### Tailwind CSS 配置

项目使用 Tailwind CSS v4，具有以下特性：

- 使用 CSS 变量进行主题化（oklch 色彩空间），定义于 `app/globals.css`
- 通过 `class` 策略支持暗色模式
- shadcn/ui 样式系统

## 生产构建

### 构建 Web 应用

```bash
pnpm build
# 输出目录：out/
```

### 构建后端 Docker 镜像

```bash
cd src-go
docker build -t react-go-quick-starter-server .
```

### 构建桌面应用

```bash
# 完整生产构建：Go 二进制 + Next.js 静态导出 + Tauri 安装包
pnpm tauri:build

# 输出位置：
# - Windows: src-tauri/target/release/bundle/msi/
# - macOS:   src-tauri/target/release/bundle/dmg/
# - Linux:   src-tauri/target/release/bundle/appimage/
```

> **注意**：生产构建需要在 `next.config.ts` 中添加 `output: "export"`。

## 部署

### Web 部署

#### Vercel（推荐）

1. 将代码推送到 GitHub/GitLab/Bitbucket
2. 在 [Vercel](https://vercel.com/new) 上导入项目
3. Vercel 会自动检测 Next.js 并部署

#### 静态托管（Nginx、Apache 等）

```bash
pnpm build
# 将 out/ 目录上传到服务器
```

### 后端部署

Go 后端编译为单个静态二进制文件，可部署到任何支持 Go 的环境：

```bash
cd src-go
make build        # 输出 bin/server
./bin/server      # 监听 PORT（默认 7777）
```

或使用 Docker：

```bash
docker compose up   # 包含 postgres + redis
```

### 桌面部署

| 平台 | 制品 | 位置 |
| --- | --- | --- |
| Windows | `.msi` 安装包 | `src-tauri/target/release/bundle/msi/` |
| macOS | `.dmg` 文件 | `src-tauri/target/release/bundle/dmg/` |
| Linux | `.AppImage` | `src-tauri/target/release/bundle/appimage/` |

## 故障排除

### 端口 3000 已被占用

```bash
# Windows
netstat -ano | findstr :3000
taskkill /PID <PID> /F

# macOS/Linux
lsof -ti:3000 | xargs kill -9
```

### 后端启动失败

```bash
# 检查 Docker 服务是否运行
docker compose ps

# 重启依赖服务
docker compose down && docker compose up -d
```

### Tauri 构建失败

```bash
pnpm tauri info    # 检查环境
rustup update      # 更新 Rust
cd src-tauri && cargo clean
```

### 模块未找到错误

```bash
rm -rf .next
rm -rf node_modules pnpm-lock.yaml
pnpm install
```

## 了解更多

### Next.js 资源

- [Next.js 文档](https://nextjs.org/docs)
- [Next.js GitHub](https://github.com/vercel/next.js)

### Go 后端资源

- [Echo 框架](https://echo.labstack.com/)
- [golang-jwt/jwt](https://github.com/golang-jwt/jwt)
- [golang-migrate](https://github.com/golang-migrate/migrate)

### Tauri 资源

- [Tauri 文档](https://tauri.app/)
- [Tauri GitHub](https://github.com/tauri-apps/tauri)

### UI 和样式

- [shadcn/ui](https://ui.shadcn.com/)
- [Tailwind CSS](https://tailwindcss.com/docs)
- [Radix UI](https://www.radix-ui.com/)

## 贡献

欢迎贡献！请遵循以下步骤：

1. Fork 仓库
2. 创建功能分支（`git checkout -b feature/amazing-feature`）
3. 提交更改（`git commit -m 'feat: add amazing feature'`）
4. 推送到分支（`git push origin feature/amazing-feature`）
5. 打开 Pull Request

## 许可证

本项目是开源的，采用 [MIT 许可证](LICENSE)。

## 支持

- 查看[故障排除](#故障排除)部分
- 查阅 [Next.js 文档](https://nextjs.org/docs)
- 查阅 [Tauri 文档](https://tauri.app/)
- 在 GitHub 上提出 issue
