# Teacher QR + Student Ease Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Teachers can generate QR sessions like reps, and students can join via link + camera scan with auto-GPS.

**Architecture:** Widen 3 QR routes to `teacher`, switch QR payload to `{"s":id,"c":code}` JSON with back-compat parser, add copy-join-link buttons to rep/teacher, add teacher QR section mirroring rep, add student scanner + `?session=` boot + auto-GPS.

**Tech Stack:** Go 1.x + sqlmock tests, vanilla JS no build, qrcodejs CDN, native BarcodeDetector with jsQR CDN fallback, nginx same-origin `/v1/`.

## Global Constraints

- No DB migration; no change to TOTP period (5s), geofence math, rate limiting.
- Join link is code-free: `student.html?session=<id>` only.
- QR JSON is `{"s":<int>,"c":"<6-digit>"}`; raw 6-digit codes must still scan.
- Keep manual Session + Code inputs as fallback on student page.
- Frontend has no build step; edit `web/*.html` + `web/app.js` directly.

---

## File Structure

- `api/internal/httpapi/server.go:65-68` — widen role gates (3 lines). Single responsibility: route auth.
- `api/internal/httpapi/server_test.go` — add teacher-QR gate test. Single responsibility: prove gate.
- `web/app.js` — add shared `Atlas.parseQRPayload(text)` + `Atlas.joinLink(sessionId)` + `Atlas.sessionFromQuery()`. Single responsibility: shared QR/link helpers.
- `web/rep.html` — switch `drawQR` to JSON, add Copy code + Copy join link buttons. Single responsibility: rep session display.
- `web/teacher.html` — add Session-control + QR hero + Live feed section mirroring rep. Single responsibility: teacher session display + existing review queue untouched.
- `web/student.html` — add Scan QR block (`<video>`, canvas), `?session=` boot, auto-GPS, tolerant parser. Single responsibility: easy check-in.

---

### Task 1: Backend — allow teacher on QR session routes + gate test

**Files:**
- Modify: `api/internal/httpapi/server.go:65-68`
- Test: `api/internal/httpapi/server_test.go`

**Interfaces:**
- Consumes: existing `s.prot(h, allowed...)` JWT role gate.
- Produces: `POST /v1/qr/session`, `GET /v1/qr/session/{id}/code`, `POST /v1/qr/session/{id}/close` accept `teacher` in addition to `class_rep, rep, admin`. No signature change.

- [ ] **Step 1: Write the failing test**

In `api/internal/httpapi/server_test.go` append:

```go
func TestTeacherCanOpenQRSessionGate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	srv := newTestServer(db, attend.NewMemLimiter(10, time.Minute))
	h := srv.Routes()

	mkToken := func(role string) string {
		tok, err := auth.Token("11", role, testSecret)
		if err != nil {
			t.Fatal(err)
		}
		return tok
	}

	tr := httptest.NewRequest("POST", "/v1/qr/session", strings.NewReader(`{}`))
	tr.Header.Set("Authorization", "Bearer "+mkToken("teacher"))
	trRec := httptest.NewRecorder()
	h.ServeHTTP(trRec, tr)
	if trRec.Code == http.StatusForbidden {
		t.Fatalf("teacher open session: expected pass-through (400/200), got 403 %s", trRec.Body.String())
	}

	sr := httptest.NewRequest("POST", "/v1/qr/session", strings.NewReader(`{}`))
	sr.Header.Set("Authorization", "Bearer "+mkToken("student"))
	srRec := httptest.NewRecorder()
	h.ServeHTTP(srRec, sr)
	if srRec.Code != http.StatusForbidden {
		t.Fatalf("student open session: expected 403, got %d %s", srRec.Code, srRec.Body.String())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/httpapi/ -run TestTeacherCanOpenQRSessionGate -v`
Expected: FAIL with teacher 403 before fix. Run from `api/` dir.

- [ ] **Step 3: Write minimal implementation**

In `api/internal/httpapi/server.go:65-68` replace with:

