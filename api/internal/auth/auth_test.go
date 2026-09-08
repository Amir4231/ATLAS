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
