# Class Enrollment (Join Links + Approval) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Students join classes via shareable links with teacher approval, and manage multiple class memberships from dashboards.

**Architecture:** New `class_memberships` table (backfilled from `users.class_id`) becomes enrollment truth; new `api/internal/enroll` package mirrors `absent` patterns; five new endpoints; teacher cards with copyable join links plus inline approve/reject; student join card via `?join=`.

**Tech Stack:** Go 1.x + sqlmock tests, PostgreSQL 16 (manual `psql -f` migration, no auto-migrate per docker-compose.yml), vanilla JS no build.

## Global Constraints

- No change to TOTP period (5s), geofence math, QR scan flow, rate-limit values, absence pipeline.
- Status strings are exactly `pending`, `approved`, `rejected`.
- Error codes are exactly `NOT_PENDING`, `NOT_A_MEMBER`, `NOT_YOUR_CLASS`, `ALREADY_MEMBER`, `ALREADY_PENDING`, `FORBIDDEN`.
- Join links are code-free: `student.html?join=<class_id>` only.
- `users.class_id` is legacy: read nothing new from it, write nothing to it.
- Migrations are single-apply manual files (`migrations/002_*.sql`), applied with `psql -f`; use `IF NOT EXISTS` + `ON CONFLICT DO NOTHING` so re-apply is safe.

---

## File Structure

- `migrations/002_memberships.sql` — creates `class_memberships`, backfills approved rows. Single responsibility: schema + data backfill.
- `api/internal/enroll/enroll.go` — `RequestJoin`, `ReviewJoin`, list helpers, `ErrNotPending`. Single responsibility: enrollment business logic (mirrors `absent` package).
- `api/internal/enroll/enroll_test.go` — sqlmock unit tests. Single responsibility: prove enroll logic.
- `api/internal/httpapi/server.go` — five new routes + handlers. Single responsibility: HTTP wiring (logic stays in `enroll`).
- `api/internal/httpapi/enroll_test.go` — route/gate tests. Single responsibility: prove gates + status mapping.
- `api/internal/attend/attend.go:Close` — absent-marking switches to approved members. Single responsibility unchanged.
- `web/app.js` — `Atlas.classJoinLink`, `Atlas.joinClassFromQuery`. Single responsibility: shared link helpers.
- `web/teacher.html` — "My classes" cards + approval queue. Existing sections byte-identical.
- `web/student.html` — join card + `?join=` boot. Existing tabs byte-identical.

---

### Task 1: Migration — memberships table + backfill

**Files:**
- Create: `migrations/002_memberships.sql`
- Test: apply + query via psql (no Go test covers DDL in this repo)

**Interfaces:**
- Consumes: existing `users(id)`, `classes(id)`, `users.class_id`.
- Produces: `class_memberships(user_id INT REFERENCES users(id), class_id INT REFERENCES classes(id), status TEXT DEFAULT 'pending' CHECK(status IN ('pending','approved','rejected')), created_at TIMESTAMPTZ DEFAULT now(), UNIQUE(user_id, class_id))` plus backfilled approved rows.

- [ ] **Step 1: Write the migration file**

```sql
CREATE TABLE IF NOT EXISTS class_memberships(
  user_id INT NOT NULL REFERENCES users(id),
  class_id INT NOT NULL REFERENCES classes(id),
  status TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','approved','rejected')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(user_id, class_id)
);
INSERT INTO class_memberships(user_id, class_id, status)
  SELECT id, class_id, 'approved' FROM users WHERE class_id IS NOT NULL
  ON CONFLICT (user_id, class_id) DO NOTHING;
```

- [ ] **Step 2: Apply and verify**

Run: `psql "$DATABASE_URL" -f migrations/002_memberships.sql` (or `docker compose exec postgres psql -U atlas -d atlas -f - < migrations/002_memberships.sql`)
Expected: `CREATE TABLE` (or `already exists` notice on re-apply, zero errors), `INSERT 0 N` with N = seeded users having class_id (3 in seed: rep, s1, s2).

Run: `psql "$DATABASE_URL" -c "SELECT user_id, class_id, status FROM class_memberships ORDER BY 1;"`
Expected: one `approved` row per user that had `class_id`; re-running Step 2's apply changes nothing.

- [ ] **Step 3: Commit**

```bash
git add migrations/002_memberships.sql
git commit -m "feat: add class_memberships with backfill"
```

---

### Task 2: Enroll package — request/review logic + unit tests

