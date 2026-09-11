# Task 10 Brief: Minimal static PWA

## Plan source
docs/superpowers/plans/2026-09-08-atlas.md, Task 10 (verbatim below).

## Task text (single source of requirements)

**Files:**
- Create: `web/index.html`, `web/rep.html`, `web/student.html`, `web/teacher.html`, `web/admin.html`, `web/app.js`, `web/manifest.json`, `web/sw.js`

Pages: login+role router; rep QR (poll code endpoint every 5s, draw via canvas QR lib CDN, live feed list); student scan form (code + geolocation GPS lock badge) + absence upload form; teacher queue with doc iframe + approve/reject + rate cards; admin class form + audit table. Service worker caches app shell.

- [ ] **Step 1: Build pages + JS**
- [ ] **Step 2: Serve via Nginx, click-through**
Login as each seed role, complete one full session. Expected: rep sees check-ins live, teacher approves, student status flips.
- [ ] **Step 3: Commit** `feat(atlas): minimal role PWA`.

## Actual API surface (controller-verified from api/internal/httpapi/server.go — use these exact routes)
- POST /v1/auth/login {email,password} -> {token}
- GET /v1/me (Bearer)
- POST /v1/qr/session {class_id} (roles: class_rep, rep, admin) -> {id / session}
- GET /v1/qr/session/{id}/code (rep roles) -> {code, expires_at/exp}
- POST /v1/attendance/scan {session_id?, code, lat, lng} (role: student) — check server.go handleScan for exact field names before coding
- POST /v1/qr/session/{id}/close (rep roles)
- GET /v1/attendance?session_id= (all authed; implementer must read handleAttendanceList for query params)
- POST /v1/absences (multipart: session_id, reason_type, file) (role: student)
- GET /v1/absences (roles: teacher, admin, student)
- PATCH /v1/absences/{id}/review {decision: approve|reject} (roles: teacher, admin)
- POST /v1/admin/classes (role: admin)
- GET /v1/classes
- GET /v1/analytics/class?class_id=&session_id= (roles: teacher, admin)
- GET /v1/analytics/college (role: admin)
- GET /v1/notifications + PATCH /v1/notifications/{id}/read
- GET /v1/audit-logs (role: admin)
- Error mapping: invalid code -> 401, out-of-fence -> 403+distance_m, rate-limited -> 429. Show these distinctly in UI.
- JWT: `Authorization: Bearer <token>`. Store token in localStorage.

## Seed accounts (password Atlas123!)
- admin@atlas.local / teacher@atlas.local / rep@atlas.local / s1@atlas.local / s2@atlas.local
- Class CS-101 lat 3.1390 lng 101.6869 radius 30m.

## Current state
- BASE: 03364ab. `web/.gitkeep` exists (empty). Replace it with the 8 files (delete .gitkeep in same commit or keep — either is fine, prefer delete).
- Stack is UP: nginx on localhost:8080 proxies /v1/ to api, serves web/ static. Migration applied, users=5.
- nginx serves `/` static with `try_files $uri =404` — index.html at web/index.html will be served at http://localhost:8080/.

## Constraints
- Minimal static, no build step, no framework. Vanilla HTML+JS + canvas QR via CDN lib (spec mandates QR lib CDN; pick one stable CDN, e.g. qrcodejs or qrjs, with offline-degraded message if CDN blocked).
- Rep page polls code endpoint every 5s (matches TOTP period). Show countdown to expiry.
- Student page: code input + geolocation via navigator.geolocation, GPS lock badge showing accuracy; absence upload via multipart form (5MB, pdf/jpg/png only — surface 400/413 errors).
- Teacher page: absence queue with doc iframe (proof_file_url under /uploads/), approve/reject buttons, rate cards from analytics/class.
- Admin page: class create form + audit table.
- index.html: login form + role router (redirect per role claim from /v1/me or JWT payload).
- manifest.json + sw.js: cache app shell (the 5 html + app.js + manifest).
- Keep each page small and consistent; shared fetch helper in app.js (token attach, 401 -> back to login).

## GUARDS
- Repo root C:\Users\amirn\Desktop\attendance. NEVER git init. NEVER touch outside root.
- Work from C:\Users\amirn\Desktop\attendance.
- Commit scoped: `git add web/` only. NEVER `git add -A`. Leave `.superpowers/`, `android-mcp.log` untracked.

## Report contract
Write full report to C:\Users\amirn\Desktop\attendance\.superpowers\sdd\2026-09-08-atlas\task-10-report.md (what built per file, click-through evidence per role incl. HTTP statuses, files changed, self-review). Reply ONLY: Status, commits, one-line test summary, concerns, report path.