```go
	mux.Handle("POST /v1/qr/session", s.prot(s.handleOpenSession, "class_rep", "rep", "admin", "teacher"))
	mux.Handle("GET /v1/qr/session/{id}/code", s.prot(s.handleGetCode, "class_rep", "rep", "admin", "teacher"))
	mux.Handle("POST /v1/attendance/scan", s.prot(s.handleScan, "student"))
	mux.Handle("POST /v1/qr/session/{id}/close", s.prot(s.handleCloseSession, "class_rep", "rep", "admin", "teacher"))
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./...`
Expected: PASS all packages.

- [ ] **Step 5: Commit**

```bash
git add api/internal/httpapi/server.go api/internal/httpapi/server_test.go
git commit -m "feat: allow teacher to open and display QR sessions"
```

---

### Task 2: Shared frontend helpers — QR parse + join link

**Files:**
- Modify: `web/app.js`

**Interfaces:**
- Consumes: nothing new.
- Produces:
  - `Atlas.parseQRPayload(text) -> {sessionId, code}` accepts `{"s":42,"c":"123456"}`, `{"session_id":42,"code":"123456"}`, raw `123456`.
  - `Atlas.joinLink(sessionId) -> string` absolute URL to `student.html?session=<id>`.
  - `Atlas.sessionFromQuery() -> number` reads `?session=` or `?session_id=`.

- [ ] **Step 1: Add helpers to web/app.js**

Before `window.Atlas = {` insert:

```js
  function parseQRPayload(text) {
    var raw = (text || '').trim();
    if (!raw) return { sessionId: 0, code: '' };
    try {
      var o = JSON.parse(raw);
      var sid = o.s != null ? +o.s : o.session_id != null ? +o.session_id : 0;
      var code = o.c != null ? String(o.c).trim() : o.code != null ? String(o.code).trim() : '';
      if (code || sid) return { sessionId: sid || 0, code: code || '' };
    } catch (e) {}
    return { sessionId: 0, code: raw };
  }

  function joinLink(sessionId) {
    var base = location.href.split('?')[0].split('#')[0];
    var dir = base.slice(0, base.lastIndexOf('/') + 1);
    return dir + 'student.html?session=' + encodeURIComponent(sessionId);
  }

  function sessionFromQuery() {
    try {
      var q = new URLSearchParams(location.search);
      return +(q.get('session') || q.get('session_id')) || 0;
    } catch (e) { return 0; }
  }
```

Extend export to include `parseQRPayload, joinLink, sessionFromQuery`.

- [ ] **Step 2: Verify in browser console**

Open any page, run `Atlas.parseQRPayload('{"s":42,"c":"123456"}')`, `Atlas.parseQRPayload('123456')`, `Atlas.joinLink(42)`.
Expected: session+code object, raw-code fallback, URL ending `student.html?session=42`.

- [ ] **Step 3: Commit**

```bash
git add web/app.js
git commit -m "feat: add shared QR parse and join-link helpers"
```

---

### Task 3: Rep page — JSON QR + copy buttons

**Files:**
- Modify: `web/rep.html`

**Interfaces:**
- Consumes: `Atlas.joinLink(sessionId)`, `GET /v1/qr/session/{id}/code`.
- Produces: QR encodes JSON, visible code stays raw, Copy code + Copy join link buttons.

- [ ] **Step 1: Add buttons to QR hero**

After `<div class="qr-hero__code" id="code">` insert Copy code + Copy join link buttons + `#shareHint` paragraph.

- [ ] **Step 2: Switch drawQR to JSON + wire copy**

Replace `drawQR(code)` call with `drawQR(sessionId, code)` plus `shareHint` update to `Atlas.joinLink(sessionId)`. Replace `qrObj.makeCode(code)` with `qrObj.makeCode(JSON.stringify({ s: sid, c: code }))`. Add `copyText` + `fallbackCopy` with clipboard API and execCommand fallback.

- [ ] **Step 3: Manual verify**

Serve `web/`, open `rep.html` as rep, Open session, confirm QR refreshes every 5s, Copy join link yields `student.html?session=N`.
Expected: QR decoded text is JSON.

- [ ] **Step 4: Commit**

```bash
git add web/rep.html
git commit -m "feat: rep QR encodes session+code with join-link copy"
```

