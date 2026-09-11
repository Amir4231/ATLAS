# Task 4 Report — Auth login + JWT middleware (TDD)

## Steps
1. Created `api/internal/auth/auth_test.go` verbatim from brief (4 tests).
2. RED: `go test ./internal/auth/ -v` → FAIL build failed, `undefined: Token` (plus Parse/Hash/Check/UID/Role) — expected failure confirmed.
3. Implemented `api/internal/auth/auth.go` (stdlib-only JWT + bcrypt cost 10, exact symbols Hash/Check/Token/Parse/Middleware/UID/Role).
4. GREEN: `go test ./internal/auth/ -v` → 4/4 PASS; `gofmt -l .` clean; `go vet ./internal/auth/` clean.
5. Committed scoped: `git add Desktop/attendance/api/internal/auth` → `0d68219 feat(atlas): jwt auth and middleware`.

## Test evidence
- RED output: `FAIL atlas/internal/auth [build failed]` — `internal\auth\auth_test.go:13:14: undefined: Token` (+ Parse/Hash/Check/UID/Role).
- GREEN output: `TestTokenRoundTrip PASS`, `TestParseRejectsTampered PASS`, `TestHashCheck PASS` (~0.18s bcrypt), `TestMiddleware PASS`; `ok atlas/internal/auth`.

## Verify
- `go test ./internal/auth/ -v` 4/4 PASS
- `gofmt -l .` no output (clean)
- `go vet ./internal/auth/` no output (clean)

## Commit
- `0d68219 feat(atlas): jwt auth and middleware` (2 files, +188): `api/internal/auth/auth.go`, `api/internal/auth/auth_test.go`

## Self-review
- Exact Task 8 symbols present: Hash/Check/Token/Parse/Middleware/UID/Role — no renames.
- Token format per spec: header `{"alg":"HS256","typ":"JWT"}`, payload `uid|role|expUnix`, HMAC-SHA256, RawURLEncoding, 8h expiry.
- Parse: 3-part check, constant-time sig compare, `ErrInvalid` on bad sig/structure, `ErrExpired` on expiry.
- Middleware: `Authorization: Bearer <tok>`, 401 `{"error":"UNAUTHORIZED"}` JSON on missing/malformed/Parse error, uid+role via unexported ctx key; UID/Role return "" when absent.
- Stdlib-only JWT; bcrypt reused from go.mod (`golang.org/x/crypto`).

## Concerns
- None. Note: 401 body written without trailing newline (exact JSON match); `http.Error` not used to preserve `application/json` content-type.
