# Task 11 Report: End-to-end verification (final task)

BASE: 547d97e → HEAD: d496967 (`feat(atlas): e2e verified MVP`, 2 files, +3/−2).
Stack: compose on localhost:8080 (postgres/redis healthy, api rebuilt, nginx recreated). Migration NOT re-applied (single-apply guard). Live DB had Task-10 pollution (session #1, absence #1, class CS-102 id 2); journey used FRESH resources only (session #3, absence #2).

## Step 1 — Unit + vet (from api/)
- `go test ./... -v`: ALL PASS (absent 7/7, attend 7/7, auth 4/4, geo 1/1, httpapi 7/7, report 7/7, storage 5/5, totp 2/2). Re-ran `go test ./...` after the storage.go change: all `ok` (storage + httpapi re-ran uncached).
- `go vet ./...`: clean (exit True = $?=True, no findings). `gofmt -l .`: empty (clean).

## Step 2 — Parked stack fixes (before/after curl evidence)
1. `GET /` → 404. Cause: `location /` had `try_files $uri =404` (never resolves `/` to index.html). Fix (nginx/nginx.conf): added `index index.html;` + `try_files $uri $uri/ /index.html;`. `/v1/` proxy and `/uploads/` alias untouched. `docker compose up -d --force-recreate nginx` (needed: config is a bind-mount, plain `up -d` did not recreate). Before: `GET / → 404`, `GET /index.html → 200`. After: `GET / → 200` (serves Atlas login page, `<title>Atlas … Login</title>`), `/index.html` still 200.
2. `GET /uploads/<proof>` → 403. Cause: `LocalStorage.Save` wrote `0o600 root:root`; nginx worker (different uid) could not read. Fix (api/internal/storage/storage.go:64): `0o600` → `0o644`. MIME gate + 5MB limit untouched. `docker compose up -d --build api`. Before: `-rw------- root root …9216c4de…pdf`, `GET /uploads/9216c4de…pdf → 403`. After: fresh upload `-rw-r--r-- root root …a602c14f…pdf`, `GET /uploads/a602c14f97ae8aa8-proof-tmp.pdf → 200, content-type application/pdf, 77 bytes` (PDF magic intact). Old 0600 file still 403 (pre-existing artifact, expected — fix is forward-looking by design).

## Step 3 — Full journey (fresh resources, via nginx localhost:8080)
- Login 5 seeds (admin/teacher/rep/s1/s2, Atlas123!): 200 ×5, tokens issued.
- Rep opened NEW session id=3 (class 1 CS-101, fence 3.1390,101.6869 r=30m): 200 `{"session_id":3}`.
- Code fetched (629056). s1 valid scan @ fence center → **200 `{"status":"present","distance_m":0}`**.
- Screenshot-replay: waited 13s until code rotated twice (old 629056 → new 954746; TOTP period 5s ±1-step window requires ≥2 rotations), reused old code → **401 `{"error":"INVALID_CODE"}`**.
- Far scan (0,0) with fresh code → **403 `{"error":"OUT_OF_GEOFENCE","distance_m":11305090.18}`**.
- Close session → 200 `{"marked":2}` (rep + s2 → absent_unexcused; s1 already present). Attendance list shows s1 present w/ scan lat/lng + recorded_at.
- Rate-limit spot check SKIPPED per brief (would risk blocking journey; budget 10/min/IP, journey used 4 scans).
- s2 submitted FRESH absence id=2 (multipart pdf, reason sick, session 3) → 200. Teacher pending-list shows it pending → PATCH review `approved` → 200 `{"status":"approved"}` → approved-list shows id=2 approved (status flip verified; pre-existing id=1 also listed, ignored).
- Analytics class (teacher, session 3) → 200 `present:1/total:3, categories:[sick:1]`. Analytics college (admin) → 200 `present:2/total:9`. Notifications (s2) → 200, includes `absence.reviewed` for id=2. Audit-logs (admin) → 200, 6 rows incl. `attendance.scan→9`, `absence.submit→2`, `absence.review→2`.
- PWA: `/`, `/index.html`, `/rep.html`, `/student.html`, `/teacher.html`, `/admin.html`, `/app.js`, `/sw.js`, `/manifest.json` → all 200. Proof URL 200 (teacher iframe unblocked).

## Files changed (scoped commits, no -A; .superpowers/ + android-mcp.log left untracked)
- d496967 `feat(atlas): e2e verified MVP`: nginx/nginx.conf, api/internal/storage/storage.go.

## Self-review
- Guards honored: no git init, nothing outside repo root (journey PS scripts + temp pdf lived in repo root, deleted post-run; verified via `git status`), no `git add -A` (two explicit pathspecs), migration untouched.
- Fixes minimal: 1-line Go perm change; 2-line nginx change preserving proxy/alias. MIME gate, 5MB limit, audit/notification behavior intact (all exercised in journey).
- False start noted: first scan attempt used curl.exe with pretty-printed JSON → server 400s (client artifact, confirmed via Invoke-RestMethod reproducing correct 410 on closed session). Re-ran journey cleanly on session #3; session #2 (polluted by the 400-run + close) excluded from assertions.
- Concern (minor): pre-existing `/uploads/9216c4de…pdf` (0600, Task-10 era) still 403s. Optional follow-up: one-off `chmod 644` on the volume or backfill; not required for MVP.