---

### Task 4: Teacher page — full session control

**Files:**
- Modify: `web/teacher.html`

**Interfaces:**
- Consumes: `Atlas.joinLink`, `GET /v1/classes`, `POST /v1/qr/session`, `GET .../code`, `POST .../close`, `GET /v1/attendance`.
- Produces: teacher open/QR/feed/close UI. Existing rate + queue unchanged. Guard stays `requireRole(['teacher','admin'])`.

- [ ] **Step 1: Add session-control + QR hero + feed sections**

Insert new section with `#cls`, `#open`, `#close`, `#m0` before Attendance-rate section. Add `#qrSection` (`#qr`, `#code`, `#copyCode`, `#copyLink`, `#shareHint`, `#cd`) and `#feedSection` (`#feed`). Use `#m0` to avoid collision with existing `#m`.

- [ ] **Step 2: Add qrcodejs + QR logic**

Add qrcodejs CDN script. Append open/tick/close/feed/copy logic mirroring rep: `tickQR` renders `JSON.stringify({s:sessionId,c:code})`, `shareHint` shows join link, feed polls attendance, countdown with urgent <=8s.

- [ ] **Step 3: Manual verify**

Login as teacher, Open session returns 200, QR visible, join link opens student with session prefilled, Close shows marked count, queue still works.
Expected: no 403 for teacher.

- [ ] **Step 4: Commit**

```bash
git add web/teacher.html
git commit -m "feat: teacher can generate QR sessions with join link"
```

---

### Task 5: Student page — join link boot + auto-GPS + camera scan

**Files:**
- Modify: `web/student.html`

**Interfaces:**
- Consumes: `Atlas.parseQRPayload`, `Atlas.sessionFromQuery`, `POST /v1/attendance/scan`.
- Produces: `?session=` prefill, auto-GPS, Scan QR video flow filling `#sess` + `#code`.

- [ ] **Step 1: Add Scan QR UI block**

Insert `#scanBtn`, `#stopBtn`, `#scanStatus`, `<video id="cam">`, `<canvas id="frame">` before `<form id="scan">`.

- [ ] **Step 2: Add boot + scanner script**

Add join-link boot (prefill `#sess` + `#asess`, hint message, `autoGPS(true)`), `autoGPS(silent)` updating `#lat/#lng/#gpsBadge`, `startScan/stopScan/beaconLoop/jsqrLoop/onScanned` using BarcodeDetector primary + lazy jsQR fallback. `onScanned` uses `Atlas.parseQRPayload`.

- [ ] **Step 3: Manual verify**

Open `student.html?session=1` → sess=1, GPS auto-attempts; Scan QR → camera → QR fills sess+code → Check in succeeds; deny camera → manual fallback; raw code typing still works.
Expected: no console errors.

- [ ] **Step 4: Commit**

```bash
git add web/student.html
git commit -m "feat: student join-link boot, auto-GPS, and QR camera scan"
```

---

### Task 6: Full verification

**Files:** none.

- [ ] **Step 1: Run backend suite**

Run: `go test ./...` from `api/`.
Expected: PASS.

- [ ] **Step 2: Static check frontend**

Serve `web/`, confirm no duplicate IDs on teacher page, qrcode loads on rep+teacher, jsQR lazy-loads only on Scan.
Expected: clean tree, 5 feature commits + spec.

## Self-Review

- Spec 5.1 teacher routes → Task 1. Covered.
- Spec 5.2 JSON QR + copy → Tasks 2+3+4. Covered.
- Spec 5.3 teacher mirror → Task 4. Covered.
- Spec 5.4 rep JSON + link → Task 3. Covered.
- Spec 5.5 student boot + scanner + GPS + parser → Tasks 2+5. Covered.
- Spec 6 flow, 7 errors, 8 security (code-free link) → Tasks 3-5. Covered.
- Spec 9 testing → Tasks 1+6. Covered.
- No placeholders. Type consistency: `Atlas.parseQRPayload(text)->{sessionId,code}`, `Atlas.joinLink(id)->string`, `Atlas.sessionFromQuery()->number` used identically.