**Files:**
- Create: `api/internal/enroll/enroll.go`
- Test: `api/internal/enroll/enroll_test.go`

**Interfaces:**
- Consumes: `*sql.DB` only.
- Produces:
  - `func RequestJoin(db *sql.DB, studentID, classID int) (status, prev string, err error)` — fresh insert returns `(status, "", nil)`; existing row returns `(status, status, nil)` EXCEPT rejected rows, which flip back to pending and return `("pending", "rejected", nil)`. Unknown class → `("", "", ErrNoClass)`.
  - `func ReviewJoin(db *sql.DB, reqID int, approve bool) error` — pending-only transition else `ErrNotPending` (mirrors `absent.Review` tx pattern).
  - `var ErrNotPending`, `var ErrNoClass`.

- [ ] **Step 1: Write the failing test**

In `api/internal/enroll/enroll_test.go`:

```go
package enroll

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestRequestJoinInsertsPending(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery("INSERT INTO class_memberships").
		WithArgs(7, 3).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("pending"))

	st, prev, err := RequestJoin(db, 7, 3)
	if err != nil {
		t.Fatal(err)
	}
	if st != "pending" || prev != "" {
		t.Fatalf("expected (pending, \"\"), got (%q, %q)", st, prev)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestReviewJoinRejectsNonPending(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT status FROM class_memberships").
		WithArgs(9).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("approved"))

	if err := ReviewJoin(db, 9, true); err != ErrNotPending {
		t.Fatalf("expected ErrNotPending, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run from `api/`: `go test ./internal/enroll/ -v`
Expected: FAIL — `undefined: RequestJoin` / `undefined: ReviewJoin`.

- [ ] **Step 3: Write minimal implementation**

In `api/internal/enroll/enroll.go`:

```go
package enroll

import (
	"database/sql"
	"errors"
)

var (
	ErrNotPending = errors.New("join request is not pending")
	ErrNoClass    = errors.New("class not found")
)

func RequestJoin(db *sql.DB, studentID, classID int) (string, string, error) {
	var exists int
	if err := db.QueryRow(`SELECT 1 FROM classes WHERE id=$1`, classID).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", "", ErrNoClass
		}
		return "", "", err
	}
	var status string
	err := db.QueryRow(
		`INSERT INTO class_memberships(user_id, class_id, status) VALUES ($1, $2, 'pending') ON CONFLICT (user_id, class_id) DO NOTHING RETURNING status`,
		studentID, classID,
	).Scan(&status)
	if err == nil {
		return status, "", nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", "", err
	}
	var flipped string
	ferr := db.QueryRow(
		`UPDATE class_memberships SET status='pending' WHERE user_id=$1 AND class_id=$2 AND status='rejected' RETURNING status`,
		studentID, classID,
	).Scan(&flipped)
	if ferr == nil {
		return flipped, "rejected", nil
	}
	if !errors.Is(ferr, sql.ErrNoRows) {
		return "", "", ferr
	}
	if qerr := db.QueryRow(
		`SELECT status FROM class_memberships WHERE user_id=$1 AND class_id=$2`,
		studentID, classID,
	).Scan(&status); qerr != nil {
		return "", "", qerr
	}
	return status, status, nil
}

