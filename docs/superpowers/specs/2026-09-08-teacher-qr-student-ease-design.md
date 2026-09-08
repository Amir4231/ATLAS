# Design: Teacher QR Access + Easier Student Check-in + Join Link

Date: 2026-09-08
Status: Approved
Scope: ATLAS attendance — teacher QR parity, student camera scan, session join link

## 1. Problem
- Only `class_rep`/`rep`/`admin` can open QR sessions and fetch live codes. Teachers cannot generate QR despite owning absence review and analytics.
- Student check-in is high-friction: hand-type Session ID + hand-type 6-digit rotating code + tap Use-my-location + Check in. No camera. Rep QR encodes raw code only, so session ID is never auto-filled.
- No shareable way for teacher/rep to send students into the right session.

## 2. Goals
- Teacher gets full session control identical to class rep: open, live QR + code + timer, live feed, close, copy join link.
- Student check-in becomes: open link or scan QR -> GPS auto-locked -> one tap Check in. Manual typing kept as fallback.
- Join link shareable by teacher and rep, stable for session lifetime, does not weaken anti-proxy.
- No DB/migration change. Small JS, no build step. Keep Go memory budget.

## 3. Non-goals
- No auto-check-in from link alone. Link carries session only, never a valid code.
- No change to TOTP period (5s), geofence math, rate limiting, absence pipeline.
- No new auth roles. No push/email change.

## 4. Architecture
- Backend: widen role gate on 3 QR routes to include `teacher`.
- Frontend contract: QR payload JSON `{"s":<session_id>,"c":"<code>"}`. Student parser accepts JSON or legacy raw code.
- Join link format: `<origin>/student.html?session=<id>` (code-free). Student boots from `?session=`.
- Student scanner: native `BarcodeDetector` when available, else `jsQR` CDN fallback (lazy-loaded only on Scan tap). No always-on dependency.
- GPS: auto-request on student page load and on `?session=` boot, plus existing manual button. Badge shows lock state.

## 5. Components

### 5.1 Backend (`api/internal/httpapi/server.go`)
- `POST /v1/qr/session`: allowed `class_rep, rep, admin, teacher`
- `GET /v1/qr/session/{id}/code`: allowed `class_rep, rep, admin, teacher`
- `POST /v1/qr/session/{id}/close`: allowed `class_rep, rep, admin, teacher`
- No handler logic change. `prot()` already enforces JWT + FORBIDDEN.
- Test: extend role-gate test to assert teacher allowed on open/code/close, student still FORBIDDEN on open, rep still FORBIDDEN on absence list.

### 5.2 QR payload (rep + teacher)
- `drawQR(sessionId, code)`: `qrObj.makeCode(JSON.stringify({s:sessionId,c:code}))`.
- Visible large code text stays raw `code` for manual typing.
- `Copy code` button (clipboard) + `Copy join link` button (`student.html?session=` absolute URL). Fallback text select if clipboard blocked.
- Countdown timer unchanged (exp from `/code`, urgent <=8s).

### 5.3 Teacher page (`web/teacher.html`)
- Keep existing Session-rate + Pending queue sections unchanged.
- Add new top section Session control + QR hero + Live feed, mirroring `rep.html`: class `<select>` from `GET /v1/classes`, Open/Close, `sessLabel`, `#qr`, `#code`, `#cd`, `#feed`.
- Guard: `requireRole(['teacher','admin'])`. Admin keeps access everywhere.
- Share buttons: Copy code, Copy join link.

### 5.4 Rep page (`web/rep.html`)
- Add Copy code + Copy join link buttons under QR hero. Keep all existing behavior.
- Switch `drawQR(code)` to `drawQR(sessionId, code)` JSON payload.

### 5.5 Student page (`web/student.html`)
- On load: parse `?session=` -> prefill `#sess` + `#asess`, auto-trigger GPS lock, show hint "Joined session #X via link — scan QR or enter code".
- New Scan QR block: `<video playsinline>`, Start scan / Stop, `<canvas hidden>` for jsQR fallback, status line.
- Scan success: parse payload -> set `#sess` + `#code`, stop stream, flash ok message, keep GPS fields for one-tap submit.
- Parser: try `JSON.parse`; if `o.s` + `o.c` use them; else if `/^\d{6}$/` treat as code only; else show raw + ask for session.
- GPS auto-lock on boot (if permission granted silently; on deny show manual button, no hard error).
- Check-in submit unchanged (`POST /v1/attendance/scan`), reuses `Atlas.scanError`.
- Keep manual Session + Code inputs as fallback. Keep absence tab unchanged except `#asess` prefill.

## 6. Data flow
1. Teacher/rep: Open session -> `session_id`.
2. Poll `/code` every 5s -> `{code, exp}` -> render JSON QR + timer.
3. Share: copy `student.html?session=id` via messenger/whiteboard; students open link.
4. Student: link prefills session + GPS; camera scan fills code; POST scan with `{session_id, code, lat, lng}`.
5. Server validates TOTP window (c-1,c,c+1), geofence, rate limit -> present.
6. Close session marks remaining `absent_unexcused`.

## 7. Error handling
- Camera unavailable/denied: show "Camera blocked — type code manually" + keep manual open.
- QR CDN (qrcodejs) blocked: existing fallback text retained.
- jsQR CDN blocked + no BarcodeDetector: scan button disabled with hint.
- INVALID_CODE 401: "check code with rep — codes refresh every 5s".
- OUT_OF_GEOFENCE 403: show distance_m.
- RATE_LIMITED 429, SESSION_CLOSED 410: reuse existing wording.
- Clipboard blocked: select text for manual copy.

## 8. Security
- Join link alone cannot mark present: still needs fresh TOTP code (5s rotation, +-1 window) + in-geofence GPS + student JWT + rate limit.
- No code in URL, so screenshots of links do not bypass TOTP.
- Teacher widening is least-privilege: only QR session routes, absence review already teacher-owned.

## 9. Testing
- `go test ./...` (existing scan 401/403/429, auth, role gates + new teacher-QR gate assertions).
- Manual: teacher open -> QR visible -> copy link opens student with session prefilled -> phone camera scan fills code -> check-in <2s -> close marks absent.
- Fallbacks: block camera, block CDN, expired code, far GPS.

## 10. Rollout
- Frontend only + 3-line backend role change. No migration. Static `web/` redeploy + Go redeploy.
- Back-compat: old raw-code QR still scans (parser fallback). Old student manual flow still works.
