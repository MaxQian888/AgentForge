# React + Go Quick Starter

A modern, full-stack starter template combining **Next.js 16** with **React 19** for web applications, a **Go (Echo)** backend with PostgreSQL and Redis, and **Tauri 2.9** for cross-platform desktop applications. Built with TypeScript, Tailwind CSS v4, and shadcn/ui components.

[中文文档](./README_zh.md)

## Features

- ⚡️ **Next.js 16** with App Router and React 19
- 🦫 **Go (Echo v4)** backend with JWT auth, PostgreSQL, Redis, and WebSocket
- 🖥️ **Tauri 2.9** for native desktop applications (Windows, macOS, Linux)
- 🎨 **Tailwind CSS v4** with CSS variables and dark mode support
- 🧩 **shadcn/ui** component library with Radix UI primitives
- 📦 **Zustand** for lightweight state management
- 🔤 **Geist Font** optimized with next/font
- 🎯 **TypeScript** for type safety
- 🎭 **Lucide Icons** for beautiful iconography
- 🗄️ **PostgreSQL** database with auto-migrations
- 🔴 **Redis** for token cache and session management
- 📱 Dual deployment: Web app OR Desktop app from the same codebase

## Prerequisites

### For Web Development

- **Node.js** 20.x or later ([Download](https://nodejs.org/))
- **pnpm** 8.x or later (recommended)

  ```bash
  npm install -g pnpm
  ```

### For Backend Development (Additional Requirements)

- **Go** 1.22 or later ([Download](https://go.dev/dl/))
- **Docker** (for PostgreSQL + Redis via Docker Compose)

  ```bash
  # Verify installation
  go version
  docker compose version
  ```

### For Desktop Development (Additional Requirements)

- **Rust** 1.77.2 or later ([Install](https://www.rust-lang.org/tools/install))

  ```bash
  # Verify installation
  rustc --version
  cargo --version
  ```

- **System Dependencies** (varies by OS):
  - **Windows**: Microsoft Visual Studio C++ Build Tools
  - **macOS**: Xcode Command Line Tools
  - **Linux**: See [Tauri Prerequisites](https://tauri.app/v1/guides/getting-started/prerequisites)

## Installation

1. **Clone the repository**

   ```bash
   git clone <your-repo-url>
   cd react-go-quick-starter
   ```

2. **Install frontend dependencies**

   ```bash
   pnpm install
   ```

3. **Configure the backend**

   ```bash
   cp src-go/.env.example src-go/.env
   # Edit src-go/.env as needed
   ```

4. **Start backend dependencies (PostgreSQL + Redis)**

   ```bash
   docker compose up -d
   ```

5. **Verify installation**

   ```bash
   pnpm dev          # Frontend at http://localhost:3000
   cd src-go && go run ./cmd/server  # Backend at http://localhost:7777
   ```

## Development

### Frontend (Next.js)

```bash
pnpm dev      # Start dev server at http://localhost:3000
pnpm lint     # Run ESLint
pnpm test     # Run Jest tests
```

### Backend (Go Echo)

```bash
# Start dev dependencies first
docker compose up -d

# Run backend
cd src-go && go run ./cmd/server

# Or use Make
cd src-go && make run
cd src-go && make test
cd src-go && make build
```

The backend starts at `http://localhost:7777` and automatically runs database migrations on startup.

### Desktop Application (Tauri + Go sidecar)

```bash
# Compile Go backend for current platform and start Tauri dev mode
pnpm tauri:dev

# Or step by step:
pnpm build:backend:dev   # Compile Go sidecar for current platform
pnpm tauri dev           # Launch Tauri desktop app
```

## Available Scripts

### Frontend Scripts

| Command | Description |
| --- | --- |
| `pnpm dev` | Start Next.js dev server on port 3000 |
| `pnpm build` | Build Next.js app for production (outputs to `out/`) |
| `pnpm start` | Start Next.js production server |
| `pnpm lint` | Run ESLint |
| `pnpm test` | Run Jest tests |
| `pnpm test:watch` | Run tests in watch mode |
| `pnpm test:coverage` | Run tests with coverage report |

### Backend Scripts

| Command | Description |
| --- | --- |
| `pnpm build:backend` | Cross-compile Go sidecar for all platforms |
| `pnpm build:backend:dev` | Compile Go sidecar for current platform only (fast) |

### Tauri (Desktop) Scripts

| Command | Description |
| --- | --- |
| `pnpm tauri:dev` | Compile Go sidecar + start Tauri dev mode |
| `pnpm tauri:build` | Full production build (Go + Next.js + Tauri installer) |
| `pnpm tauri info` | Display Tauri environment information |

### Adding UI Components (shadcn/ui)

```bash
pnpm dlx shadcn@latest add button
pnpm dlx shadcn@latest add card dialog
```

## Project Structure

```text
react-go-quick-starter/
├── app/                      # Next.js App Router
│   ├── layout.tsx           # Root layout with fonts and metadata
│   ├── page.tsx             # Main landing page
│   ├── globals.css          # Global styles and Tailwind config
│   └── favicon.ico          # App favicon
├── components/              # React components
│   └── ui/                  # shadcn/ui components (Button, etc.)
├── lib/                     # Utility functions
│   └── utils.ts            # Helper functions (cn, etc.)
├── __tests__/               # Jest tests (React Testing Library)
├── public/                  # Static assets
├── scripts/
│   └── build-backend.sh    # Cross-compile Go sidecar for Tauri
├── src-go/                  # Go Echo backend
│   ├── cmd/server/          # Main entry point
│   ├── internal/
│   │   ├── config/          # Config from env / .env file
│   │   ├── handler/         # HTTP handlers (auth, health, ws)
│   │   ├── middleware/       # JWT middleware
│   │   ├── model/           # Domain models (User)
│   │   ├── repository/      # DB + cache access layer
│   │   ├── server/          # Echo setup and route registration
│   │   ├── service/         # Business logic (AuthService)
│   │   └── version/         # Build version info
│   ├── migrations/          # Embedded SQL migrations
│   ├── pkg/database/        # PostgreSQL + Redis clients
│   ├── Dockerfile           # Multi-stage Docker image
│   ├── Makefile             # Build, test, lint shortcuts
│   ├── .env.example         # Environment variable template
│   ├── go.mod
│   └── go.sum
├── src-tauri/              # Tauri desktop application
│   ├── binaries/           # Compiled Go sidecar binaries
│   ├── src/
│   │   ├── main.rs         # Rust main entry point
│   │   └── lib.rs          # Rust library code
│   ├── icons/              # Desktop app icons
│   └── tauri.conf.json     # Tauri configuration
├── docker-compose.yml       # PostgreSQL + Redis for local dev
├── components.json          # shadcn/ui configuration
├── next.config.ts          # Next.js configuration
├── tailwind.config.ts      # Tailwind CSS configuration
├── tsconfig.json           # TypeScript configuration
└── package.json            # Node.js dependencies and scripts
```

## API Endpoints

The Go backend exposes the following endpoints at `http://localhost:7777`:

| Method | Path | Auth | Description |
| --- | --- | --- | --- |
| `GET` | `/health` | — | Health check with version info |
| `GET` | `/api/v1/health` | — | Versioned health check |
| `POST` | `/api/v1/auth/register` | — | Register new user |
| `POST` | `/api/v1/auth/login` | — | Login, returns JWT tokens |
| `POST` | `/api/v1/auth/refresh` | — | Refresh access token |
| `POST` | `/api/v1/auth/logout` | JWT | Logout and revoke token |
| `GET` | `/api/v1/users/me` | JWT | Get current user profile |
| `GET` | `/ws` | — | WebSocket connection |

## Configuration

### Backend Environment (`src-go/.env`)

Copy `src-go/.env.example` to `src-go/.env`:

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

**Important**:

- `JWT_SECRET` is required in production (min 32 characters). In development, a default is used with a warning.
- Never commit `src-go/.env` to version control.

### Frontend Environment (`.env.local`)

```env
NEXT_PUBLIC_API_URL=http://localhost:7777
NEXT_PUBLIC_APP_NAME=React Go Quick Starter
```

### Tauri Configuration

Edit `src-tauri/tauri.conf.json` to customize your desktop app:

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

### Path Aliases

Configured in `components.json` and `tsconfig.json`:

```typescript
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"
```

Available aliases: `@/components`, `@/lib`, `@/ui`, `@/hooks`, `@/utils`

### Tailwind CSS Configuration

The project uses Tailwind CSS v4 with:

- CSS variables for theming (oklch color space) in `app/globals.css`
- Dark mode via `class` strategy
- shadcn/ui styling system

## Building for Production

### Build Web Application

```bash
pnpm build
# Output: out/
```

### Build Backend Docker Image

```bash
cd src-go
docker build -t react-go-quick-starter-server .
```

### Build Desktop Application

```bash
# Full production build: Go binary + Next.js static export + Tauri installer
pnpm tauri:build

# Output locations:
# - Windows: src-tauri/target/release/bundle/msi/
# - macOS:   src-tauri/target/release/bundle/dmg/
# - Linux:   src-tauri/target/release/bundle/appimage/
```

> **Note**: Production builds require `output: "export"` in `next.config.ts`.

## Deployment

### Web Deployment

#### Vercel (Recommended)

1. Push your code to GitHub/GitLab/Bitbucket
2. Import project on [Vercel](https://vercel.com/new)
3. Vercel auto-detects Next.js and deploys

#### Static Hosting (Nginx, Apache, etc.)

```bash
pnpm build
# Upload out/ directory to your server
```

### Backend Deployment

The Go backend ships as a single static binary. Deploy it anywhere Go binaries run:

```bash
cd src-go
make build        # outputs bin/server
./bin/server      # runs on PORT (default 7777)
```

Or use Docker:

```bash
docker compose up   # includes postgres + redis
```

### Desktop Deployment

| Platform | Artifact | Location |
| --- | --- | --- |
| Windows | `.msi` installer | `src-tauri/target/release/bundle/msi/` |
| macOS | `.dmg` file | `src-tauri/target/release/bundle/dmg/` |
| Linux | `.AppImage` | `src-tauri/target/release/bundle/appimage/` |

## Troubleshooting

### Port 3000 already in use

```bash
# Windows
netstat -ano | findstr :3000
taskkill /PID <PID> /F

# macOS/Linux
lsof -ti:3000 | xargs kill -9
```

### Backend fails to start

```bash
# Check Docker services are running
docker compose ps

# Restart dependencies
docker compose down && docker compose up -d
```

### Tauri build fails

```bash
pnpm tauri info    # Check environment
rustup update      # Update Rust
cd src-tauri && cargo clean
```

### Module not found errors

```bash
rm -rf .next
rm -rf node_modules pnpm-lock.yaml
pnpm install
```

## Learn More

### Next.js Resources

- [Next.js Documentation](https://nextjs.org/docs)
- [Next.js GitHub](https://github.com/vercel/next.js)

### Go Backend Resources

- [Echo Framework](https://echo.labstack.com/)
- [golang-jwt/jwt](https://github.com/golang-jwt/jwt)
- [golang-migrate](https://github.com/golang-migrate/migrate)

### Tauri Resources

- [Tauri Documentation](https://tauri.app/)
- [Tauri GitHub](https://github.com/tauri-apps/tauri)

### UI & Styling

- [shadcn/ui](https://ui.shadcn.com/)
- [Tailwind CSS](https://tailwindcss.com/docs)
- [Radix UI](https://www.radix-ui.com/)

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'feat: add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is open source and available under the [MIT License](LICENSE).

## Support

- Check the [Troubleshooting](#troubleshooting) section
- Review [Next.js Documentation](https://nextjs.org/docs)
- Review [Tauri Documentation](https://tauri.app/)
- Open an issue on GitHub
