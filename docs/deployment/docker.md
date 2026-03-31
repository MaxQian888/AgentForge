# Docker Deployment Guide / Docker Compose 部署指南

This guide documents the Docker-backed deployment surfaces that already exist in
the repository.

## Source Files

- `docker-compose.yml`
- `docker-compose.dev.yml`
- `src-go/Dockerfile`

## Current Topology

### Base compose file

`docker-compose.yml` currently provisions infrastructure for local and server
bring-up:

- PostgreSQL 16 on `5432`
- Redis 7 on `6379`

The file also contains a commented-out `server` service that can be enabled if
you want the Go API inside the compose stack.

### Extended dev compose file

`docker-compose.dev.yml` adds:

- `bridge` on `7800`

This bridge container expects the Go backend and WebSocket endpoint to be
reachable via `host.docker.internal`.

## Quick Start

### Infrastructure only

```bash
docker compose up -d
```

### Infrastructure plus bridge

```bash
docker compose -f docker-compose.yml -f docker-compose.dev.yml up -d
```

## Volumes

Persistent volumes declared in the base compose file:

- `postgres_data`
- `redis_data`

## Health Checks

### PostgreSQL

- command: `pg_isready -U dev -d appdb`
- interval: `5s`
- retries: `5`

### Redis

- command: `redis-cli ping`
- interval: `5s`
- retries: `5`

## Containerized Go API

`src-go/Dockerfile` builds a minimal Alpine image:

1. `golang:1.25-alpine` downloads modules and builds `./cmd/server`
2. `alpine:3.21` copies the `server` binary
3. container exposes port `7777`

Manual build:

```bash
cd src-go
docker build -t agentforge-server .
```

## Compose Example Environment Variables

The commented `server` service uses:

```env
POSTGRES_URL=postgres://dev:dev@postgres:5432/appdb?sslmode=disable
REDIS_URL=redis://redis:6379
JWT_SECRET=change-me-in-production-at-least-32-chars
ENV=development
```

See [environment-variables.md](./environment-variables.md) for the full matrix.

## Production Notes

- the repository does not currently ship a production reverse proxy stack
- the recommended split is:
  - static frontend from `out/`
  - Go API on `:7777`
  - WebSocket endpoints on `/ws`
- front the API and websocket paths with a TLS terminator

See [tls.md](./tls.md) for reverse-proxy guidance.
