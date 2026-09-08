# Design: Class Join Links + Beacon "I Am Here" Attendance

Date: 2026-09-08
Status: Approved by user
Scope: ATLAS attendance — multi-class enrollment via shareable links with teacher approval, plus one-tap beacon check-in coexisting with QR flow

## 1. Problem
- Class membership today is a single `users.class_id` column assigned by admin/seed. There is no way for a student to join a class themselves, and a student can belong to only one class.
- Check-in requires typing a rotating 6-digit code (QR flow). Users want a simpler path: teacher "lights up the beacon" (starts something visible), nearby students tap one "I am here" button.
- Teachers need a shareable class link that stays available on their dashboard.

## 2. Goals
- Student opens a class link, requests to join, teacher approves; student can belong to multiple classes with a "My classes" dashboard list.
- While a session is open (beacon lit), approved members see a prominent "I am here" button; one tap + GPS marks them present. No code typing.
- Existing QR-code flow keeps working unchanged as the second check-in method.
- Join links are long-lived and code-free; teacher approval (not link secrecy) is the gate.

## 3. Non-goals
- No removal of the QR/TOTP flow, no change to TOTP period (5s), geofence math, rate limiting, absence pipeline, notifications.
- No auto-check-in: the student must press the button each session.
- No roster import or email allowlist management.

## 4. Architecture
- New `class_memberships` table is the source of truth for enrollment: `(user_id, class_id)` unique, `status` pending/approved/rejected, timestamps.
- One SQL migration creates the table and backfills one approved row per existing `users.class_id`. `users.class_id` remains as legacy data, no longer read by attendance logic.
- Beacon state reuses `qr_sessions`: a session with `session_end IS NULL` is "lit". No new session machinery.
- New `POST /v1/attendance/here` mirrors `attend.Scan` minus TOTP: resolve live session, geofence check, conflict-safe present insert, same error shapes.

## 5. Components

### 5.1 Migration (`migrations/`)
- `CREATE TABLE class_memberships(user_id INT REFERENCES users, class_id INT REFERENCES classes, status TEXT DEFAULT 'pending', created_at TIMESTAMPTZ DEFAULT now(), UNIQUE(user_id, class_id))`.
- Backfill: `INSERT INTO class_memberships(user_id, class_id, status) SELECT id, class_id, 'approved' FROM users WHERE class_id IS NOT NULL ON CONFLICT DO NOTHING`.
- Switch session-close absent-marking from `JOIN users u ON u.class_id = q.class_id` to `JOIN class_memberships m ON m.class_id = q.class_id AND m.status='approved' JOIN users u ON u.id = m.user_id`.

### 5.2 Backend (`api/internal/httpapi/server.go` + new `api/internal/enroll/` package)
- `POST /v1/classes/{id}/join` (student): insert pending membership; idempotent — returns current status if row exists (`already_member` / `already_pending` still 200 with status field).
- `GET /v1/classes/{id}/join-requests?status=pending` (teacher of that class, admin): list pending requests with student name/email.
- `PATCH /v1/classes/requests/{reqId}` (teacher of that class, admin) body `{decision: approved|rejected}`: flip status; only pending rows transition (else 409 NOT_PENDING, mirroring absence review).
- `GET /v1/classes/mine` (student): approved/pending classes with `beacon_live` bool each (true when an open session exists for that class) — one query powering the whole student dashboard.
- `GET /v1/classes/teaching` (teacher, admin): classes they own with `pending_count` + join-link path, powering dashboard cards.
- `POST /v1/attendance/here` (student) body `{class_id, lat, lng}`: resolve caller's approved membership; find open session for that class (404 NO_BEACON if none); geofence check (403 OUT_OF_GEOFENCE + distance_m); rate limit per IP (429 RATE_LIMITED); insert present conflict-safe; 410 SESSION_CLOSED if session ended mid-request. If `class_id` omitted and exactly one live beacon exists among member classes, auto-select it; if several, 400 AMBIGUOUS_BEACON.
- Teacher authorization for join-request endpoints: requester's class must have `homeroom_teacher_id = caller` (admin bypasses).

