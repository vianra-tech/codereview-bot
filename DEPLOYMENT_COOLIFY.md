# CodeReview.ai - Coolify Deployment Guide

## Overview
This guide deploys the complete CodeReview.ai platform to **Coolify** (self-hosted PaaS) using Docker Compose.

## Architecture on Coolify

```
┌─────────────────────────────────────────────────────────────┐
│                      Coolify Server                         │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐         │
│  │  Ingress    │  │  Analysis   │  │Aggregation  │         │
│  │  (Port 8080)│  │  (Worker)   │  │  (Worker)   │         │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘         │
│         │                │                │                 │
│         └────────────────┼────────────────┘                 │
│                          ▼                                  │
│              ┌─────────────────────┐                        │
│              │     NATS JetStream  │                        │
│              │    (Port 4222)      │                        │
│              └──────────┬──────────┘                        │
│                         │                                    │
│         ┌───────────────┼───────────────┐                   │
│         ▼               ▼               ▼                   │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐           │
│  │ Integration │ │   Admin     │ │  Dashboard  │           │
│  │   (Worker)  │ │  (Port 8081)│ │  (Port 3001)│           │
│  └─────────────┘ └─────────────┘ └─────────────┘           │
│         │               │               │                    │
│         ▼               ▼               ▼                    │
│  ┌─────────────────────────────────────────────┐           │
│  │           PostgreSQL (Supabase)             │           │
│  │         Redis + ClickHouse (Local)          │           │
│  └─────────────────────────────────────────────┘           │
└─────────────────────────────────────────────────────────────┘
```

---

## Prerequisites

1. **Coolify Instance** running (v4.0+)
2. **Domain** with DNS pointing to Coolify server
3. **GitHub App** created with credentials
4. **Supabase Project** with database credentials
5. **NVIDIA NIM API Key** (or self-hosted NIM)

---

## Step 1: Prepare Repository Structure

Ensure your repository has this structure:

```
codereview-bot/
├── docker-compose.coolify.yml    # Coolify-specific compose
├── docker-compose.yml            # Local development
├── services/
│   ├── ingress-svc/
│   ├── analysis-svc/
│   ├── aggregation-svc/
│   ├── integration-svc/
│   └── admin-svc/
├── dashboard/                    # Next.js app
├── migrations/                   # SQL migrations
└── .env.example                  # Template
```

---

## Step 2: Create Coolify Docker Compose

Create `docker-compose.coolify.yml`:

