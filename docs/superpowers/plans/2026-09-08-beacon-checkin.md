# Beacon "I Am Here" Check-in Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** While a session is live, approved class members tap one "I am here" button (GPS only, no code) to be marked present.

**Architecture:** New `attend.Here` mirrors `attend.Scan` minus TOTP (live-session resolve + Haversine + rate limit + conflict-safe insert); `POST /v1/attendance/here` maps its errors; `GET /v1/classes/mine` feeds the student dashboard beacon button with ~10s polling.

**Tech Stack:** Go 1.x + sqlmock tests, vanilla JS no build.

## Global Constraints

- No change to TOTP period (5s), QR scan flow, geofence math, rate-limit values, absence pipeline.
- Beacon state = open `qr_sessions` row (`session_end IS NULL`); no new session machinery.
- Error codes are exactly `NO_BEACON`, `AMBIGUOUS_BEACON`, `OUT_OF_GEOFENCE`, `RATE_LIMITED`, `SESSION_CLOSED`, `NOT_A_MEMBER`, `BAD_REQUEST`.
- Never auto-submit: the student presses the button every session.
- Requires the Class Enrollment plan merged first (`class_memberships`, join/approve endpoints).

---

## File Structure

- `api/internal/attend/attend.go` — add `Here` + `ErrNoBeacon`. Single responsibility unchanged (attendance writes).
- `api/internal/attend/here_test.go` — unit tests for `Here`. Single responsibility: prove beacon logic.
- `api/internal/httpapi/server.go` — two routes + handlers (`/here`, `/mine`). Single responsibility: HTTP wiring.
- `api/internal/httpapi/here_test.go` — mapping/gate tests. Single responsibility: prove status mapping.
- `web/app.js` — `Atlas.hereError`. Single responsibility: error wording.
- `web/student.html` — "My classes" + beacon button + polling. QR/absence/join-card sections byte-identical.

---

### Task 1: attend.Here + unit tests

**Files:**
- Modify: `api/internal/attend/attend.go` (append `Here`; add `ErrNoBeacon` to var block)
- Test: `api/internal/attend/here_test.go`

**Interfaces:**
- Consumes: `Limiter`, `geo.HaversineM`, same tables as `Scan`.
- Produces: `func Here(db *sql.DB, lim Limiter, ip string, studentID, classID int, lat, lng float64, t time.Time) (sessionID int, distM float64, err error)`; errors `ErrNoBeacon` (none live), `ErrOutOfFence`, `ErrRateLimited`, `ErrSessionClosed`, `ErrNotMember` (new sentinel for unapproved/unknown membership).

- [ ] **Step 1: Write the failing test**

In `api/internal/attend/here_test.go`:

```go
package attend

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestHereNoBeacon(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery("FROM class_memberships").WithArgs(7, 3).
		WillReturnRows(sqlmock.NewRows([]string{"one"}).AddRow(1))
	mock.ExpectQuery("FROM qr_sessions").WithArgs(3).
		WillReturnRows(sqlmock.NewRows([]string{"id", "totp_secret", "geofence_lat", "geofence_lng", "geofence_radius_m"}))

	_, _, err = Here(db, NewMemLimiter(10, time.Minute), "192.0.2.9", 7, 3, 3.1390, 101.6869, time.Now())
	if err != ErrNoBeacon {
		t.Fatalf("expected ErrNoBeacon, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHereFarGPS(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	secret := []byte("0123456789abcdef0123456789abcdef")
	mock.ExpectQuery("FROM class_memberships").WithArgs(7, 3).
		WillReturnRows(sqlmock.NewRows([]string{"one"}).AddRow(1))
	mock.ExpectQuery("FROM qr_sessions").WithArgs(3).
		WillReturnRows(sqlmock.NewRows([]string{"id", "totp_secret", "geofence_lat", "geofence_lng", "geofence_radius_m"}).
			AddRow(5, secret, 3.1390, 101.6869, 30))

	_, _, err = Here(db, NewMemLimiter(10, time.Minute), "192.0.2.9", 7, 3, 3.1480, 101.6869, time.Now())
	if err != ErrOutOfFence {
		t.Fatalf("expected ErrOutOfFence, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run from `api/`: `go test ./internal/attend/ -run 'TestHere' -v`
Expected: FAIL — `undefined: Here` / `undefined: ErrNoBeacon`.

- [ ] **Step 3: Write minimal implementation**

Append to `attend.go` (mirror `Scan` structure; membership check first, then live session, then distance, then insert + audit + notification):

```go
var ErrNotMember = errors.New("not a class member")

