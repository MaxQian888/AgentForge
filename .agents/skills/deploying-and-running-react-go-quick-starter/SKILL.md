---
name: deploying-and-running-react-go-quick-starter
description: Use whenever you need to run, package, build, or deploy this repository's Next.js 16 frontend, Go sidecar, Tauri desktop shell, Docker-backed local services, or GitHub Actions release flow. Make sure to apply this skill when choosing between web mode, backend mode, desktop mode, static export delivery, or release artifacts, even if the user only mentions a single surface like 'start the backend' or 'build the desktop app'.
---

# Deploying And Running React Go Quick Starter

## Overview

Use this skill to choose the correct runtime or deployment path for this repository without falling back to generic Next.js, Go, or Tauri advice that does not match the current project setup.

Keep the repo truth anchored in `package.json`, `next.config.ts`, `scripts/build-backend.sh`, `src-go/.env.example`, `src-tauri/tauri.conf.json`, `src-tauri/src/lib.rs`, and `.github/workflows/*`.

## Start Here

- First classify the task: web-only local dev, backend-only local dev, desktop dev, static web deployment, desktop release, or CI/CD triage.
- Then read only the reference file that matches the task:
  - `references/local-run-matrix.md` for local dev and build entrypoints
  - `references/deployment-surfaces.md` for static deployment and GitHub Actions flows
  - `references/runtime-contracts-and-pitfalls.md` for ports, sidecar packaging, and command mismatches
- If the task changes product code after the runtime path is understood, also load the domain skill that matches the changed surface:
  - Next.js app work: `$next-best-practices`
  - Go backend or sidecar contract work: `$echo-go-backend`
  - Tauri shell/config work: `$tauri-v2`

## Operating Rules

- Treat `pnpm build` as a static export that writes `out/`.
- Do not recommend `pnpm start` as the production runtime for this repo. `next start` is incompatible with `output: "export"`.
- For backend-backed local work, start Postgres and Redis first unless the user explicitly says those services already exist elsewhere.
- On Windows, expect `pnpm build:backend` and `pnpm build:backend:dev` to require `bash`. If `/bin/bash` is missing, fall back to direct Go commands in `src-go` and explain that Tauri sidecar packaging still depends on the script's output contract.
- Keep the port contract aligned across frontend, Go, and Tauri. Browser mode falls back to `NEXT_PUBLIC_API_URL` and then `http://localhost:7777`; desktop mode asks Rust for the backend URL; Rust sidecar startup currently binds port `7777`.
- Prove claims with the narrowest command first, then escalate to broader builds only when needed.

## Quick Checks

- Web dev: `pnpm dev`
- Static web build: `pnpm build`
- Backend deps: `docker compose up -d postgres redis`
- Backend direct run: `cd src-go && go run ./cmd/server`
- Backend direct build fallback: `cd src-go && go build ./cmd/server`
- Desktop dev happy path: `pnpm tauri:dev`
- Desktop release happy path: `pnpm tauri:build`
- CI/CD entrypoints: `.github/workflows/ci.yml`, `.github/workflows/build-tauri.yml`, `.github/workflows/deploy.yml`, `.github/workflows/release.yml`

## Common Mistakes

- Following generic Next.js guidance and suggesting `pnpm start` after `pnpm build`.
- Assuming `pnpm tauri dev` alone is the repo's preferred desktop entrypoint. In this repo, the helper path is `pnpm tauri:dev`.
- Treating Vercel preview or production deploys as enabled by default. They are not.
- Changing backend port, binary naming, or build output without updating the frontend hook, Rust sidecar, Tauri config, and backend build script together.
