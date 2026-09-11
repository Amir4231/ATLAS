# Task 3 Brief — Postgres schema + seed (from docs/superpowers/plans/2026-09-08-atlas.md)

## Approved deviation (human APPROVED pre-flight)

Postgres does not exist until Task 9 boots Compose, so this task does NOT
apply the migration. Deliverable: migration SQL + compiling store/seed code.
`go build ./...` must PASS. Task 9 performs the actual apply + seed verify.

## Global Constraints (bind this task)

- Tables/columns per spec: users(id, full_name, email unique, password_hash,
  role admin|teacher|class_rep|student, class_id), classes(id, class_name,
  homeroom_teacher_id, geofence_lat, geofence_lng, geofence_radius_m default 30),
  qr_sessions, attendance_records with UNIQUE(student_id, session_id),
  absence_requests(status pending default), notifications, audit_logs.
- Seed: admin, teacher, class_rep, 2 students, 1 class CS-101
  (lat 3.1390, lng 101.6869, radius 30). All seed passwords `Atlas123!`
  hashed with bcrypt cost 10.
- Commits scoped to Desktop/attendance pathspecs only.

## Environment

- PowerShell 5.1, refresh PATH for `go` if needed (see Task 2 brief).
- Work from `C:\Users\amirn\Desktop\attendance\api`.
- External deps allowed for DB driver + bcrypt: `github.com/lib/pq`,
  `golang.org/x/crypto/bcrypt`. Run `go mod tidy` after adding imports.

## Task

**Files:**
- Create: `migrations/001_init.sql` (at `Desktop/attendance/migrations/`)
- Create: `api/internal/store/store.go`
- Create: `api/internal/store/seed.go`

**Interfaces (exact — later tasks import these):**
- `store.Open(dsn string) (*sql.DB, error)` — opens + pings.
- `store.Seed(db *sql.DB) error` — idempotent (safe to run twice; use
  `ON CONFLICT DO NOTHING` or existence checks), inserts 5 users + 1 class,
  links teacher as homeroom, rep + students to class.

- [ ] **Step 1: Write migration SQL** (`Desktop/attendance/migrations/001_init.sql`)

Exactly these 7 tables (fail if any differ):

```sql
CREATE TABLE users(id SERIAL PRIMARY KEY, full_name TEXT NOT NULL, email TEXT UNIQUE NOT NULL, password_hash TEXT NOT NULL, role TEXT NOT NULL CHECK(role IN ('admin','teacher','class_rep','student')), class_id INT);
CREATE TABLE classes(id SERIAL PRIMARY KEY, class_name TEXT NOT NULL, homeroom_teacher_id INT, geofence_lat DOUBLE PRECISION NOT NULL, geofence_lng DOUBLE PRECISION NOT NULL, geofence_radius_m INT NOT NULL DEFAULT 30);
CREATE TABLE qr_sessions(id SERIAL PRIMARY KEY, class_id INT NOT NULL REFERENCES classes(id), totp_secret BYTEA NOT NULL, session_start TIMESTAMPTZ NOT NULL DEFAULT now(), session_end TIMESTAMPTZ);
CREATE TABLE attendance_records(id SERIAL PRIMARY KEY, student_id INT NOT NULL REFERENCES users(id), class_id INT NOT NULL, session_id INT NOT NULL REFERENCES qr_sessions(id), status TEXT NOT NULL, scan_lat DOUBLE PRECISION, scan_lng DOUBLE PRECISION, recorded_at TIMESTAMPTZ NOT NULL DEFAULT now(), UNIQUE(student_id, session_id));
CREATE TABLE absence_requests(id SERIAL PRIMARY KEY, student_id INT NOT NULL REFERENCES users(id), session_id INT NOT NULL, reason_type TEXT NOT NULL, proof_file_url TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'pending', created_at TIMESTAMPTZ NOT NULL DEFAULT now());
CREATE TABLE notifications(id SERIAL PRIMARY KEY, user_id INT NOT NULL REFERENCES users(id), kind TEXT NOT NULL, title TEXT NOT NULL, body TEXT NOT NULL, read_at TIMESTAMPTZ, created_at TIMESTAMPTZ NOT NULL DEFAULT now());
CREATE TABLE audit_logs(id SERIAL PRIMARY KEY, actor_id INT, action TEXT NOT NULL, entity TEXT NOT NULL, entity_id TEXT NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now());
```

- [ ] **Step 2: Write store.go + seed.go, tidy, build**

`store.go`:

```go
package store

import (
	"database/sql"

	_ "github.com/lib/pq"
)

func Open(dsn string) (*sql.DB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	return db, db.Ping()
}
```

`seed.go`: `func Seed(db *sql.DB) error` — bcrypt cost 10, password
`Atlas123!`; users admin@atlas.local (System Admin, admin),
teacher@atlas.local (Homeroom Teacher, teacher),
rep@atlas.local (Class Rep, class_rep), s1@atlas.local + s2@atlas.local
(students); class `CS-101` lat 3.1390 lng 101.6869 radius 30 with the teacher
as homeroom; rep and students linked via class_id. Idempotent.

Run: `go mod tidy`; then `go build ./...` Expected: PASS with no errors.
Run: `gofmt -l .` Expected: clean.

- [ ] **Step 3: Commit** (repo root `C:\Users\amirn`)

```bash
git add Desktop/attendance/migrations Desktop/attendance/api/internal/store Desktop/attendance/api/go.mod Desktop/attendance/api/go.sum
git commit -m "feat(atlas): postgres schema and demo seed"
```

Note: go.mod/go.sum included because tidy adds lib/pq + x/crypto.
