# Task 1 Brief — Toolchain + repo scaffold (from docs/superpowers/plans/2026-09-08-atlas.md)

## Global Constraints (bind this task)

- Go >= 1.23.
- All new files live under `Desktop/attendance/`. Every `git add` MUST scope
  pathspecs to `Desktop/attendance/...` — NEVER run bare `git add -A`.

## Environment

- OS win32, shell PowerShell 5.1. Chain dependent commands as
  `cmd1; if ($?) { cmd2 }`. Do NOT `cd`; pass `workdir` instead.
- `winget` v1.29.290 available. No Go, Docker, or Postgres installed.
- Node v26 and Python 3.12 exist (irrelevant to this task).

## Task

**Files:**
- Create: `api/go.mod`
- Create: `.env.example`
- Create: `.gitignore`

**Interfaces:**
- Consumes: nothing.
- Produces: `module atlas` importable; env keys `DATABASE_URL, REDIS_ADDR, JWT_SECRET, PORT, UPLOAD_DIR, S3_*`.

- [ ] **Step 1: Install toolchain and verify**

```bash
winget install --id GoLang.Go -e --silent
winget install --id Docker.DockerDesktop -e --silent
```

Run: `go version` Expected: `go1.23+`. Run: `docker --version` Expected:
Docker version line. If Docker Desktop demands a reboot/WSL setup you cannot
complete, do NOT reboot — finish the Go-side steps, commit them, and report
DONE_WITH_CONCERNS noting Docker needs a reboot by the human.

- [ ] **Step 2: Create go.mod**

```go
module atlas

go 1.23
```

File path: `C:\Users\amirn\Desktop\attendance\api\go.mod`.
Run `go vet ./...` in `api/` — Expected: PASS (no output).

- [ ] **Step 3: Create .env.example** at `C:\Users\amirn\Desktop\attendance\.env.example`

```bash
PORT=8080
DATABASE_URL=postgres://atlas:atlas@postgres:5432/atlas?sslmode=disable
REDIS_ADDR=redis:6379
JWT_SECRET=change-me-to-32-bytes-minimum
UPLOAD_DIR=/data/uploads
S3_ENDPOINT=
S3_BUCKET=
S3_ACCESS_KEY=
S3_SECRET_KEY=
```

- [ ] **Step 4: Create .gitignore** at `C:\Users\amirn\Desktop\attendance\.gitignore`

```bash
.env
data/
uploads/*
!uploads/.gitkeep
```

- [ ] **Step 5: Commit** (repo root is `C:\Users\amirn`)

```bash
git add Desktop/attendance/api/go.mod Desktop/attendance/.env.example Desktop/attendance/.gitignore
git commit -m "chore(atlas): scaffold toolchain and env"
```
