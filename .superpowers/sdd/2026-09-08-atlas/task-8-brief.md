# Task 8 Brief — HTTP wiring + main (from docs/superpowers/plans/2026-09-08-atlas.md)

## GIT GUARD (mandatory)

- Repo root IS `C:\Users\amirn\Desktop\attendance`. NEVER `git init`,
  NEVER touch anything outside it.
- Commits: `git add api/internal/httpapi api/main.go api/internal/attend`
  (attend only if you add the Secret accessor below). Nothing else.

## Global Constraints (bind this task)

- All `/v1/` routes JSON, JWT-required except login. Error envelope
  `{"error":"CODE"}`. Role checks → 403 `{"error":"FORBIDDEN"}`.
- Scan error mapping: attend.ErrInvalidCode → 401 INVALID_CODE;
  ErrOutOfFence → 403 OUT_OF_GEOFENCE with `distance_m`; ErrRateLimited →
  429 RATE_LIMITED; ErrSessionClosed → 410 SESSION_CLOSED.
- Rate limit 10/min/IP via attend.RedisLimiter backed by go-redis client.
- Uploads: `http.MaxBytesReader` 6MB at handler; storage.CheckUpload enforces
  5MB + MIME; files saved via storage.LocalStorage{Dir: UPLOAD_DIR,
  BaseURL: "/uploads"}.
- JWT secret from env, min 32 bytes or refuse to boot.
- No live services here: handler tests use httptest + sqlmock + MemLimiter.

## Environment

- PowerShell 5.1, refresh PATH for `go` if needed.
- Go workspace `C:\Users\amirn\Desktop\attendance\api`.

## FIRST: read existing signatures

Before writing code, read these files (small) and use their EXACT symbols:
`api/internal/auth/auth.go`, `api/internal/attend/attend.go`,
`api/internal/storage/storage.go`, `api/internal/absent/absent.go`,
`api/internal/report/report.go`, `api/internal/store/store.go`.
Note: `report.CollegeRate(db)` returns `ClassRate` (no CollegeRate type —
type+func collision, do NOT reference `report.CollegeRate` as a type).

## Missing accessor (authorize + test it)

Task 5 did not export a secret loader, but `GET code` needs one. Add to
`api/internal/attend/attend.go`:
`func SessionSecret(db *sql.DB, sessionID int) ([]byte, error)` —
`SELECT totp_secret FROM qr_sessions WHERE id=$1` (no rows → ErrSessionClosed).
Add `TestSessionSecret` (sqlmock) to `attend_test.go`.

## Task

**Files:**
- Create: `api/internal/httpapi/server.go` (Server struct, routes, handlers)
- Test: `api/internal/httpapi/server_test.go`
- Create: `api/main.go`

**Server shape:**

```go
type Server struct {
  DB *sql.DB
  Limiter attend.Limiter
  JWTSecret string
  Store storage.Storage
}
func NewServer(db *sql.DB, lim attend.Limiter, secret string, st storage.Storage) *Server
func (s *Server) Routes() http.Handler  // net/http ServeMux, no external router
```

Helper: `writeJSON(w, code, v)`, `writeErr(w, code, msg)`,
`requireRole(allowed ...string)` middleware using auth.UID/Role (auth.Middleware
applied globally except login — apply per-route: public mux for login,
protected sub-mux wrapped in auth.Middleware).

**Routes (all under /v1/):**
- POST /v1/auth/login {email,password} → 200 {token}; unknown/bad → 401 INVALID_CREDENTIALS. (Query users: id, password_hash, role.)
- GET /v1/me → {id, full_name, email, role, class_id}.
- POST /v1/qr/session {class_id} [rep,admin] → {session_id} (attend.OpenSession).
- GET /v1/qr/session/{id}/code [rep,admin] → {code, exp} (SessionSecret + attend.Code(time.Now())).
- POST /v1/attendance/scan {session_id, code, lat, lng} [student] → {status, distance_m} with error mapping above. studentID = atoi(auth.UID).
- POST /v1/qr/session/{id}/close [rep,admin] → {marked} (attend.Close).
- GET /v1/attendance?session_id= [all] (students auto-filtered to self: append `AND student_id=$n`) → {records:[{id,student_id,status,scan_lat,scan_lng,recorded_at}]}.
- POST /v1/absences [student] multipart (session_id, reason_type, file) → {id} (absent.Submit; proofURL from Store.Save).
- GET /v1/absences?status= [teacher,admin all; student own] → {requests:[...]}.
- PATCH /v1/absences/{id}/review {decision:"approved"|"rejected"} [teacher,admin] → {status} (absent.Review; bad decision → 400).
- POST /v1/admin/classes {class_name, homeroom_teacher_id, geofence_lat, geofence_lng, geofence_radius_m} [admin] → {id}.
- GET /v1/classes [all] → {classes:[...]}.
- GET /v1/analytics/class?session_id= [teacher,admin] → {rate, categories}.
- GET /v1/analytics/college [admin] → college rate (report.CollegeRate → ClassRate shape).
- GET /v1/notifications [all] → {notifications}; PATCH /v1/notifications/{id}/read → {ok:true}.
- GET /v1/audit-logs?limit= [admin] → {logs}.

Path params: parse with strings.TrimPrefix (ServeMux patterns like
`POST /v1/qr/session/{id}/code` need Go 1.22+ ServeMux — allowed since
toolchain is Go 1.27; use `r.PathValue("id")`).

**main.go:** read PORT (default 8080), DATABASE_URL, REDIS_ADDR,
JWT_SECRET (fatal if <32 bytes), UPLOAD_DIR (default ./uploads);
store.Open (fatal on error); redis.NewClient + Ping (fatal on error);
if SEED=="true" → store.Seed(db); NewServer(db, attend.NewRedisLimiter(rdb,
10, time.Minute), secret, storage.LocalStorage{Dir, BaseURL:"/uploads"});
http.ListenAndServe(":"+port).

**Tests (server_test.go, httptest + sqlmock + MemLimiter):**
1. Login success (user row → 200 token parses via auth.Parse) + bad password → 401.
2. Scan mapping: bad code row → 401 INVALID_CODE; far GPS → 403 with distance_m; pre-consumed MemLimiter(0?) → 429. (Use NewMemLimiter with limit consumed, or a stub Limiter returning false.)
3. Role gate: student calling POST /v1/admin/classes → 403; no token → 401.
4. Me endpoint returns row.

- [ ] **Step 1: Read package files, then write failing test** (compile FAIL
  "undefined: httpapi" expected — or test-first per handler; minimum: write
  server_test.go first, run → FAIL).

- [ ] **Step 2: Implement server.go + main.go + Secret accessor.**

- [ ] **Step 3: Run PASS + commit**

Run: `go test ./internal/httpapi/ ./internal/attend/ -v` Expected: PASS.
Run: `go test ./...`, `gofmt -l .` clean, `go vet ./...` clean,
`go build ./...` PASS.

```bash
git add api/internal/httpapi api/main.go api/internal/attend
git commit -m "feat(atlas): http wiring and service entrypoint"
```
