# Task 7 Brief — Analytics + notifications + audit reads (from docs/superpowers/plans/2026-09-08-atlas.md)

## GIT GUARD (mandatory — a prior task destroyed adjacent history)

- Repo root IS `C:\Users\amirn\Desktop\attendance`. NEVER run `git init`,
  NEVER delete/move any `.git`, NEVER touch anything outside this directory.
- Commits: `git add api/internal/report` (relative pathspec, from repo root)
  then commit. No `-A`, no `سو`, no parent paths.

## Global Constraints (bind this task)

- Analytics are read-only SQL aggregations over attendance_records (+
  absence_requests for category breakdown).
- Chronic absenteeism flag: unexcused share > 15% (strictly greater).
- Notifications/audit helpers must match the rows written by attend/absent
  tasks (do not change those packages).
- No live DB: tests use go-sqlmock (already available).
- Commits scoped to `api/internal/report` only.

## Environment

- PowerShell 5.1, refresh PATH for `go` if needed.
- Work from `C:\Users\amirn\Desktop\attendance` (repo root) for git,
  `...\attendance\api` for go commands.

## Task

**Files:**
- Create: `api/internal/report/report.go`
- Test: `api/internal/report/report_test.go`

**Interfaces (exact — Task 8 imports these):**
- `type ClassRate struct { Present, Total int; Pct float64 }`
- `func SessionRate(db *sql.DB, sessionID int) (ClassRate, error)` —
  `SELECT COUNT(*) total, COUNT(*) FILTER (WHERE status='present') present FROM attendance_records WHERE session_id=$1`; Pct = 100*present/total (0 if total==0).
- `type CollegeRate struct { Present, Total int; Pct float64 }`
- `func CollegeRate(db *sql.DB) (CollegeRate, error)` — same over whole table (no WHERE).
- `type ChronicRow struct { StudentID, ClassID int; Unexcused, Total int; Pct float64 }`
- `func Chronic(db *sql.DB, thresholdPct float64) ([]ChronicRow, error)` —
  per (student_id, class_id): total rows, unexcused rows
  (status='absent_unexcused'), pct; return rows with pct > thresholdPct
  (filter in Go after scanning grouped query).
- `type CategoryRow struct { Reason string; Count int }`
- `func CategoryBreakdown(db *sql.DB, sessionID int) ([]CategoryRow, error)` —
  join absence_requests to sessions? Schema has absence_requests.session_id
  directly: `SELECT reason_type, COUNT(*) FROM absence_requests WHERE session_id=$1 GROUP BY reason_type`.
- `type Notification struct { ID int; Kind, Title, Body string; Read *time.Time; Created time.Time }`
- `func ListNotifications(db *sql.DB, userID int) ([]Notification, error)` —
  latest 50, newest first. `func MarkRead(db *sql.DB, id, userID int) error` —
  `UPDATE notifications SET read_at=now() WHERE id=$1 AND user_id=$2`.
- `type AuditRow struct { ID int; ActorID *int; Action, Entity, EntityID string; Created time.Time }`
- `func ListAudit(db *sql.DB, limit int) ([]AuditRow, error)` — newest first,
  limit clamped to 1..200.

- [ ] **Step 1: Failing tests** (`api/internal/report/report_test.go`, sqlmock)

`TestSessionRate` (total 4, present 3 → Pct 75), `TestSessionRateEmpty`
(total 0 → Pct 0, no NaN), `TestCollegeRate`, `TestChronicThreshold`
(two grouped rows 20% + 10%, threshold 15 → only first),
`TestCategoryBreakdown`, `TestNotifications` (list + mark read),
`TestListAudit` (limit clamp: pass 5000 → query uses 200; check via
WithArgs or query inspection — simplest: call ListAudit(db, 5000) and expect
query with LIMIT $1 and arg 200).

- [ ] **Step 2: Run, verify FAIL**

Run: `go test ./internal/report/ -v` Expected: FAIL "undefined: SessionRate".

- [ ] **Step 3: Minimal impl** per interfaces above.

- [ ] **Step 4: Run PASS + commit**

Run: `go test ./internal/... -v` Expected: PASS.
Run: `gofmt -l .` clean, `go vet ./...` clean, `go build ./...` PASS.

```bash
git add api/internal/report
git commit -m "feat(atlas): analytics and feeds"
```
