# Task 4 Brief — Auth login + JWT middleware, TDD (from docs/superpowers/plans/2026-09-08-atlas.md)

## Global Constraints (bind this task)

- JWT HS256, 8h expiry. bcrypt cost 10. Stdlib JWT (no external JWT lib);
  `golang.org/x/crypto/bcrypt` already in go.mod — reuse it.
- Middleware: reads `Authorization: Bearer <tok>`, 401 JSON
  `{"error":"UNAUTHORIZED"}` on any failure, sets `uid` + `role` in request
  context for downstream handlers.
- Commits scoped to Desktop/attendance pathspecs only.

## Environment

- PowerShell 5.1, refresh PATH for `go` if needed.
- Work from `C:\Users\amirn\Desktop\attendance\api`.

## Task

**Files:**
- Create: `api/internal/auth/auth.go`
- Test: `api/internal/auth/auth_test.go`

**Interfaces (exact — Task 8 imports these):**
- `auth.Hash(pw string) (string, error)`
- `auth.Check(pw, hash string) bool`
- `auth.Token(uid, role, secret string) (string, error)`
- `auth.Parse(tok, secret string) (uid, role string, err error)`
- `auth.Middleware(secret string, next http.Handler) http.Handler`
- `auth.UID(r *http.Request) string`, `auth.Role(r *http.Request) string`
  (context accessors for handlers)

Token format: `base64url(header).base64url(payload).base64url(sig)` where
header `{"alg":"HS256","typ":"JWT"}`, payload `uid|role|expUnix`
pipe-joined, sig = HMAC-SHA256(secret, `headerB64.payloadB64`).
Expiry: now+8h; Parse rejects expired (`ErrExpired`) and bad sig
(`ErrInvalid`). Use `crypto/hmac`, `crypto/sha256`, `crypto/subtle`,
`encoding/base64` (RawURLEncoding), `strings`, `strconv`, `time`.

- [ ] **Step 1: Failing tests** (`api/internal/auth/auth_test.go`)

```go
package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

const testSecret = "test-secret-32-bytes-minimum-xyz"

func TestTokenRoundTrip(t *testing.T) {
	tok, err := Token("7", "student", testSecret)
	if err != nil {
		t.Fatal(err)
	}
	uid, role, err := Parse(tok, testSecret)
	if err != nil || uid != "7" || role != "student" {
		t.Fatalf("bad roundtrip %v %v %v", uid, role, err)
	}
}

func TestParseRejectsTampered(t *testing.T) {
	tok, _ := Token("7", "student", testSecret)
	if _, _, err := Parse(tok+"x", testSecret); err == nil {
		t.Fatal("tampered token must fail")
	}
	if _, _, err := Parse(tok, "wrong-secret-32-bytes-minimum-00"); err == nil {
		t.Fatal("wrong secret must fail")
	}
}

func TestHashCheck(t *testing.T) {
	h, err := Hash("Atlas123!")
	if err != nil {
		t.Fatal(err)
	}
	if !Check("Atlas123!", h) || Check("wrong", h) {
		t.Fatal("bcrypt check mismatch")
	}
}

func TestMiddleware(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if UID(r) != "7" || Role(r) != "rep" {
			t.Errorf("bad ctx %q %q", UID(r), Role(r))
		}
		w.WriteHeader(200)
	})
	tok, _ := Token("7", "rep", testSecret)
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	Middleware(testSecret, next).ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("want 200 got %d", rec.Code)
	}
	bad := httptest.NewRequest("GET", "/", nil)
	badRec := httptest.NewRecorder()
	Middleware(testSecret, next).ServeHTTP(badRec, bad)
	if badRec.Code != 401 {
		t.Fatalf("want 401 got %d", badRec.Code)
	}
	_ = time.Now
}
```

- [ ] **Step 2: Run, verify FAIL**

Run: `go test ./internal/auth/ -v` Expected: FAIL "undefined: Token".

- [ ] **Step 3: Minimal impl** (`api/internal/auth/auth.go`)

Per format above. Middleware: missing/malformed header or Parse error →
`http.Error(w, {"error":"UNAUTHORIZED"}, 401)` with Content-Type
application/json. Use unexported context key type. UID/Role return "" when
absent.

- [ ] **Step 4: Run PASS + commit**

Run: `go test ./internal/auth/ -v` Expected: PASS (4/4).
Run: `gofmt -l .` clean, `go vet ./internal/auth/` clean.

```bash
git add Desktop/attendance/api/internal/auth
git commit -m "feat(atlas): jwt auth and middleware"
```
