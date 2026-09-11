# Task 11 Brief: End-to-end verification (final task)

## Plan source
docs/superpowers/plans/2026-09-08-atlas.md, Task 11 (verbatim):

- [ ] **Step 1: Unit + vet**
Run: `go test ./... -v` Expected: PASS. Run: `go vet ./...` + `gofmt -l .` Expected: clean.
- [ ] **Step 2: Full journey on Compose**
Seed login → rep session → student scan (valid, replay-rejected, far-rejected) → close → absence → teacher approve → analytics + notifications + audit visible. Screenshot-replay (reuse code after 5s) must 401.
- [ ] **Step 3: Final commit/tag** `feat(atlas): e2e verified MVP`.

## Current state (BASE 547d97e)
- Tasks 1-10 complete. Stack UP on localhost:8080 (postgres/redis healthy, api Up, nginx Up). Migration applied once, seed users=5 (password Atlas123!). PWA 8 files served (note: GET /index.html 200, GET / 404 pre-existing).
- NOTE: Task 10 testing already polluted the live DB (session #1, absence #1, class CS-102 id 2 exist). Your journey must use FRESH resources (new session, new absence) — do not assert counts tied to polluted state.

## Parked stack fixes — fix THEN verify, in this task (both load-bearing for e2e)
1. `GET /` → 404. nginx `try_files $uri =404` never resolves `/` to index.html. Fix: minimal nginx change (e.g. `try_files $uri $uri/ /index.html` or `index index.html` + try_files fallback) in nginx/nginx.conf, `docker compose up -d nginx`, verify `GET /` 200 serves login page. Keep /v1/ proxy + /uploads/ alias intact.
2. `GET /uploads/<proof>` → 403. API saves proofs `0600 root:root`, nginx worker can't read. Fix: minimal api-side change (chmod 0644 on save in api/internal/storage/storage.go or wherever SaveLocal writes — read the code first; do NOT change MIME gate or 5MB limit), rebuild api (`docker compose up -d --build api`), re-upload a fresh absence, verify `GET <proof_file_url>` 200 with PDF bytes. Keep audit/notification behavior intact.

## E2E journey (fresh resources, over HTTP via nginx localhost:8080)
1. go test ./... -v (from api/), go vet ./..., gofmt -l . — record outputs.
2. Login all 5 seeds → tokens. Rep opens NEW session (class 1). Fetch code, student s1 scans valid @ 3.1390,101.6869 → present + distance_m small. Replay SAME code after 5s+ (poll for fresh code rotation or sleep past expiry) → 401. Far scan (0,0) → 403 + distance_m large. Rate-limit spot check optional (>10/min → 429 — skip if it would block the journey; note if skipped).
3. Close session. Student s2 submits fresh absence (multipart pdf). Teacher lists, approves (decision `approved`). Verify s2 status flips to approved.
4. Analytics class (session_id) + college visible; notifications for student; audit-logs for admin visible.
5. PWA: GET / 200 (post-fix), /index.html + role pages 200, proof URL 200 (post-fix, teacher iframe unblocked).

## GUARDS
- Repo root C:\Users\amirn\Desktop\attendance. NEVER git init. NEVER touch outside root.
- Commits scoped to relative pathspecs (nginx/nginx.conf, api/internal/storage/..., web/ only if PWA fix needed). NEVER `git add -A`. Leave `.superpowers/`, `android-mcp.log` untracked.
- Migration single-apply — NEVER re-apply 001_init.sql.

## Report contract
Write full report to C:\Users\amirn\Desktop\attendance\.superpowers\sdd\2026-09-08-atlas\task-11-report.md (unit/vet outputs, each journey step with HTTP statuses + IDs, the two stack fixes with before/after curl evidence, files changed, self-review). Reply ONLY: Status, commits, one-line test summary, concerns, report path.