func ReviewJoin(db *sql.DB, reqID int, approve bool) error {
	var status string
	if err := db.QueryRow(`SELECT status FROM class_memberships WHERE id=$1`, reqID).Scan(&status); err != nil {
		return err
	}
	if status != "pending" {
		return ErrNotPending
	}
	newStatus := "rejected"
	if approve {
		newStatus = "approved"
	}
	_, err := db.Exec(`UPDATE class_memberships SET status=$1 WHERE id=$2`, newStatus, reqID)
	return err
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run from `api/`: `go test ./internal/enroll/ -v`
Expected: PASS both tests.

- [ ] **Step 5: Commit**

```bash
git add api/internal/enroll/enroll.go api/internal/enroll/enroll_test.go
git commit -m "feat: enrollment request and review logic"
```

---

### Task 3: HTTP routes — join/requests/review/teaching/class + gate tests

**Files:**
- Modify: `api/internal/httpapi/server.go` (routes + 5 handlers)
- Test: `api/internal/httpapi/enroll_test.go`

**Interfaces:**
- Consumes: `enroll.RequestJoin`, `enroll.ReviewJoin`, `auth.UID`, `auth.Role`, existing `s.prot` gate.
- Produces:
  - `POST /v1/classes/{id}/join` (student) → 200 `{"status":"pending"|"approved"|"rejected"}`; unknown class 404 `NO_CLASS`; maps existing `approved`→200 with `ALREADY_MEMBER`? NO — body is `{"status":..., "note":"ALREADY_MEMBER"|"ALREADY_PENDING"|""}` with 200 always on known class. Exact: fresh → `{"status":"pending","note":""}`; existing pending → `{"status":"pending","note":"ALREADY_PENDING"}`; existing approved → `{"status":"approved","note":"ALREADY_MEMBER"}`; existing rejected → re-request allowed? NO — rejected stays, returns `{"status":"rejected","note":""}` and student must wait; teacher re-approval path is a fresh request only after teacher deletes? Keep simple: rejected → 200 `{"status":"rejected","note":"REJECTED"}` (student may tap again but status stays rejected until teacher approves? There is no re-request path — instead allow re-request: `UPDATE ... SET status='pending' WHERE status='rejected'` then return pending). FINAL: rejected rows flip back to pending on re-request, return `{"status":"pending","note":"REREQUESTED"}`.
  - `GET /v1/classes/{id}/join-requests?status=pending` (teacher of class, admin) → 200 `{"requests":[{"id","student_id","student_name","student_email","status","created_at"}]}`; other teacher 403 `NOT_YOUR_CLASS`.
  - `PATCH /v1/classes/requests/{reqId}` (teacher of that request's class, admin) body `{"decision":"approved"|"rejected"}` → 200 `{"status":...}`; non-pending 409 `NOT_PENDING`; bad decision 400.
  - `GET /v1/classes/teaching` (teacher, admin) → 200 `{"classes":[{"id","class_name","member_count","pending_count"}]}` (homeroom-owned only unless admin).
  - `GET /v1/classes/{id}` (any authenticated) → 200 `{"id","class_name"}` (name-only, powers the join card); unknown 404 `NO_CLASS`.

- [ ] **Step 1: Write the failing gate test**

In `api/internal/httpapi/enroll_test.go`:

```go
package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"atlas/internal/attend"
	"atlas/internal/auth"

	"github.com/DATA-DOG/go-sqlmock"
)

func enrollTeacherToken(t *testing.T, uid string) string {
	t.Helper()
	tok, err := auth.Token(uid, "teacher", testSecret)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func TestJoinRequiresStudent(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	srv := newTestServer(db, attend.NewMemLimiter(10, time.Minute))
	req := httptest.NewRequest("POST", "/v1/classes/3/join", nil)
	req.Header.Set("Authorization", "Bearer "+enrollTeacherToken(t, "11"))
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("teacher join: expected 403, got %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "FORBIDDEN") {
		t.Fatalf("expected FORBIDDEN, got %s", rec.Body.String())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run from `api/`: `go test ./internal/httpapi/ -run TestJoinRequiresStudent -v`
Expected: FAIL — 404 no route (before routes exist).

- [ ] **Step 3: Write minimal implementation**

In `server.go` `Routes()` add (roles: join=student; join-requests=teacher,admin; review=teacher,admin; teaching=teacher,admin; single-class get=all authenticated via `s.prot(h)` with no roles):

```go
mux.Handle("POST /v1/classes/{id}/join", s.prot(s.handleClassJoin, "student"))
mux.Handle("GET /v1/classes/{id}/join-requests", s.prot(s.handleJoinRequests, "teacher", "admin"))
mux.Handle("PATCH /v1/classes/requests/{reqId}", s.prot(s.handleJoinReview, "teacher", "admin"))
mux.Handle("GET /v1/classes/teaching", s.prot(s.handleTeaching, "teacher", "admin"))
mux.Handle("GET /v1/classes/{id}", s.prot(s.handleClassGet))
```

Handlers (exact behavior):
- `handleClassJoin`: uid from `auth.UID`; class id from `PathValue("id")` (400 BAD_REQUEST if not int); call `enroll.RequestJoin` which returns `(status, prev, err)` per Task 2; `ErrNoClass`→404 NO_CLASS; note mapping: prev=="" → `""`; prev=="pending" → `ALREADY_PENDING`; prev=="approved" → `ALREADY_MEMBER`; prev=="rejected" → `REREQUESTED`. Always 200 on known class with body `{"status":..., "note":...}`.
- `handleJoinRequests`: verify caller owns class (`SELECT homeroom_teacher_id ...`; admin skips) else 403 NOT_YOUR_CLASS; list rows.
- `handleJoinReview`: verify ownership via request's class (join to classes) else 403 NOT_YOUR_CLASS; `enroll.ReviewJoin`; `ErrNotPending`→409 NOT_PENDING; bad decision→400 BAD_REQUEST.
- `handleTeaching`: teacher → own classes with counts (`COUNT(*) FILTER (WHERE status='approved')`, `FILTER (WHERE status='pending')`); admin → all.
- `handleClassGet`: name-only; 404 NO_CLASS.

Rejected-flip SQL inside `RequestJoin` conflict path:

```sql
UPDATE class_memberships SET status='pending' WHERE user_id=$1 AND class_id=$2 AND status='rejected' RETURNING status
```

If that returns a row → `("pending","rejected",nil)`. Else select current status → `(status,status,nil)`.

- [ ] **Step 4: Run tests to verify they pass**

Run from `api/`: `go test ./...`
Expected: PASS all packages including new gate test.

- [ ] **Step 5: Commit**

```bash
git add api/internal/httpapi/server.go api/internal/httpapi/enroll_test.go
git commit -m "feat: class join and approval endpoints"
```

---

### Task 4: Close-session marks approved members (not users.class_id)

**Files:**
- Modify: `api/internal/attend/attend.go:179-191` (`Close`)
- Test: `api/internal/attend/attend_test.go` (update `Close` expectation)

**Interfaces:**
- Consumes: `class_memberships` rows.
- Produces: identical signature `Close(db, sessionID) (marked int64, err error)`; absent-marking covers approved members of the session's class across ALL their classes (member joins session's class only — one class per session, unchanged).

- [ ] **Step 1: Update the failing test**

In `attend_test.go` find the `Close` test's `ExpectExec` for the absent-insert; change expected SQL pattern from `JOIN users u ON u.class_id` to `JOIN class_memberships m` (match the new query below; keep args `(sessionID)`).

- [ ] **Step 2: Run test to verify it fails**

Run from `api/`: `go test ./internal/attend/ -run TestClose -v`
Expected: FAIL — query mismatch.

- [ ] **Step 3: Write minimal implementation**

Replace the `Close` insert with:

```sql
INSERT INTO attendance_records(student_id, class_id, session_id, status) SELECT u.id, q.class_id, q.id, 'absent_unexcused' FROM qr_sessions q JOIN class_memberships m ON m.class_id = q.class_id AND m.status='approved' JOIN users u ON u.id = m.user_id WHERE q.id = $1 ON CONFLICT(student_id, session_id) DO NOTHING
```

- [ ] **Step 4: Run tests to verify they pass**

Run from `api/`: `go test ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add api/internal/attend/attend.go api/internal/attend/attend_test.go
git commit -m "feat: close marks approved members absent"
```

---

### Task 5: Teacher dashboard — My classes + join links + approval queue

**Files:**
- Modify: `web/teacher.html` (add section + script; existing rate/QR/queue byte-identical)
- Modify: `web/app.js` (add `Atlas.classJoinLink`, export it)

**Interfaces:**
- Consumes: `GET /v1/classes/teaching`, `GET /v1/classes/{id}/join-requests?status=pending`, `PATCH /v1/classes/requests/{id}`, `Atlas.classJoinLink(classId)` → absolute `student.html?join=<id>`.
- Produces: class cards with member/pending counts, Copy join link, inline Approve/Reject.

- [ ] **Step 1: Add app.js helper**

Before `window.Atlas`:

```js
  function classJoinLink(classId) {
    var base = location.href.split('?')[0].split('#')[0];
    var dir = base.slice(0, base.lastIndexOf('/') + 1);
    return dir + 'student.html?join=' + encodeURIComponent(classId);
  }
```

Add `classJoinLink` to the `window.Atlas` export list (keep existing entries).

- [ ] **Step 2: Add My-classes section + script to teacher.html**

Insert after page-header, before session-control section:

```html
  <section>
    <div class="section-title">
      <div class="section-title__icon">🏫</div>
      <h2>My classes</h2>
    </div>
    <div id="tclasses"><p class="empty">Loading classes…</p></div>
  </section>
```

Append inside the existing IIFE (after `loadClassOptions()` call site — new function names `loadTeaching`, `loadJoinQueue`, no clashes with `loadQueue`/`loadClassOptions`):

```js
  loadTeaching();
  async function loadTeaching() {
    var r = await Atlas.api('GET', '/v1/classes/teaching');
    if (r.status !== 200) { Atlas.el('tclasses').innerHTML = '<p class="empty">Could not load classes (' + r.status + ').</p>'; return; }
    var box = Atlas.el('tclasses');
    if (!r.data.classes.length) { box.innerHTML = '<p class="empty">No classes assigned yet.</p>'; return; }
    box.innerHTML = '';
    r.data.classes.forEach(function (c) {
      var card = document.createElement('div');
      card.className = 'card';
      card.innerHTML =
        '<div class="card__title">' + escapeHtml(c.class_name) +
        ' <span class="badge badge--blue">' + c.member_count + ' members</span>' +
        (c.pending_count ? ' <span class="badge badge--yellow">' + c.pending_count + ' pending</span>' : '') +
        '</div>' +
        '<div class="text-muted text-sm" style="margin-bottom:.5rem">Join link: <code>' + Atlas.classJoinLink(c.id) + '</code></div>' +
        '<div class="btn-row" style="margin-top:0"></div>' +
        '<div class="joinq" style="margin-top:.75rem"></div>';
      var row = card.querySelector('.btn-row');
      var cb = document.createElement('button');
      cb.className = 'btn btn--secondary btn--sm'; cb.textContent = 'Copy join link';
      cb.onclick = function () { copyTextT(Atlas.classJoinLink(c.id), 'Join link copied — share with students.'); };
      row.appendChild(cb);
      box.appendChild(card);
      loadJoinQueue(c.id, card.querySelector('.joinq'));
    });
  }
  async function loadJoinQueue(classId, box) {
    var r = await Atlas.api('GET', '/v1/classes/' + classId + '/join-requests?status=pending');
    if (r.status !== 200 || !r.data.requests.length) { box.innerHTML = '<p class="empty">No pending join requests.</p>'; return; }
    box.innerHTML = '';
    r.data.requests.forEach(function (x) {
      var d = document.createElement('div');
      d.className = 'flex items-center gap-2 flex-wrap';
      d.style.marginBottom = '.4rem';
      d.innerHTML = '<span class="font-semibold text-sm">' + escapeHtml(x.student_name) + '</span>' +
        '<span class="text-muted text-sm">' + escapeHtml(x.student_email) + '</span>';
      [['approved', 'Approve', 'btn--primary'], ['rejected', 'Reject', 'btn--danger']].forEach(function (pair) {
        var b = document.createElement('button');
        b.className = 'btn btn--sm ' + pair[2]; b.textContent = pair[1];
        b.onclick = function () { reviewJoin(x.id, pair[0], b, d); };
        d.appendChild(b);
      });
      box.appendChild(d);
    });
  }
  async function reviewJoin(id, decision, btn, rowEl) {
    btn.classList.add('loading'); btn.disabled = true;
    var r = await Atlas.api('PATCH', '/v1/classes/requests/' + id, { decision: decision });
    if (r.status === 200) { rowEl.style.opacity = '.4'; rowEl.style.pointerEvents = 'none'; setTimeout(loadTeaching, 900); }
    else { Atlas.msg('m0', 'Join review failed (' + r.status + '): ' + (r.data.error || ''), true); btn.classList.remove('loading'); btn.disabled = false; }
  }
  function copyTextT(t, okMsg) {
    function done() { Atlas.msg('m0', okMsg, false); }
    if (navigator.clipboard && navigator.clipboard.writeText) { navigator.clipboard.writeText(t).then(done, function () { fallbackT(t); done(); }); }
    else { fallbackT(t); done(); }
  }
  function fallbackT(t) {
    var ta = document.createElement('textarea'); ta.value = t; document.body.appendChild(ta); ta.select();
    try { document.execCommand('copy'); } catch (e) {}
    document.body.removeChild(ta);
  }
  function escapeHtml(s) { return String(s == null ? '' : s).replace(/[&<>"']/g, function (c) { return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]; }); }
```

- [ ] **Step 3: Manual verify**

Serve `web/`, login as teacher: My-classes cards show name/counts, Copy join link yields `.../student.html?join=N`, pending request appears with name/email, Approve flips it (re-check counts).
Expected: no console errors; existing sections render as before.

- [ ] **Step 4: Commit**

```bash
git add web/teacher.html web/app.js
git commit -m "feat: teacher class cards with join links and approvals"
```

---

### Task 6: Student — join card + ?join= boot + request flow

**Files:**
- Modify: `web/student.html` (join card section + boot script; QR/absence tabs byte-identical)
- Test: manual (no JS harness in repo)

**Interfaces:**
- Consumes: `Atlas.joinClassFromQuery()` (new, parse `?join=`), `GET /v1/classes/{id}` (name), `POST /v1/classes/{id}/join`.
- Produces: join card with Request-to-join → pending/member states.

- [ ] **Step 1: Add app.js query helper**

```js
  function joinClassFromQuery() {
    try {
      var q = new URLSearchParams(location.search);
      return +(q.get('join') || q.get('join_class')) || 0;
    } catch (e) { return 0; }
  }
```

Export it alongside `classJoinLink`.

- [ ] **Step 2: Add join card + boot script to student.html**

Insert as the first `<section>` inside `#tab-checkin` (before Scan QR block):

```html
        <div class="field" id="joinCard" style="display:none">
          <label>Join class</label>
          <div class="flex items-center gap-2 flex-wrap">
            <span id="joinName" class="font-semibold"></span>
            <button type="button" id="joinBtn" class="btn btn--primary btn--sm">Request to join</button>
          </div>
          <div class="field-hint" id="joinHint">Ask your teacher for the class link.</div>
        </div>
```

Boot script (top of student IIFE, before role guard usage — runs for any student load):

```js
  (function joinBoot() {
    var cid = Atlas.joinClassFromQuery();
    if (!cid) return;
    var card = document.getElementById('joinCard');
    card.style.display = '';
    Atlas.api('GET', '/v1/classes/' + cid).then(function (g) {
      if (g.status === 200) document.getElementById('joinName').textContent = g.data.class_name + ' (#' + cid + ')';
      else document.getElementById('joinName').textContent = 'Class #' + cid;
    });
    document.getElementById('joinBtn').onclick = async function () {
      var btn = document.getElementById('joinBtn');
      btn.classList.add('loading'); btn.disabled = true;
      var r = await Atlas.api('POST', '/v1/classes/' + cid + '/join', {});
      btn.classList.remove('loading');
      var hint = document.getElementById('joinHint');
      if (r.status !== 200) { Atlas.msg('m', 'Join failed (' + r.status + '): ' + (r.data.error || ''), true); btn.disabled = false; return; }
      if (r.data.status === 'approved') { hint.textContent = 'You are already a member of this class.'; btn.style.display = 'none'; }
      else { hint.textContent = 'Request sent — wait for your teacher to approve.'; btn.disabled = true; }
    };
  })();
```

- [ ] **Step 3: Manual verify**

Open `student.html?join=<id>` as student: card shows class name, Request → hint becomes pending; teacher sees it in queue; unknown id shows `Class #N` + 404 message on tap.
Expected: QR tab and absence tab untouched and working.

- [ ] **Step 4: Commit**

```bash
git add web/student.html web/app.js
git commit -m "feat: student class join card"
```

---

### Task 7: Full verification

**Files:** none.

- [ ] **Step 1: Run backend suite**

Run from `api/`: `go test ./...`
Expected: PASS all packages.

- [ ] **Step 2: Apply migration to dev DB and check backfill**

Run: `psql "$DATABASE_URL" -f migrations/002_memberships.sql` then `SELECT status, COUNT(*) FROM class_memberships GROUP BY 1;`
Expected: zero errors; approved count equals users-with-class_id count.

- [ ] **Step 3: End-to-end manual matrix**

Teacher copies link → student opens `?join=` → request → approve → student in member count; re-request idempotent; other teacher gets NOT_YOUR_CLASS; close session marks members absent; QR scan still works.
Expected: all green, no console errors.

## Self-Review

- Spec §5.1 migration + close switch → Tasks 1+4. Covered.
- Spec §5.2 five endpoints + teacher scoping + idempotent join + rejected-flip → Tasks 2+3. Covered; `RequestJoin→(status, prev, err)` contract defined once in Task 2 and consumed in Task 3.
- Spec §5.3 teacher cards + queue → Task 5. Covered.
- Spec §5.4 join card + `?join=` (minus beacon button, which is Plan 2) → Task 6 + `GET /v1/classes/{id}` in Task 3. Covered.
- Spec §7 join/review errors → Tasks 2+3 tests + Task 6 manual. Covered.
- Spec §9 Go tests → Tasks 2–4; manual → Tasks 5–7. Covered.
- No placeholders: every step has exact code, commands, expected outputs. Type consistency: `RequestJoin→(status, prev, err)`, notes `""|ALREADY_PENDING|ALREADY_MEMBER|REREQUESTED`, `classJoinLink(id)→string`, `joinClassFromQuery()→number` used identically in Tasks 5–6.
