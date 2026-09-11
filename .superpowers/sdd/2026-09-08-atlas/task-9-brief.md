# Task 9 Brief: Docker Compose + Nginx + tuning

## Plan source
docs/superpowers/plans/2026-09-08-atlas.md, Task 9 (verbatim below).

## Task text (single source of requirements — exact values to use verbatim)

**Files:**
- Create: `docker-compose.yml`
- Create: `nginx/nginx.conf`
- Create: `api/Dockerfile`

Compose services with mem_limit 1024m/100m/128m/32m; postgres `shared_buffers=256MB max_connections=100`; redis `maxmemory 96mb allkeys-lru`; nginx proxies /v1/ to api:8080, serves web/ static; api multi-stage build. Volumes postgres-data, redis-data, uploads.

- [ ] **Step 1: Write the three files**
- [ ] **Step 2: Boot + migrate + seed**
Run: `docker compose up -d --build` then migration + seed. Expected: `docker compose ps` all healthy.
- [ ] **Step 3: Commit** `feat(atlas): compose stack with memory budget`.

## Current state (controller-observed, replaces any guessing)
- BASE commit: 06a0b31 (Task 8 complete).
- All three files ALREADY EXIST as untracked files (a prior partial attempt). Do NOT recreate blindly — read and verify each against the spec above, fix only what deviates:
  - `docker-compose.yml` (65 lines): postgres:16 mem 1024m shared_buffers=256MB max_connections=100; redis:7 mem 128m maxmemory 96mb allkeys-lru; api build context . dockerfile api/Dockerfile mem 100m, env PORT/DATABASE_URL/REDIS_ADDR/JWT_SECRET/UPLOAD_DIR/SEED=true; nginx:alpine mem 32m ports 8080:80, mounts ./nginx/nginx.conf, ./web, uploads.
  - `api/Dockerfile` (12 lines): FROM golang:1.26-alpine AS build, COPY api/go.mod api/go.sum, go mod download, COPY api/, CGO_ENABLED=0 go build -o /atlas .; FROM alpine:3.21 + ca-certificates.
  - `nginx/nginx.conf` (33 lines): upstream api:8080, /v1/ proxy, /uploads/ alias, / static root.
- `web/` does NOT exist yet (Task 10). Nginx mounts `./web:/usr/share/nginx/html:ro` — with no web/ dir, compose may fail or serve empty. Handle minimally: create empty `web/.gitkeep` ONLY if required for compose to start; do NOT build Task 10 pages.
- Migration `migrations/001_init.sql` is NOT re-runnable (no IF NOT EXISTS) — single-apply only. Check whether volume pgdata already has tables before applying; if `docker compose ps` shows fresh volumes, apply once via `docker compose exec` or psql against localhost.
- Seed is AUTOMATIC: `api/main.go` runs `store.Seed(db)` when `SEED=true` (already set in compose). No manual seed script needed — verify via users count = 5 after boot.
- `.env` exists locally with JWT_SECRET (32+ bytes). Compose uses `${JWT_SECRET}` — ensure container gets it (docker compose reads `.env` automatically).
- Docker is HEALTHY: Docker 29.7.2, Compose v5.5.1, `docker info` works. User fixed the prior docker problem.

## GUARDS (violations broke the repo before — follow exactly)
- Repo root IS C:\Users\amirn\Desktop\attendance. NEVER run `git init`. NEVER touch anything outside this root.
- Work from C:\Users\amirn\Desktop\attendance.
- Commits scoped to relative pathspecs ONLY, e.g. `git add docker-compose.yml api/Dockerfile nginx/nginx.conf` (and web/.gitkeep if created). NEVER `git add -A`, NEVER include `.superpowers/`, `android-mcp.log`, or anything outside the three files.
- Working on master by established pattern (ledger documents in-place work; worktree would duplicate home checkout).

## Report contract
Write full report to: C:\Users\amirn\Desktop\attendance\.superpowers\sdd\2026-09-08-atlas\task-9-report.md
- What you verified/changed per file
- `docker compose ps` output (all healthy expected)
- Migration evidence (single-apply, \dt or users count)
- Seed evidence (SELECT count(*) FROM users = 5)
- Files changed + self-review findings
Then reply with ONLY: Status (DONE/DONE_WITH_CONCERNS/BLOCKED/NEEDS_CONTEXT), commits (short SHA + subject), one-line test summary, concerns, report path.
