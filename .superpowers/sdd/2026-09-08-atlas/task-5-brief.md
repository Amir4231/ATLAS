# Task 5 Brief — QR session + scan engine, TDD (from docs/superpowers/plans/2026-09-08-atlas.md)

## Global Constraints (bind this task)

- TOTP 5s ±1 via `totp` package; Haversine gate via `geo` package; default
  radius 30m (per-class `geofence_radius_m`).
- Rate limit 10 scans/min/IP. One mark per (student_id, session_id),
  idempotent re-scan returns present, no error.
- Fail-closed: bad code → ErrInvalidCode; outside fence → ErrOutOfFence;
  over limit → ErrRateLimited; closed session → ErrSessionClosed.
- Every present-mark writes one audit_logs row + one student notification row.
- No live Postgres/Redis in this task (they arrive in Task 9). All tests use
  `go-sqlmock` + in-memory limiter. New test-only dep allowed:
  `github.com/DATA-DOG/go-sqlmock`. Runtime Redis client dep allowed:
  `github.com/redis/go-redis/v9` (used by RedisLimiter; live-tested in T11).
- Commits scoped to Desktop/attendance pathspecs only.

## Environment

- PowerShell 5.1, refresh PATH for `go` if needed.
- Work from `C:\Users\amirn\Desktop\attendance\api`.

## Task

**Files:**
- Create: `api/internal/attend/attend.go`
- Test: `api/internal/attend/attend_test.go`

**Interfaces (exact — Task 8 imports these):**
- `var ErrInvalidCode, ErrOutOfFence, ErrRateLimited, ErrSessionClosed error`
- `type Limiter interface { Allow(key string) (bool, error) }`
- `type MemLimiter struct` + `func NewMemLimiter(limit int, window time.Duration) *MemLimiter` (sliding-window in-memory; used in tests and as fallback)
- `type RedisLimiter struct` + `func NewRedisLimiter(rdb *redis.Client, limit int, window time.Duration) *RedisLimiter` (INCR+EXPIRE; `Allow("scan:<ip>:<unixminute>")`)
- `func OpenSession(db *sql.DB, classID int) (id int, secret []byte, err error)` — 32 crypto-rand bytes, `INSERT INTO qr_sessions(class_id, totp_secret) ... RETURNING id`.
- `func Code(secret []byte, t time.Time) (code string, exp int64)` — `totp.Generate`, exp = next 5s boundary Unix (`(t.Unix()/5+1)*5`).
- `func Scan(db *sql.DB, lim Limiter, ip string, sessionID int, studentID int, code string, lat, lng float64, t time.Time) (status string, distM float64, err error)` — order: (1) `lim.Allow("scan:"+ip)` false → ErrRateLimited; (2) load session: secret, class geofence lat/lng/radius, session_end — missing or ended → ErrSessionClosed; (3) `totp.Valid(secret, code, t)` false → ErrInvalidCode; (4) `geo.HaversineM` > radius → ErrOutOfFence (return distM); (5) `INSERT ... ON CONFLICT(student_id,session_id) DO NOTHING RETURNING id`, if inserted write audit_logs (`attendance.scan`) + notifications (student, `attendance.present`) rows; return "present", distM, nil in all success cases including duplicates.
- `func Close(db *sql.DB, sessionID int) (marked int64, err error)` — set session_end=now; `INSERT ... SELECT` unmarked class students as absent_unexcused `ON CONFLICT DO NOTHING`; return rows added.

- [ ] **Step 1: Failing tests** (`api/internal/attend/attend_test.go`)

Tests use `github.com/DATA-DOG/go-sqlmock` for `*sql.DB` and `MemLimiter`:
1. `TestScanRejectsBadCode` — mock session row (secret = fixed 32B, class fence 3.1390/101.6869/30, session_end NULL); Scan with "000000" (ensure it differs from real code at that instant by computing `totp.Generate` first and skipping if equal) → `errors.Is(err, ErrInvalidCode)`.
2. `TestScanRejectsFarGPS` — same row; real code via `totp.Generate(secret, now)`; lat/lng 1km away → `errors.Is(err, ErrOutOfFence)` and distM > 900.
3. `TestScanRateLimited` — limiter pre-filled (NewMemLimiter(1, time.Minute), one Allow consumed); Scan → `errors.Is(err, ErrRateLimited)`; expect NO db queries (`mock.ExpectationsWereMet()`).
4. `TestScanHappyPathIdempotent` — real code; expect INSERT ... RETURNING (first call returns id, second call returns no rows = conflict); both return status "present", nil error; audit+notification INSERTs expected exactly once.
5. `TestMemLimiter` — limit 2/window: Allow,Allow true, third false.
6. `TestCodeBoundary` — `Code(secret, time.Unix(7,0))` exp == 10.

Session-row query shape (implementer must match in impl — single query):
`SELECT totp_secret, geofence_lat, geofence_lng, geofence_radius_m, session_end FROM qr_sessions JOIN classes ON classes.id = qr_sessions.class_id WHERE qr_sessions.id = $1`.

- [ ] **Step 2: Run, verify FAIL**

Run: `go mod tidy` (adds sqlmock + go-redis), then
`go test ./internal/attend/ -v` Expected: FAIL "undefined: Scan" (or MemLimiter).

- [ ] **Step 3: Minimal impl** (`api/internal/attend/attend.go`)

Per interfaces above. RedisLimiter.Allow: `n := rdb.Incr(ctx, key).Val(); if n == 1 { rdb.Expire(ctx, key, window) }; return n <= limit`. MemLimiter: mutex-guarded map[key][]UnixNano, prune older than window.

- [ ] **Step 4: Run PASS + commit**

Run: `go test ./internal/attend/ -v` Expected: PASS (6/6).
Run: `gofmt -l .` clean, `go vet ./internal/attend/` clean, `go build ./...` PASS.

```bash
git add Desktop/attendance/api/internal/attend Desktop/attendance/api/go.mod Desktop/attendance/api/go.sum
git commit -m "feat(atlas): qr session and scan engine"
```
