# Task 6 Report — Absence upload + review (TDD)

## Steps (in order)

1. Read brief + existing conventions (`attend/attend.go`, sqlmock patterns, `migrations/001_init.sql`).
2. Created `api/internal/storage/storage_test.go` (5 tests) and `api/internal/absent/absent_test.go`
   (7 tests: brief's 6 + decline path + missing-row path... actually brief's 4 absent tests + 3 extra:
   insert-fallback, decline, missing row).
3. RED run: `go test ./internal/storage/ ./internal/absent/ -v` → build FAILED,
   `undefined: CheckUpload` / `undefined: Submit` (evidence captured from output).
   Note: `go` not on PATH (PowerShell 5.1); used `C:\Program Files\Go\bin\go.exe`.
4. Wrote minimal impls `storage.go` and `absent.go` per exact interfaces.
5. GREEN: `go test ./internal/... -v` → all PASS; `gofmt -l .` clean; `go vet ./...` clean;
   `go build ./...` PASS.
6. Self-review: exact Task 8 symbols verified (see below). One slip fixed: impl was first
   written to `absent_test.go` path by mistake — test file restored byte-identical, impl moved
   to `absent.go`; GREEN re-run confirms.
7. Committed scoped to task paths only.

## Test summary

- `atlas/internal/storage`: 5/5 PASS (bad MIME, oversize, PDF accept, save round-trip, path traversal)
- `atlas/internal/absent`: 7/7 PASS (bad reason, submit happy, approve, approve-insert-fallback,
  decline, not-pending rollback, missing row)
- Full suite `go test ./internal/... -v`: all packages PASS (attend/auth/geo/totp cached PASS).

## Commits

- `36e7ce4 feat(atlas): absence pipeline with file gate` — 4 files, +442:
  `api/internal/storage/{storage,storage_test}.go`, `api/internal/absent/{absent,absent_test}.go`
- Pathspec note: brief assumed repo root `C:\Users\amirn`, but the actual git root is
  `C:\Users\amirn\Desktop\attendance` (root-commit repo, everything else still untracked),
  so the equivalent scoped add `api/internal/storage api/internal/absent` was used.
  Tasks 1–5 files remain untracked/uncommitted (pre-existing state, untouched).

## Symbols for Task 8 (exact, do not rename)

- storage: `ErrBadType`, `ErrTooLarge`, `MaxUploadBytes = 5 << 20`,
  `CheckUpload(data []byte, filename string) error`, `Storage` interface,
  `LocalStorage{Dir, BaseURL}` + value-receiver `Save`.
- absent: `ErrBadReason`, `ErrNotPending`,
  `Submit(db *sql.DB, studentID, sessionID int, reason, proofURL string) (int, error)`,
  `Review(db *sql.DB, absenceID int, approve bool) error` (single tx, approve flips/creates
  `absent_excused`, non-pending → rollback + `ErrNotPending`, missing row → `sql.ErrNoRows`).

## Concerns

- None blocking. Minor: audit `entity_id` (TEXT column) is passed as int driver value,
  same as existing `attend` code — relies on pg driver conversion; fine for sqlmock tests.
- `go` binary not on PATH in this environment; future tasks need the full path or a PATH refresh.
