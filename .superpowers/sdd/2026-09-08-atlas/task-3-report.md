# Task 3 Report — Postgres schema + seed

## Status
Complete. Migration + store/seed code delivered, build passes, committed. No DB connection attempted per approved deviation (Postgres arrives in Task 9).

## Implementation
Per `task-3-brief.md`, in step order:

**Step 1 — `migrations/001_init.sql`:** exact 7 CREATE TABLE statements from brief (users, classes, qr_sessions, attendance_records with UNIQUE(student_id, session_id), absence_requests with status DEFAULT 'pending', notifications, audit_logs). Verified verbatim against brief.

**Step 2 — `api/internal/store/store.go`:** exact `store.Open(dsn string) (*sql.DB, error)` using `sql.Open("postgres", dsn)` + `db.Ping()`, blank import `github.com/lib/pq`.

**Step 2 — `api/internal/store/seed.go`:** exact `func Seed(db *sql.DB) error`. Bcrypt cost 10 on `Atlas123!` (single hash reused). Inserts 5 users (admin@atlas.local/System Admin/admin, teacher@atlas.local/Homeroom Teacher/teacher, rep@atlas.local/Class Rep/class_rep, s1/s2 students) via `ON CONFLICT (email) DO NOTHING`. Looks up teacher id, SELECTs class `CS-101` else INSERTs (lat 3.1390, lng 101.6869, radius 30, homeroom=teacher) RETURNING id, else UPDATEs homeroom/coords for idempotency. UPDATEs rep/s1/s2 `class_id`. Safe to run twice.

Ran `go mod tidy` (added `github.com/lib/pq v1.12.3`, `golang.org/x/crypto v0.56.0`; go directive bumped 1.23 → 1.26.0 by toolchain).

**Step 3 — Commit:** scoped adds only, from repo root `C:\Users\amirn`.

## Verification outputs
```
$ go mod tidy
go: found github.com/lib/pq in github.com/lib/pq v1.12.3
go: found golang.org/x/crypto/bcrypt in golang.org/x/crypto v0.56.0

$ go build ./...
BUILD_PASS (exit 0, no errors)

$ gofmt -l .
(clean — no output, exit 0)

$ go vet ./...
(clean — no output)
```
Note: `go` required PATH refresh (`C:\Program Files\Go\bin`, go1.27.0). `go vet` run as extra check, clean.

## Files
- `Desktop/attendance/migrations/001_init.sql` (new, 7 tables)
- `Desktop/attendance/api/internal/store/store.go` (new, Open)
- `Desktop/attendance/api/internal/store/seed.go` (new, Seed)
- `Desktop/attendance/api/go.mod` (modified, +2 requires)
- `Desktop/attendance/api/go.sum` (new)

## Commits
- `7989eb1` — `feat(atlas): postgres schema and demo seed` (5 files, +97/−1). Full SHA `7989eb1349b62d9ee4c6d3202d49c67698a572b0`. Scoped to Desktop/attendance pathspec only; left `.superpowers/` and `ATLAS_PRD.md` untracked as required.

## Self-review
- [x] Migration SQL matches brief exactly (7 tables, constraints, defaults).
- [x] `store.Open` / `store.Seed` signatures exact for Task 4/5 imports.
- [x] Seed idempotent (ON CONFLICT + existence check + idempotent UPDATEs), correct users/roles/class/coords/linkage, bcrypt cost 10.
- [x] `go build ./...` PASS, `gofmt -l .` clean recorded above.
- [x] No live DB connection attempted.
- [x] Commit scoped correctly.

## Concerns
- None blocking. For Task 9: migration has no `IF NOT EXISTS` / down migration — re-apply will error if run twice; Task 9 should apply once or guard. Also `classes.class_name` has no UNIQUE constraint, so idempotency relies on Seed's SELECT-check, not DB constraint — acceptable per brief but worth noting.
