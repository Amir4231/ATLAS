# Task 10 Report: Minimal static PWA

BASE: 03364ab → commit 547d97e `feat(atlas): minimal role PWA` (9 files: 8 created, `web/.gitkeep` deleted).
Scope check: `git add web/` only; `.superpowers/`, `android-mcp.log` left untracked; nothing outside repo root touched.

## What was built (per file)
- `web/app.js` — shared helper: Bearer attach, `api()` returning `{status,data}`, `login/logout`, `roleHome()` (admin/teacher/class_rep|rep/student), `requireRole()` page guard (401+token → login), `scanError()` (401 invalid-code / 403 out-of-fence+distance / 429 rate-limited / 410 closed), `uploadError()` (413 too-large / 400 bad-type / bad-reason).
- `web/index.html` — login form + auto role-router via `GET /v1/me`.
- `web/rep.html` — class select, open/close session, code polled every 5s with expiry countdown, QR via cdnjs qrcodejs with offline-degraded text fallback, live check-in feed from `GET /v1/attendance?session_id=`.
- `web/student.html` — scan form (session_id, code, lat/lng + geolocation button with GPS-lock ±m badge), distinct scan-error display, own-status list, multipart absence form (sick/outreach/personal, pdf/jpg/png) with 400/413 surfacing.
- `web/teacher.html` — session rate cards from `GET /v1/analytics/class?session_id=`, pending queue with proof `<iframe src=proof_file_url>`, approve/reject → `PATCH .../review {decision: approved|rejected}`.
- `web/admin.html` — class create form, class list, college-rate card, audit table (`GET /v1/audit-logs?limit=50`).
- `web/manifest.json`, `web/sw.js` — app-shell cache (`atlas-v1`: 5 html + app.js + manifest); API/uploads/CDN left network-only.

## Contract corrections vs brief (controller-verified in `server.go`, UI matches server)
- `POST /v1/qr/session` returns `{session_id}` (not `{id}`); code endpoint returns `{code, exp}` where **exp is unix seconds** (not RFC3339) — first draft used `Date.parse(exp)` → NaN countdown; fixed to handle numeric/string before commit.
- Review decision is `approved|rejected` (brief said `approve|reject`); UI sends `approved`/`rejected`.
- `GET /v1/analytics/class` requires `session_id` (class_id ignored); reasons limited to sick/outreach/personal.

## Click-through evidence (all over HTTP via nginx localhost:8080)
- Static: all 8 files `200` (`/index.html`, `/rep.html`, `/student.html`, `/teacher.html`, `/admin.html`, `/app.js`, `/manifest.json`, `/sw.js`).
- Login: all 5 seeds → tokens; `/v1/classes` → CS-101 id 1 @ 3.1390,101.6869 r=30m.
- Rep (class_rep): open session → `{"session_id":1}`; code fetch → `{"code":"384229","exp":...}` (a later fetch gave `158301` — codes rotate ~30s; an early scan with a stale code returned 401, confirming rotation and why the 5s poll matters).
- Student s1: fresh-code scan @ class coords → `{"status":"present","distance_m":0}`; feed shows record `#1 student #4 present`.
- Errors: bad code → `401 {"error":"INVALID_CODE"}`; lat 0/lng 0 → `403 {"error":"OUT_OF_GEOFENCE","distance_m":11305090.18}`.
- Student s2: multipart absence upload → `{"id":1}`, `proof_file_url /uploads/9216c4de607d1ddb-tmp_proof.pdf`.
- Teacher: queue shows pending #1; rate → `present 1/1 100%`, categories sick:1; review approved → `{"status":"approved"}`; s2's list flips to `approved`.
- Admin: class CS-102 created → `{"id":2}`; college → `present 1/total 2/50%`; audit shows scan/submit/review entries. Session closed → `{"marked":1}`.

## Self-review
- No framework/build step; CDN is the only external dep and degrades to big-text code. Token in localStorage per spec. Temp verification files (tokens, mini-pdf) deleted before commit; worktree clean except required untracked items.

## Concerns for Task 11 (pre-existing stack issues, out of scope — commit is web/-only)
1. `GET /` → 404: nginx `try_files $uri =404` never resolves `/` to `index.html`. E2E must use `/index.html` explicitly or fix nginx try_files.
2. `GET /uploads/<proof>` → 403: API saves uploads as `0600 root:root`, nginx worker can't read them — teacher doc iframe will be blank until API chmods on save (api-side fix needed).
