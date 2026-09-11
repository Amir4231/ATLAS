# Task 2 Report — TOTP + Haversine domain libs (TDD)

## What was implemented
- `api/internal/totp/totp.go`: RFC6238 SHA1 TOTP, 6-digit, `Period = 5`, ±1 step window via `Generate(secret []byte, t time.Time) string` and `Valid(secret []byte, code string, t time.Time) bool`. Byte-exact to brief.
- `api/internal/totp/totp_test.go`: `TestGenerateSixDigits`, `TestValidAcceptsWindow`. Byte-exact to brief.
- `api/internal/geo/geo.go`: `HaversineM(lat1, lng1, lat2, lng2 float64) float64` in meters (R=6371000). Byte-exact to brief.
- `api/internal/geo/geo_test.go`: `TestHaversineKnownDistance` (~15m KL check + same-point-zero). Byte-exact to brief.
- Stdlib only; `go.mod` unchanged (`module atlas`, `go 1.23`, no deps).

## TDD evidence

### TOTP RED — `go test ./internal/totp/ -v` (from `api/`, before `totp.go` existed)
```
# atlas/internal/totp [atlas/internal/totp.test]
internal\totp\totp_test.go:9:7: undefined: Generate
internal\totp\totp_test.go:18:10: undefined: Generate
internal\totp\totp_test.go:19:6: undefined: Valid
internal\totp\totp_test.go:22:6: undefined: Valid
internal\totp\totp_test.go:22:15: undefined: Generate
internal\totp\totp_test.go:25:5: undefined: Valid
internal\totp\totp_test.go:25:32: undefined: Generate
FAIL  atlas/internal/totp [build failed]
FAIL
```
Why expected: test file existed, production file did not — build failure on `undefined: Generate` matches brief expectation. Fails because feature missing, not a typo.

### TOTP GREEN — `go test ./internal/totp/ -v` (after `totp.go`)
```
=== RUN   TestGenerateSixDigits
--- PASS: TestGenerateSixDigits (0.00s)
=== RUN   TestValidAcceptsWindow
--- PASS: TestValidAcceptsWindow (0.00s)
PASS
ok    atlas/internal/totp  0.353s
```

### GEO RED — `go test ./internal/geo/ -v` (before `geo.go` existed)
```
# atlas/internal/geo [atlas/internal/geo.test]
internal\geo\geo_test.go:6:7: undefined: HaversineM
internal\geo\geo_test.go:10:5: undefined: HaversineM
FAIL  atlas/internal/geo [build failed]
FAIL
```
Why expected: same TDD pattern — test references `HaversineM` before implementation exists.

### GEO + FULL GREEN — `go test ./internal/... -v` (after both impls)
```
=== RUN   TestHaversineKnownDistance
--- PASS: TestHaversineKnownDistance (0.00s)
PASS
ok    atlas/internal/geo  0.422s
=== RUN   TestGenerateSixDigits
--- PASS: TestGenerateSixDigits (0.00s)
=== RUN   TestValidAcceptsWindow
--- PASS: TestValidAcceptsWindow (0.00s)
PASS
ok    atlas/internal/totp (cached)
```
- `gofmt -l .` → clean (no output).
- `go vet ./internal/...` → clean (no output).
- `go.mod` confirmed dependency-free.

## Files changed, commits
- Added: `api/internal/totp/totp.go`, `api/internal/totp/totp_test.go`, `api/internal/geo/geo.go`, `api/internal/geo/geo_test.go` (4 files, +84).
- Commit: `9f6f3d1 feat(atlas): totp 5s engine and haversine gate` (scoped `git add Desktop/attendance/api/internal/totp Desktop/attendance/api/internal/geo` from repo root `C:\Users\amirn`; `git show --stat HEAD` confirms only those 4 files).
- Pre-existing untracked `Desktop/attendance/.superpowers/` and `Desktop/attendance/ATLAS_PRD.md` were NOT staged/committed.

## Self-review findings, concerns
- Signatures match brief exactly (`totp.Generate`, `totp.Valid`, `totp.Period == 5`, `geo.HaversineM`); files are `gofmt`-clean and stdlib-only.
- Minor edge note (no action taken, code kept byte-exact to brief): `Valid` at Unix epoch (counter 0) computes `hotp(secret, c-1)` with `c-1` wrapping to `MaxUint64`. Harmless — just an HMAC comparison that won't match — so fail-closed, no panic. Flagging for Task 5 awareness only.
- Benign: git emitted LF→CRLF warnings on add (Windows autocrlf); content unaffected.
- Concern: none blocking.
