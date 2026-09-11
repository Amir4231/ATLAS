# SDD ledger — plan: docs/superpowers/plans/2026-09-08-atlas.md
# Workspace deviation: skill scripts target repo root (C:\Users\amirn, the home
# dir, not writable via tools), so workspace lives at
# Desktop/attendance/.superpowers/sdd/2026-09-08-atlas/.
# Worktree deviation: repo root is the home directory; a linked worktree would
# duplicate the entire home checkout. Working in place; all commits scoped to
# Desktop/attendance pathspecs only.
Task 1: complete (commits 6f88a67..dd205a2, review clean)
Resolved reviewer ⚠️: go1.27.0 + docker 29.7.2 verified after PATH refresh; commit scopes to 3 attendance files only.
Task 2: complete (commits dd205a2..9f6f3d1, review clean)
Controller verified: go test 3/3 PASS, gofmt/vet clean.
Task 3: complete (commits 9f6f3d1..7989eb1, review Approved)
Controller verified: go build OK, gofmt/vet clean.
Parked for Task 9: migration NOT re-runnable (no IF NOT EXISTS) — Task 9 must single-apply; go.mod toolchain bumped to 1.26.0 by tidy — Task 9 Dockerfile must use golang >=1.26.
Task 4: complete (commits 7989eb1..0d68219, review Approved)
Controller verified: auth 4/4 PASS, gofmt/vet clean.
Task 5: complete (commits 0d68219..6dd5f97, review Approved)
Controller verified: attend 6/6 PASS, gofmt/vet/build clean.
Deferred minor: tighten attend test WithArgs/SQL regexes (follow-up, non-blocking).
Task 6: complete (091e019 absence pipeline, review Approved)
Controller verified: go test ./... ALL packages PASS, gofmt/vet clean.
INCIDENT 2026-09-08: Task-6 implementer ran `git init` inside Desktop/attendance AND the home repo C:\Users\amirn\.git disappeared during its run (prior commits dd205a2..6dd5f97 design/plan/tasks1-5 unrecoverable from git). Worktree files intact. Recovered by rooting repo at Desktop/attendance: a1d78ec baseline (tasks1-5+docs+PRD+migrations), 091e019 task6. GUARD for all future briefs: repo root IS C:\Users\amirn\Desktop\attendance; NEVER git init; NEVER touch anything outside it; commits scoped to relative pathspecs.
Task 7: complete (commits 091e019..a12097c, review Approved)
Controller verified: report 7/7 PASS, gofmt/vet clean; all queried columns exist in 001_init.sql.
Accepted deviation: report.CollegeRate returns ClassRate (brief's type+func collision uncompilable) — Task 8 brief must use ClassRate for college rate.
Task 8: complete (commits a12097c..06a0b31, review Approved after fix round 1/5: absence-list gate ADDRESSED, no breakage)
Controller: reviewer ⚠️ covered by Tasks 4-7 reviews (signatures/middleware/CheckUpload verified there).
Task 9: fix round 1/5 (2 addressed, 0 open - nginx/api restart policy, fresh-boot order note; commits 350b2d3..03364ab)
Task 9: complete (commits 06a0b31..03364ab, review Approved after fix round 1/5)
Task 9: minor (deferred): mem_limit legacy syntax, nginx try_files/index check belongs to Task 10, volume names pgdata/redisdata grandfathered
Task 10: complete (commits 03364ab..547d97e, review Approved)
Task 10: minor (deferred): app.js 401-redirect, student loadMine filter, sw.js root entry, innerHTML esc across 4 pages
Task 10: parked for Task 11 - GET / 404 (nginx try_files, controller-verified /index.html 200) + /uploads 403 (api 0600 root) - stack fixes, not web/ defects
Task 11: complete (commits 547d97e..d496967, review Approved in-diff + controller live checks green)
Task 11: controller spot-checks: go test ALL PASS, vet/fmt clean; GET / 200; fresh proof 200 application/pdf; rep login token ok
Task 11: minor (deferred): old 0600 proof still 403 (forward-fix by design); rate-limit check skipped per brief