### 5.3 Teacher dashboard (`web/teacher.html`)
- New "My classes" section above the session block: one card per teaching class with class name, member count, pending count badge, **Copy join link** button (absolute `student.html?join=<id>` via shared helper), and inline pending queue with Approve/Reject buttons (same card pattern as absence queue).
- Existing session-control/QR, rate, and absence-queue sections unchanged.

### 5.4 Student dashboard (`web/student.html`)
- New "My classes" section (default first tab content): each member class shows name, membership status, beacon state. When `beacon_live`, a large **"I am here"** button with GPS badge (reuse auto-GPS pattern: lock on page load, show accuracy).
- `?join=<id>` boot: prefill/hide to a "Join class" card showing class name + Request-to-join button; after requesting, show pending state. Join card links back to dashboard list.
- Existing QR check-in tab and absence tab unchanged.
- Shared helper addition in `web/app.js`: `Atlas.classJoinLink(classId)`, `Atlas.joinClassFromQuery()` (parse `?join=`), mirroring existing `joinLink`/`sessionFromQuery` helpers.

### 5.5 QR coexistence
- Both `attendance/scan` and `attendance/here` insert `status='present'` with `ON CONFLICT(student_id, session_id) DO NOTHING`, so using both methods never double-marks; second call returns present.
- Close-session behavior identical for both: unmarked approved members become `absent_unexcused`.

## 6. Data flow
1. Teacher copies class join link from dashboard, shares it (chat, board, printed QR).
2. Student opens `student.html?join=7` → sees class name → Request to join → pending row.
3. Teacher approves → student sees class in "My classes" as member.
4. Teacher taps Open session (existing button = beacon lit).
5. Member dashboard polls `GET /v1/classes/mine` (every ~10s while visible) → beacon_live true → big "I am here" button appears.
6. Student taps → GPS + `POST /v1/attendance/here` → present (or explicit error: too far, no beacon, rate-limited).
7. Teacher closes session → remaining members marked absent; existing review/analytics flow applies.

## 7. Error handling
- Join: unknown class 404; duplicate request idempotent 200 with status; non-student role 403.
- Review: non-pending 409 NOT_PENDING; wrong teacher 403 (not your class).
- Here: 404 NO_BEACON (nothing lit), 400 AMBIGUOUS_BEACON (several lit, pick one), 403 OUT_OF_GEOFENCE + distance_m, 429 RATE_LIMITED, 410 SESSION_CLOSED, 403 NOT_A_MEMBER (pending/rejected taps button — button hidden client-side, enforced server-side).
- Reuse `Atlas.scanError`-style wording; add `Atlas.hereError` mapping the two new codes.

## 8. Security
- Join link carries only a class id; entry requires teacher approval, so leaked links are harmless (worst case: stranger in pending queue, visible to teacher).
- Beacon check-in requires: valid student JWT + approved membership + temporally live session + in-geofence GPS + per-IP rate limit. No rotating code, so marginally weaker than QR against a determined on-site proxy — accepted trade-off, QR retained for high-stakes sessions.
- Teacher scoping enforced per-request (homeroom ownership), not just role.

## 9. Testing
- Go: enroll request/approve/reject transitions incl. idempotent re-join and NOT_PENDING; here-handler 404/400/403+distance/429/410 mapping; close-session marks members (not `users.class_id`); teacher-scoping (other teacher 403).
- Manual: teacher copies link → logged-out/student opens → request → approve → open session → student sees beacon → I-am-here → present; close → absent for non-tappers; QR path still works side by side.
- `go test ./...` green before merge.

## 10. Rollout
- Deploy migration first (backfill keeps current behavior: same members marked absent as before), then API, then static `web/`.
- Back-compat: old clients ignore new endpoints; `users.class_id` untouched so rollback is migration-down + redeploy.
- Phasable: (1) memberships + join/approve + dashboards, (2) beacon endpoint + student button. Each phase independently testable.