```yaml
version: '3.8'

services:
  # Message Queue
  nats:
    image: nats:2.10-alpine
    command: ["-js", "-m", "8222"]
    ports:
      - "4222:4222"
      - "8222:8222"
    volumes:
      - nats_data:/data
      - nats_config:/etc/nats
    healthcheck:
      test: ["CMD", "nats", "server", "check", "jetstream"]
      interval: 10s
      timeout: 5s
      retries: 5
    deploy:
      resources:
        limits:
          memory: 512M
        reservations:
          memory: 256M

  # Cache & Idempotency
  redis:
    image: redis:7-alpine
    command: redis-server --appendonly yes --maxmemory 256mb --maxmemory-policy allkeys-lru
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 10s
      timeout: 5s
      retries: 5
    deploy:
      resources:
        limits:
          memory: 512M
        reservations:
          memory: 256M

  # Analytics Database
  clickhouse:
    image: clickhouse/clickhouse-server:24.3
    ports:
      - "8123:8123"
      - "9000:9000"
    volumes:
      - clickhouse_data:/var/lib/clickhouse
    environment:
      CLICKHOUSE_DEFAULT_ACCESS_MANAGEMENT: 1
      CLICKHOUSE_USER: ${CLICKHOUSE_USER:-default}
      CLICKHOUSE_PASSWORD: ${CLICKHOUSE_PASSWORD}
    ulimits:
      nofile:
        soft: 262144
        hard: 262144
    healthcheck:
      test: ["CMD", "clickhouse-client", "--query", "SELECT 1"]
      interval: 10s
      timeout: 5s
      retries: 5
    deploy:
      resources:
        limits:
          memory: 2G
        reservations:
          memory: 1G

  # Object Storage (SARIF artifacts)
  minio:
    image: minio/minio:RELEASE.2024-01-16T16-07-38Z
    command: server /data --console-address ":9001"
    ports:
      - "9000:9000"
      - "9001:9001"
    volumes:
      - minio_data:/data
    environment:
      MINIO_ROOT_USER: ${MINIO_ROOT_USER:-minioadmin}
      MINIO_ROOT_PASSWORD: ${MINIO_ROOT_PASSWORD}
    healthcheck:
      test: ["CMD", "mc", "ready", "local"]
      interval: 10s
      timeout: 5s
      retries: 5
    deploy:
      resources:
        limits:
          memory: 512M

  # Ingress Service (Webhook Receiver)
  ingress-svc:
    build:
      context: .
      dockerfile: services/ingress-svc/Dockerfile
    ports:
      - "8080:8080"
    environment:
      - NATS_URL=nats://nats:4222
      - REDIS_URL=redis://redis:6379
      - GITHUB_WEBHOOK_SECRET=${GITHUB_WEBHOOK_SECRET}
      - LOG_LEVEL=info
      - PORT=8080
    depends_on:
      nats:
        condition: service_healthy
      redis:
        condition: service_healthy
    deploy:
      resources:
        limits:
          memory: 256M
        reservations:
          memory: 128M
    restart: unless-stopped

  # Analysis Service (CPU-intensive)
  analysis-svc:
    build:
      context: .
      dockerfile: services/analysis-svc/Dockerfile
    environment:
      - NATS_URL=nats://nats:4222
      - REDIS_URL=redis://redis:6379
      - NVIDIA_NIM_URL=${NVIDIA_NIM_URL}
      - NVIDIA_NIM_API_KEY=${NVIDIA_NIM_API_KEY}
      - NVIDIA_NIM_MODEL=${NVIDIA_NIM_MODEL:-meta/llama-3.1-70b-instruct}
      - WORK_DIR=/tmp/codereview
      - LOG_LEVEL=info
    volumes:
      - analysis_workdir:/tmp/codereview
    depends_on:
      nats:
        condition: service_healthy
      redis:
        condition: service_healthy
    deploy:
      resources:
        limits:
          memory: 2G
          cpus: '2'
        reservations:
          memory: 1G
          cpus: '1'
    restart: unless-stopped

  # Aggregation Service
  aggregation-svc:
    build:
      context: .
      dockerfile: services/aggregation-svc/Dockerfile
    environment:
      - NATS_URL=nats://nats:4222
      - REDIS_URL=redis://redis:6379
      - LOG_LEVEL=info
    depends_on:
      nats:
        condition: service_healthy
      redis:
        condition: service_healthy
    deploy:
      resources:
        limits:
          memory: 512M
        reservations:
          memory: 256M
    restart: unless-stopped

  # Integration Service (GitHub API)
  integration-svc:
    build:
      context: .
      dockerfile: services/integration-svc/Dockerfile
    environment:
      - NATS_URL=nats://nats:4222
      - REDIS_URL=redis://redis:6379
      - GITHUB_APP_ID=${GITHUB_APP_ID}
      - GITHUB_APP_PRIVATE_KEY=${GITHUB_APP_PRIVATE_KEY}
      - LOG_LEVEL=info
    depends_on:
      nats:
        condition: service_healthy
      redis:
        condition: service_healthy
    deploy:
      resources:
        limits:
          memory: 256M
        reservations:
          memory: 128M
    restart: unless-stopped

  # Admin Service (REST API)
  admin-svc:
    build:
      context: .
      dockerfile: services/admin-svc/Dockerfile
    ports:
      - "8081:8081"
    environment:
      - SUPABASE_DB_URL=${SUPABASE_DB_URL}
      - SUPABASE_URL=${SUPABASE_URL}
      - SUPABASE_KEY=${SUPABASE_KEY}
      - SUPABASE_JWT_SECRET=${SUPABASE_JWT_SECRET}
      - NATS_URL=nats://nats:4222
      - REDIS_URL=redis://redis:6379
      - CLICKHOUSE_URL=http://clickhouse:8123
      - LOG_LEVEL=info
      - PORT=8081
    depends_on:
      nats:
        condition: service_healthy
      redis:
        condition: service_healthy
      clickhouse:
        condition: service_healthy
    deploy:
      resources:
        limits:
          memory: 512M
        reservations:
          memory: 256M
    restart: unless-stopped

  # Dashboard (Next.js)
  dashboard:
    build:
      context: ./dashboard
      dockerfile: Dockerfile
    ports:
      - "3001:3000"
    environment:
      - NEXT_PUBLIC_API_URL=https://api.${DOMAIN}
      - NEXTAUTH_URL=https://app.${DOMAIN}
      - NEXTAUTH_SECRET=${NEXTAUTH_SECRET}
      - GITHUB_CLIENT_ID=${GITHUB_CLIENT_ID}
      - GITHUB_CLIENT_SECRET=${GITHUB_CLIENT_SECRET}
      - ADMIN_API_URL=http://admin-svc:8081
    depends_on:
      - admin-svc
    deploy:
      resources:
        limits:
          memory: 1G
        reservations:
          memory: 512M
    restart: unless-stopped

  # Migration Runner (one-shot)
  migrate:
    image: migrate/migrate:v4.20.1
    command: -path /migrations -database "${SUPABASE_DB_URL}" up
    volumes:
      - ./migrations:/migrations
    deploy:
      restart_policy:
        condition: on-failure
        max_attempts: 3

volumes:
  nats_data:
  nats_config:
  redis_data:
  clickhouse_data:
  minio_data:
  analysis_workdir:

networks:
  default:
    name: codereview-network
```

