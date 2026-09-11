# Task 5 Report — QR session + scan engine (TDD)

## Status: DONE

## Steps (in order)
1. Read brief + existing `totp`/`geo`/`store`/migration schema.
2. RED: wrote `api/internal/attend/attend_test.go` (6 tests per brief), ran `go mod tidy` + `go test ./internal/attend/ -v` → FAIL `undefined: Scan` (plus `NewMemLimiter`, `ErrInvalidCode`…).
3. GREEN: implemented `api/internal/attend/attend.go` (exact symbols: `ErrInvalidCode/ErrOutOfFence/ErrRateLimited/ErrSessionClosed`, `Limiter`, `MemLimiter/NewMemLimiter`, `RedisLimiter/NewRedisLimiter`, `OpenSession/Code/Scan/Close`).
4. First GREEN run: 5/6 — happy-path test mock expected 4 notification args, impl passes 1 (`user_id` only; kind/title/body are literals). Fixed the test expectation (test-side bug, not impl).
5. Re-ran: 6/6 PASS; `gofmt -l .` clean (after `gofmt -w`), `go vet ./internal/attend/` clean, `go build ./...` PASS, full `go test ./...` PASS (attend/auth/geo/totp).
6. Committed scoped: `git add Desktop/attendance/api/internal/attend Desktop/attendance/api/go.mod Desktop/attendance/api/go.sum`.

## Test summary
- `go test ./internal/attend/ -v`: 6/6 PASS (`TestScanRejectsBadCode`, `TestScanRejectsFarGPS`, `TestScanRateLimited`, `TestScanHappyPathIdempotent`, `TestMemLimiter`, `TestCodeBoundary`).

## Commits
- `6dd5f97` feat(atlas): qr session and scan engine (4 files: attend.go, attend_test.go, go.mod, go.sum)

## Design notes (for Task 8 consumers)
- `Scan` order: rate-limit (`lim.Allow("scan:"+ip)`) → session load (exact brief query; missing/ended → `ErrSessionClosed`) → `totp.Valid` → `geo.HaversineM` vs radius (`ErrOutOfFence` returns distM) → `INSERT … SELECT class_id FROM qr_sessions … ON CONFLICT DO NOTHING RETURNING id` (no-rows = idempotent re-scan → `"present"`, nil); only fresh inserts write `audit_logs` (`attendance.scan`) + `notifications` (`attendance.present`) rows.
- `RedisLimiter.Allow` uses the key verbatim (`INCR` + `EXPIRE` on first hit); `Scan` passes `"scan:"+ip`.
- `Close`: `UPDATE qr_sessions SET session_end=now()` then `INSERT … SELECT` unmarked class students as `absent_unexcused` `ON CONFLICT DO NOTHING`, returns rows added. No dedicated test (not in brief's 6).

## Concerns
- `go` was not on PATH (PowerShell 5.1); used `C:\Program Files\Go\bin` explicitly.
- Working tree at repo root contains many unrelated pre-existing modifications; commit scoped strictly to the 4 brief-listed paths (verified: 4 files changed).