func Here(db *sql.DB, lim Limiter, ip string, studentID, classID int, lat, lng float64, t time.Time) (int, float64, error) {
	ok, err := lim.Allow("here:" + ip)
	if err != nil {
		return 0, 0, err
	}
	if !ok {
		return 0, 0, ErrRateLimited
	}

	var one int
	if err := db.QueryRow(
		`SELECT 1 FROM class_memberships WHERE user_id=$1 AND class_id=$2 AND status='approved'`,
		studentID, classID,
	).Scan(&one); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, 0, ErrNotMember
		}
		return 0, 0, err
	}

	var sessionID int
	var secret []byte
	var flat, flng float64
	var radius int
	err = db.QueryRow(
		`SELECT id, totp_secret, geofence_lat, geofence_lng, geofence_radius_m FROM qr_sessions JOIN classes ON classes.id = qr_sessions.class_id WHERE qr_sessions.class_id=$1 AND session_end IS NULL ORDER BY id DESC LIMIT 1`,
		classID,
	).Scan(&sessionID, &secret, &flat, &flng, &radius)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, 0, ErrNoBeacon
		}
		return 0, 0, err
	}
	_ = secret
	_ = t

	distM := geo.HaversineM(flat, flng, lat, lng)
	if distM > float64(radius) {
		return 0, distM, ErrOutOfFence
	}

	var recordID int
	err = db.QueryRow(
		`INSERT INTO attendance_records(student_id, class_id, session_id, status, scan_lat, scan_lng) VALUES ($1, $2, $3, 'present', $4, $5) ON CONFLICT(student_id, session_id) DO NOTHING RETURNING id`,
		studentID, classID, sessionID, lat, lng,
	).Scan(&recordID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sessionID, distM, nil
		}
		return 0, distM, err
	}

	if _, err := db.Exec(
		`INSERT INTO audit_logs(actor_id, action, entity, entity_id) VALUES ($1, 'attendance.here', 'attendance_record', $2)`,
		studentID, recordID,
	); err != nil {
		return 0, distM, err
	}
	if _, err := db.Exec(
		`INSERT INTO notifications(user_id, kind, title, body) VALUES ($1, 'attendance.present', 'Attendance marked', 'You were marked present')`,
		studentID,
	); err != nil {
		return 0, distM, err
	}

	return sessionID, distM, nil
}
```

Add `ErrNoBeacon = errors.New("no live session")` and `ErrNotMember` to the existing `var (...)` block. Rate key is `"here:"+ip` (separate bucket from scan — deliberate so beacon taps never starve QR scans and vice versa).

- [ ] **Step 4: Run tests to verify they pass**

Run from `api/`: `go test ./internal/attend/ -v`
Expected: PASS including both new tests plus existing Scan/Close tests.

- [ ] **Step 5: Commit**

```bash
git add api/internal/attend/attend.go api/internal/attend/here_test.go
git commit -m "feat: beacon here check-in logic"
```

---

### Task 2: HTTP — POST /here + GET /mine + mapping tests

**Files:**
- Modify: `api/internal/httpapi/server.go`
- Test: `api/internal/httpapi/here_test.go`

**Interfaces:**
- Consumes: `attend.Here`, `attend.ErrNoBeacon`, `attend.ErrNotMember`, existing limiters/errors.
- Produces:
  - `POST /v1/attendance/here` (student) body `{class_id (optional, 0=auto), lat, lng}` → 200 `{"status":"present","session_id":N,"distance_m":D}`; mappings: `ErrNotMember`→403 NOT_A_MEMBER; `ErrNoBeacon`→404 NO_BEACON; auto with >1 live beacon→400 AMBIGUOUS_BEACON; `ErrOutOfFence`→403 OUT_OF_GEOFENCE+distance_m; `ErrRateLimited`→429 RATE_LIMITED; `ErrSessionClosed`→410 (kept for parity though Here only reads open rows).
  - `GET /v1/classes/mine` (student) → 200 `{"classes":[{"id","class_name","status","beacon_live","beacon_session_id"}]}` (approved + pending; beacon_live only meaningful when approved).

- [ ] **Step 1: Write the failing mapping test**

In `api/internal/httpapi/here_test.go`:

```go
package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"atlas/internal/attend"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestHereNoBeaconMaps404(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery("FROM class_memberships").WithArgs(7, 3).
		WillReturnRows(sqlmock.NewRows([]string{"one"}).AddRow(1))
	mock.ExpectQuery("FROM qr_sessions").WithArgs(3).
		WillReturnRows(sqlmock.NewRows([]string{"id", "totp_secret", "geofence_lat", "geofence_lng", "geofence_radius_m"}))

	srv := newTestServer(db, attend.NewMemLimiter(10, time.Minute))
	req := httptest.NewRequest("POST", "/v1/attendance/here",
		strings.NewReader(`{"class_id":3,"lat":3.1390,"lng":101.6869}`))
	req.Header.Set("Authorization", "Bearer "+studentToken(t, "7"))
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "NO_BEACON") {
		t.Fatalf("expected NO_BEACON, got %s", rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
```

(`studentToken` helper already exists in `server_test.go`, same package.)

- [ ] **Step 2: Run test to verify it fails**

Run from `api/`: `go test ./internal/httpapi/ -run TestHereNoBeaconMaps404 -v`
Expected: FAIL — 404 no route.

- [ ] **Step 3: Write minimal implementation**

Routes:

```go
mux.Handle("POST /v1/attendance/here", s.prot(s.handleHere, "student"))
mux.Handle("GET /v1/classes/mine", s.prot(s.handleMine, "student"))
```

`handleHere` (exact):

```go
func (s *Server) handleHere(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ClassID int     `json:"class_id"`
		Lat     float64 `json:"lat"`
		Lng     float64 `json:"lng"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "BAD_REQUEST")
		return
	}
	studentID, err := strconv.Atoi(auth.UID(r))
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "UNAUTHORIZED")
		return
	}
	classID := req.ClassID
	if classID == 0 {
		rows, err := s.DB.Query(
			`SELECT q.class_id FROM qr_sessions q JOIN class_memberships m ON m.class_id = q.class_id AND m.status='approved' WHERE m.user_id=$1 AND q.session_end IS NULL`,
			studentID,
		)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "INTERNAL")
			return
		}
		var ids []int
		for rows.Next() {
			var id int
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				writeErr(w, http.StatusInternalServerError, "INTERNAL")
				return
			}
			ids = append(ids, id)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			writeErr(w, http.StatusInternalServerError, "INTERNAL")
			return
		}
		if len(ids) == 0 {
			writeErr(w, http.StatusNotFound, "NO_BEACON")
			return
		}
		if len(ids) > 1 {
			writeErr(w, http.StatusBadRequest, "AMBIGUOUS_BEACON")
			return
		}
		classID = ids[0]
	}
	sid, dist, herr := attend.Here(s.DB, s.Limiter, clientIP(r), studentID, classID, req.Lat, req.Lng, time.Now())
	switch {
	case errors.Is(herr, attend.ErrNotMember):
		writeErr(w, http.StatusForbidden, "NOT_A_MEMBER")
		return
	case errors.Is(herr, attend.ErrNoBeacon):
		writeErr(w, http.StatusNotFound, "NO_BEACON")
		return
	case errors.Is(herr, attend.ErrOutOfFence):
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "OUT_OF_GEOFENCE", "distance_m": dist})
		return
	case errors.Is(herr, attend.ErrRateLimited):
		writeErr(w, http.StatusTooManyRequests, "RATE_LIMITED")
		return
	case errors.Is(herr, attend.ErrSessionClosed):
		writeErr(w, http.StatusGone, "SESSION_CLOSED")
		return
	case herr != nil:
		writeErr(w, http.StatusInternalServerError, "INTERNAL")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "present", "session_id": sid, "distance_m": dist})
}
```

`handleMine` (exact): one query, student-scoped:

```sql
SELECT c.id, c.class_name, m.status, EXISTS(SELECT 1 FROM qr_sessions q WHERE q.class_id = c.id AND q.session_end IS NULL) AS live, (SELECT q2.id FROM qr_sessions q2 WHERE q2.class_id = c.id AND q2.session_end IS NULL ORDER BY q2.id DESC LIMIT 1) AS live_sid FROM class_memberships m JOIN classes c ON c.id = m.class_id WHERE m.user_id=$1 ORDER BY c.id
```

Scan `live_sid` as `sql.NullInt64` (null when no beacon); response objects `{"id","class_name","status","beacon_live","beacon_session_id"}` with `beacon_session_id` null when none.

- [ ] **Step 4: Run tests to verify they pass**

Run from `api/`: `go test ./...`
Expected: PASS all packages.

- [ ] **Step 5: Commit**

```bash
git add api/internal/httpapi/server.go api/internal/httpapi/here_test.go
git commit -m "feat: here and my-classes endpoints"
```

---

### Task 3: Student dashboard — My classes + I-am-here button + polling

**Files:**
- Modify: `web/student.html` (new section + script; QR/absence/join-card byte-identical)
- Modify: `web/app.js` (add `Atlas.hereError`)

**Interfaces:**
- Consumes: `GET /v1/classes/mine`, `POST /v1/attendance/here`, existing auto-GPS pattern.
- Produces: per-class beacon button; 10s poll only while check-in tab visible; `Atlas.hereError(status, data)` mapping 404 NO_BEACON→"No live session…", 400 AMBIGUOUS_BEACON→"Several sessions live — enter Session ID…", 403 OUT_OF_GEOFENCE→distance, 403 NOT_A_MEMBER, 429, 410 (reuse scan wording).

- [ ] **Step 1: Add hereError to app.js**

```js
  function hereError(status, data) {
    data = data || {};
    if (status === 404 || data.error === 'NO_BEACON') return 'No live session (404) — ask your teacher to open the session.';
    if (status === 400 || data.error === 'AMBIGUOUS_BEACON') return 'Several sessions are live — enter the Session ID in Check in, or tap the right class below.';
    if (status === 403 && data.error === 'NOT_A_MEMBER') return 'Not a member (403) — join the class and wait for approval first.';
    return scanError(status, data);
  }
```

Export it (fallthrough reuses `scanError` for 403-distance/429/410 shapes, which are identical).

- [ ] **Step 2: Add My-classes section + beacon script to student.html**

Insert as the first `<section>` in `#tab-checkin` (above join card):

```html
    <section>
      <div class="section-title">
        <div class="section-title__icon">🏫</div>
        <h2>My classes</h2>
      </div>
      <div class="flex items-center gap-2 flex-wrap" style="margin-bottom:.6rem">
        <button type="button" id="clsReload" class="btn btn--secondary btn--sm">Refresh</button>
        <span id="clsHint" class="text-muted text-sm"></span>
      </div>
      <ul id="myclasses" class="item-list">
        <li><span class="empty" style="padding:1.25rem 0;display:block">Loading classes…</span></li>
      </ul>
    </section>
```

Script (inside student IIFE; names `loadMyClasses`, `beaconPollT` — no clashes with `loadMine`/`loadReqs`):

```js
  var beaconPollT = null;
  Atlas.el('clsReload').onclick = loadMyClasses;
  loadMyClasses();
  clearInterval(beaconPollT);
  beaconPollT = setInterval(function () {
    if (document.getElementById('tab-checkin').classList.contains('active')) loadMyClasses(true);
  }, 10000);

  async function loadMyClasses(quiet) {
    var list = Atlas.el('myclasses');
    var r = await Atlas.api('GET', '/v1/classes/mine');
    if (r.status !== 200) {
      if (!quiet) list.innerHTML = '<li><span class="empty">Could not load classes (' + r.status + ').</span></li>';
      return;
    }
    var mine = r.data.classes || [];
    if (!mine.length) {
      list.innerHTML = '<li><span class="empty">No classes yet — open your teacher\u2019s join link to request access.</span></li>';
      Atlas.el('clsHint').textContent = '';
      return;
    }
    var live = mine.filter(function (c) { return c.status === 'approved' && c.beacon_live; }).length;
    Atlas.el('clsHint').textContent = live ? live + ' live session' + (live > 1 ? 's' : '') + ' — tap I am here.' : 'No live sessions right now.';
    list.innerHTML = '';
    mine.forEach(function (c) {
      var li = document.createElement('li');
      var badge = c.status === 'approved' ? 'badge--green' : c.status === 'pending' ? 'badge--yellow' : 'badge--red';
      li.innerHTML = '<span class="font-semibold text-sm">' + escapeH(c.class_name) + '</span>' +
        ' <span class="badge ' + badge + '">' + c.status + '</span>' +
        (c.status === 'approved' && c.beacon_live ? ' <span class="badge badge--blue">● live</span>' : '');
      if (c.status === 'approved' && c.beacon_live) {
        var b = document.createElement('button');
        b.className = 'btn btn--primary btn--sm';
        b.style.marginLeft = 'auto';
        b.textContent = 'I am here';
        b.onclick = function () { tapHere(c.id, b); };
        li.appendChild(b);
      } else {
        var s = document.createElement('span');
        s.className = 'text-muted text-sm ml-auto';
        s.textContent = c.status === 'approved' ? 'no session' : 'awaiting approval';
        li.appendChild(s);
      }
      list.appendChild(li);
    });
  }

  async function tapHere(classId, btn) {
    btn.classList.add('loading'); btn.disabled = true;
    if (!Atlas.el('lat').value) autoGPS(true);
    var r = await Atlas.api('POST', '/v1/attendance/here', {
      class_id: classId, lat: +Atlas.el('lat').value, lng: +Atlas.el('lng').value
    });
    btn.classList.remove('loading'); btn.disabled = false;
    if (r.status !== 200) { Atlas.msg('m', Atlas.hereError(r.status, r.data), true); return; }
    Atlas.msg('m', 'Marked present for session #' + r.data.session_id + ' (' + r.data.distance_m + ' m).', false);
  }

  function escapeH(s) { return String(s == null ? '' : s).replace(/[&<>"']/g, function (c) { return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]; }); }
```

`autoGPS` already exists in student page (prior work) — reuse it, do not redefine. If the current file names it differently, reuse whatever exists; never add a second GPS function.

- [ ] **Step 3: Manual verify**

Teacher opens session → student My-classes shows ● live + I am here → tap → present toast with session + distance; teacher closes → next poll shows "No live sessions"; two live sessions + class_id omitted path is server-side (button always sends class_id — auto path covered by API test only).
Expected: no console errors; QR/absence/join sections untouched.

- [ ] **Step 4: Commit**

```bash
git add web/student.html web/app.js
git commit -m "feat: student beacon classes and I-am-here button"
```

---

### Task 4: Full verification

**Files:** none.

- [ ] **Step 1: Run backend suite**

Run from `api/`: `go test ./...`
Expected: PASS (enrollment plan suite + here tests).

- [ ] **Step 2: Manual matrix**

Join→approve→beacon→here→close→absent; QR scan side-by-side on same session (both yield present, no doubles); far GPS here → 403 + distance; no beacon → 404 message; teacher dashboard links still copy correctly.
Expected: all green.

## Self-Review

- Spec §5.2 `POST /here` + auto/ambiguous + error shapes → Tasks 1+2. Covered.
- Spec §5.2 `GET /mine` with beacon_live → Task 2. Covered (`beacon_session_id` included for QR-tab deep-linking later; unused fields are fine, documented).
- Spec §5.4 student beacon button + polling + `?join=` interplay (join card is Plan 1) → Task 3. Covered; `tapHere` always sends class_id so AMBIGUOUS path is API-tested, UI never hits it — stated explicitly.
- Spec §5.5 coexistence (conflict-safe double-write) → Task 1 insert mirrors Scan's `ON CONFLICT DO NOTHING`; verified in Task 4 matrix. Covered.
- Spec §7 here-errors → Tasks 1–3 (`hereError` falls through to `scanError` for shared shapes — consistent, not a placeholder).
- Spec §9 → Tasks 1+2 tests, Task 4 manual. Covered.
- Type consistency: `Here→(sessionID int, distM float64, err)`, `beacon_live bool`, `beacon_session_id int|null`, `hereError(status, data)→string`, status/note literals match Plan 1 (`approved`, `pending`). `here:` rate-key prefix distinct from `scan:` — deliberate, documented in Task 1.