---

## Step 3: Create Dockerfiles for Each Service

### `services/ingress-svc/Dockerfile`
```dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.work go.work.sum ./
COPY services/ingress-svc/go.mod services/ingress-svc/go.sum ./services/ingress-svc/
COPY gen/go/go.mod ./gen/go/
RUN go work sync

COPY services/ingress-svc/ ./services/ingress-svc/
COPY gen/go/ ./gen/go/
WORKDIR /app/services/ingress-svc
RUN CGO_ENABLED=0 GOOS=linux go build -o /ingress-svc ./cmd/server

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /ingress-svc /ingress-svc
EXPOSE 8080
ENTRYPOINT ["/ingress-svc"]
```

### `services/analysis-svc/Dockerfile`
```dockerfile
FROM golang:1.22-alpine AS builder
RUN apk add --no-cache git gcc musl-dev
WORKDIR /app
COPY go.work go.work.sum ./
COPY services/analysis-svc/go.mod services/analysis-svc/go.sum ./services/analysis-svc/
COPY gen/go/go.mod ./gen/go/
RUN go work sync

COPY services/analysis-svc/ ./services/analysis-svc/
COPY gen/go/ ./gen/go/
WORKDIR /app/services/analysis-svc
RUN CGO_ENABLED=0 GOOS=linux go build -o /analysis-svc ./cmd/server

FROM alpine:3.19
RUN apk add --no-cache ca-certificates git
COPY --from=builder /analysis-svc /analysis-svc
WORKDIR /tmp/codereview
ENTRYPOINT ["/analysis-svc"]
```

### `services/aggregation-svc/Dockerfile`
```dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.work go.work.sum ./
COPY services/aggregation-svc/go.mod services/aggregation-svc/go.sum ./services/aggregation-svc/
COPY gen/go/go.mod ./gen/go/
RUN go work sync

COPY services/aggregation-svc/ ./services/aggregation-svc/
COPY gen/go/ ./gen/go/
WORKDIR /app/services/aggregation-svc
RUN CGO_ENABLED=0 GOOS=linux go build -o /aggregation-svc ./cmd/server

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /aggregation-svc /aggregation-svc
ENTRYPOINT ["/aggregation-svc"]
```

### `services/integration-svc/Dockerfile`
```dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.work go.work.sum ./
COPY services/integration-svc/go.mod services/integration-svc/go.sum ./services/integration-svc/
COPY gen/go/go.mod ./gen/go/
RUN go work sync

COPY services/integration-svc/ ./services/integration-svc/
COPY gen/go/ ./gen/go/
WORKDIR /app/services/integration-svc
RUN CGO_ENABLED=0 GOOS=linux go build -o /integration-svc ./cmd/server

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /integration-svc /integration-svc
ENTRYPOINT ["/integration-svc"]
```

### `services/admin-svc/Dockerfile`
```dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.work go.work.sum ./
COPY services/admin-svc/go.mod services/admin-svc/go.sum ./services/admin-svc/
COPY gen/go/go.mod ./gen/go/
RUN go work sync

COPY services/admin-svc/ ./services/admin-svc/
COPY gen/go/ ./gen/go/
WORKDIR /app/services/admin-svc
RUN CGO_ENABLED=0 GOOS=linux go build -o /admin-svc ./cmd/server

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /admin-svc /admin-svc
EXPOSE 8081
ENTRYPOINT ["/admin-svc"]
```

