# Task 8 Report — HTTP wiring + main

## Status: DONE

## What was built
- `api/internal/httpapi/server.go` (new): `Server` struct
  (`DB`, `Limiter`, `JWTSecret`, `Store`), `NewServer`, `Routes()` on
  stdlib Go 1.22+ ServeMux with `r.PathValue("id")`. Helpers `writeJSON`,
  `writeErr`, `requireRole`-style `prot()` middleware over
  `auth.Middleware` (login registered public; every other route wrapped
  per-route). All 17 routes from the brief implemented with the specified
  error envelope `{"error":"CODE"}`, role gates (403 `FORBIDDEN`), and
  scan error mapping (401 `INVALID_CODE`, 403 `OUT_OF_GEOFENCE` +
  `distance_m`, 429 `RATE_LIMITED`, 410 `SESSION_CLOSED`). Uploads capped
  with `http.MaxBytesReader` 6MB; `Store.Save` enforces 5MB + MIME via
  `storage.CheckUpload`.
- `api/internal/httpapi/server_test.go` (new): 6 tests — login
  success (token verified via `auth.Parse`) + bad password 401; scan
  401/403+distance_m/429 mappings; role gate 403 + anonymous 401; `/me`
  row. httptest + sqlmock + `MemLimiter` only, no live services.
- `api/main.go` (new): env contract `PORT` (8080), `DATABASE_URL`,
  `REDIS_ADDR`, `JWT_SECRET` (fatal if <32 bytes), `UPLOAD_DIR`
  (./uploads); `store.Open`, redis Ping, `SEED=="true"` → `store.Seed`,
  `attend.NewRedisLimiter(rdb, 10, time.Minute)`,
  `storage.LocalStorage{Dir, BaseURL:"/uploads"}`,
  `http.ListenAndServe(":"+port)`.
- `api/internal/attend/attend.go`: added `SessionSecret(db, sessionID)`
  (`SELECT totp_secret ... WHERE id=$1`, no rows → `ErrSessionClosed`).
- `api/internal/attend/attend_test.go`: added `TestSessionSecret`
  (happy path + no-rows → `ErrSessionClosed`).

## Test-first evidence
- Step 1: wrote `server_test.go` before any impl → `go test`
  FAIL `undefined: Server` / `undefined: NewServer` (as expected).
- Step 2: implemented `server.go` + `main.go` + accessor.
- Step 3: `go test ./internal/httpapi/ ./internal/attend/ -v` → all
  13 tests PASS (6 httpapi + 7 attend).

## Verification
- `go test ./...` → PASS (all packages ok).
- `gofmt -l .` → clean. `go vet ./...` → clean. `go build ./...` → PASS.

## Commit
- `c8dacda feat(atlas): http wiring and service entrypoint`
- Scoped exactly per GIT GUARD:
  `git add api/internal/httpapi api/main.go api/internal/attend`
  (5 files: server.go, server_test.go, main.go new; attend.go,
  attend_test.go modified).

## Concerns / judgment calls
- `[rep,admin]` gates accept `class_rep` (+ harmless `rep` alias) and
  `admin`, matching the DB role CHECK constraint.
- Unspecified mappings chosen: upload oversize → 413 `FILE_TOO_LARGE`,
  bad MIME → 400 `INVALID_FILE_TYPE`, bad absence reason → 400
  `BAD_REASON`, review of non-pending → 409 `NOT_PENDING`, bad review
  decision → 400. Missing `session_id` query param → 400.
- `GET /v1/analytics/college` returns the `ClassRate` shape directly
  (`{present,total,pct}`); class analytics returns `{rate, categories}`.
- All list endpoints return `[]` (never null) on empty results.
- No NEEDS_CONTEXT issues: all brief interfaces matched the code as read
  (`report.CollegeRate` returns `ClassRate` — never referenced as a type).

## Fix report (2026-09-08) — absence list role gate

- Root cause: `GET /v1/absences` used `prot()` with no role gate, so any
  authenticated role (incl. `class_rep`) could list all absences.
- Changed `api/internal/httpapi/server.go`: gated route to
  `prot(s.handleAbsenceList, "teacher", "admin", "student")`.
  Students remain self-filtered by existing `student_id` logic.
- Added `TestAbsenceListGate` in `server_test.go`: `class_rep` token on
  `GET /v1/absences` → 403 `FORBIDDEN`.
- Verification: `go test ./internal/httpapi/ -v` → all 7 tests PASS;
  `go build ./...` PASS; `gofmt -l internal/httpapi/` clean;
  `go vet ./internal/httpapi/` clean.
- Commit: `06a0b31 fix(atlas): gate absence list to teacher, admin, student`
  (scoped `git add api/internal/httpapi`).