### `dashboard/Dockerfile`
```dockerfile
FROM node:20-alpine AS base
WORKDIR /app

FROM base AS builder
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM base AS runner
ENV NODE_ENV=production
COPY --from=builder /app/public ./public
COPY --from=builder /app/.next/standalone ./
COPY --from=builder /app/.next/static ./.next/static
EXPOSE 3000
ENV PORT=3000
CMD ["node", "server.js"]
```

**Note**: Add to `dashboard/next.config.js`:
```javascript
/** @type {import('next').NextConfig} */
const nextConfig = {
  output: 'standalone',
}
module.exports = nextConfig
```

---

## Step 4: Coolify Deployment Steps

### 1. Create New Project in Coolify
1. Open Coolify Dashboard → **Projects** → **New Project**
2. Name: `codereview-ai`
3. Connect your Git repository

### 2. Add Resources (in order)

#### A. Add NATS
- **Type**: Docker Compose
- **Source**: Select your repo
- **Compose File**: `docker-compose.coolify.yml`
- **Service**: `nats`
- **Ports**: 4222 (internal), 8222 (monitoring)

#### B. Add Redis
- Same compose, service: `redis`

#### C. Add ClickHouse
- Same compose, service: `clickhouse`
- **Warning**: Needs 2GB+ RAM

#### D. Add MinIO
- Same compose, service: `minio`

#### E. Add Ingress Service
- Same compose, service: `ingress-svc`
- **Port**: 8080
- **Domain**: `webhook.yourdomain.com`
- **Health Check**: `GET /healthz`

#### F. Add Analysis Service
- Same compose, service: `analysis-svc`
- **Resources**: 2 CPU, 2GB RAM minimum
- **No public port** (internal only)

#### G. Add Aggregation Service
- Same compose, service: `aggregation-svc`

#### H. Add Integration Service
- Same compose, service: `integration-svc`

#### I. Add Admin Service
- Same compose, service: `admin-svc`
- **Port**: 8081
- **Domain**: `api.yourdomain.com`

#### J. Add Dashboard
- **Type**: Docker Compose (or Static Site if you prefer)
- **Service**: `dashboard`
- **Port**: 3000
- **Domain**: `app.yourdomain.com`
- **Build Command**: `npm run build`
- **Start Command**: `npm start`

### 3. Configure Environment Variables

In Coolify, for **each service**, add these environment variables:

#### Global (All Services)
```bash
NATS_URL=nats://nats:4222
REDIS_URL=redis://redis:6379
LOG_LEVEL=info
```

#### Ingress Service
```bash
GITHUB_WEBHOOK_SECRET=this_is_gotya_from_vianra
PORT=8080
```

#### Analysis Service
```bash
NVIDIA_NIM_URL=https://integrate.api.nvidia.com/v1
NVIDIA_NIM_API_KEY=nvapi-xxxxxxxxxxxx
NVIDIA_NIM_MODEL=meta/llama-3.1-70b-instruct
WORK_DIR=/tmp/codereview
```

#### Integration Service
```bash
GITHUB_APP_ID=5041372
GITHUB_APP_PRIVATE_KEY="-----BEGIN RSA PRIVATE KEY-----\nMIIE...\n-----END RSA PRIVATE KEY-----"
```

#### Admin Service
```bash
SUPABASE_URL=https://kpdztmvwvdenvklwmflx.supabase.co
SUPABASE_DB_URL=postgresql://postgres:ClZRdc7vFHqrh8Jx@db.kpdztmvwvdenvklwmflx.supabase.co:5432/postgres?sslmode=require
SUPABASE_KEY=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
SUPABASE_JWT_SECRET=83TZBbdL3RdrA0XWEqTR9pRhBtERD/5KiDgx8x35VBDy3tF7xYyrVGAMw23vAHLZ1nUibO/2TjrpYIfDxtKEcw==
CLICKHOUSE_URL=http://clickhouse:8123
PORT=8081
```

#### Dashboard
```bash
NEXT_PUBLIC_API_URL=https://api.yourdomain.com
NEXTAUTH_URL=https://app.yourdomain.com
NEXTAUTH_SECRET=generate-with-openssl-rand-base64-32
GITHUB_CLIENT_ID=Iv23liru1HCEZfkHaK3D
GITHUB_CLIENT_SECRET=58500184b82bd2a2174a250a617e3d25e751430e
ADMIN_API_URL=http://admin-svc:8081
```

### 4. Configure Domains & SSL

In Coolify for each service with a domain:
1. Go to service → **Domains**
2. Add domain (e.g., `webhook.yourdomain.com`)
3. Enable **Force HTTPS**
4. Coolify will auto-provision Let's Encrypt SSL

**Required Domains:**
| Service | Domain | Purpose |
|---------|--------|---------|
| Ingress | `webhook.yourdomain.com` | GitHub webhook endpoint |
| Admin | `api.yourdomain.com` | REST API for dashboard |
| Dashboard | `app.yourdomain.com` | Admin UI |

### 5. Run Database Migrations

Option 1: **Coolify One-off Job**
- Create a "Job" in Coolify using the `migrate` service
- Run once after first deploy

Option 2: **Manual (via Coolify Terminal)**
```bash
# Connect to Coolify server
ssh root@your-coolify-server

# Run migration container
docker run --rm \
  -v /path/to/migrations:/migrations \
  migrate/migrate:v4.20.1 \
  -path /migrations \
  -database "postgresql://postgres:ClZRdc7vFHqrh8Jx@db.kpdztmvwvdenvklwmflx.supabase.co:5432/postgres?sslmode=require" \
  up
```

### 6. Configure GitHub App

1. Go to GitHub App settings → **General**
2. **Webhook URL**: `https://webhook.yourdomain.com/webhooks/github`
3. **Webhook Secret**: `this_is_gotya_from_vianra` (must match `GITHUB_WEBHOOK_SECRET`)
4. **Permissions**:
   - Repository: Contents (R), Metadata (R), Pull Requests (RW), Issues (RW), Checks (RW)
   - Organization: Members (R)
5. **Subscribe to events**: Push, Pull Request, Check Run, Check Suite
6. **Save** → Generate **Private Key** → Download `.pem`
7. Copy Private Key content to `GITHUB_APP_PRIVATE_KEY` env var

### 7. Verify Deployment

```bash
# Check all services are healthy
curl https://webhook.yourdomain.com/healthz
# {"status":"ok","service":"ingress-svc"}

curl https://api.yourdomain.com/healthz
# {"status":"ok","service":"admin-svc"}

# Check NATS monitoring
curl http://your-coolify-ip:8222/varz
```

### 8. Test End-to-End

1. Install GitHub App on a test repository
2. Push a commit with a known issue (e.g., hardcoded password)
3. Check PR → Should see "CodeReview.ai" check run
4. Check Files Changed → Should see inline annotations

---

## Step 5: Production Hardening

### Resource Limits (Adjust based on load)
```yaml
deploy:
  resources:
    limits:
      cpus: '4'
      memory: 4G
    reservations:
      cpus: '2'
      memory: 2G
```

### Backup Strategy
- **Supabase**: Built-in daily backups
- **ClickHouse**: Schedule `clickhouse-backup` to S3
- **NATS/Redis**: Low priority (ephemeral)

### Monitoring
- Coolify built-in metrics
- Add **Uptime Kuma** for external monitoring
- Alert on: Webhook failures, Analysis queue depth, GitHub API rate limits

### Scaling
- **Horizontal**: Increase replicas for `ingress-svc`, `analysis-svc`
- **Vertical**: Increase CPU/RAM for `analysis-svc` (LLM calls)
- **Queue**: NATS JetStream handles backpressure automatically

---

## Troubleshooting

| Issue | Solution |
|-------|----------|
| Webhook 401 | Verify `GITHUB_WEBHOOK_SECRET` matches GitHub App |
| Analysis OOM | Increase memory limit for `analysis-svc` |
| GitHub API 403 | Check App permissions & Private Key format |
| Dashboard 401 | Verify `NEXTAUTH_SECRET` and GitHub OAuth config |
| Migrations fail | Check Supabase IP allowlist includes Coolify server |

---

## Cost Estimation (Self-Hosted)

| Component | Monthly (Hetzner CX42) |
|-----------|------------------------|
| Server (8 vCPU, 16GB RAM) | ~€35 |
| Domain + SSL | ~€1 |
| Supabase (Pro) | $25 |
| NVIDIA NIM (API) | Pay-per-token |
| **Total** | **~€65-100/mo** |

---

## Support

- **Logs**: Coolify → Service → Logs
- **Metrics**: Coolify → Service → Metrics
- **Shell**: Coolify → Service → Terminal

**Deploy Order**: NATS → Redis → ClickHouse → MinIO → Ingress → Analysis → Aggregation → Integration → Admin → Dashboard → Migrations