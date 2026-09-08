package httpapi

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"atlas/internal/attend"
	"atlas/internal/auth"
	"atlas/internal/totp"

	"github.com/DATA-DOG/go-sqlmock"
)

const testSecret = "test-secret-0123456789abcdef-test-secret"

type memStore struct{}

func (memStore) Save(filename string, data []byte) (string, error) {
	return "http://x/" + filename, nil
}

func newTestServer(db *sql.DB, lim attend.Limiter) *Server {
	return NewServer(db, lim, testSecret, memStore{})
}

func studentToken(t *testing.T, uid string) string {
	t.Helper()
	tok, err := auth.Token(uid, "student", testSecret)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func TestLoginSuccessAndBadPassword(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	hash, err := auth.Hash("Atlas123!")
	if err != nil {
		t.Fatal(err)
	}

	mock.ExpectQuery("SELECT id, password_hash, role FROM users").
		WithArgs("s1@atlas.local").
		WillReturnRows(sqlmock.NewRows([]string{"id", "password_hash", "role"}).AddRow(7, hash, "student"))
	mock.ExpectQuery("SELECT id, password_hash, role FROM users").
		WithArgs("s1@atlas.local").
		WillReturnRows(sqlmock.NewRows([]string{"id", "password_hash", "role"}).AddRow(7, hash, "student"))

	srv := newTestServer(db, attend.NewMemLimiter(10, time.Minute))

	req := httptest.NewRequest("POST", "/v1/auth/login", strings.NewReader(`{"email":"s1@atlas.local","password":"Atlas123!"}`))
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("login: expected 200, got %d body %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	uid, role, err := auth.Parse(body.Token, testSecret)
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}
	if uid != "7" || role != "student" {
		t.Fatalf("unexpected token claims uid=%q role=%q", uid, role)
	}

	bad := httptest.NewRequest("POST", "/v1/auth/login", strings.NewReader(`{"email":"s1@atlas.local","password":"wrong"}`))
	badRec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(badRec, bad)
	if badRec.Code != http.StatusUnauthorized {
		t.Fatalf("bad password: expected 401, got %d body %s", badRec.Code, badRec.Body.String())
	}
	if !strings.Contains(badRec.Body.String(), "INVALID_CREDENTIALS") {
		t.Fatalf("expected INVALID_CREDENTIALS, got %s", badRec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func scanSessionRows(secret []byte) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"totp_secret", "geofence_lat", "geofence_lng", "geofence_radius_m", "session_end"}).
		AddRow(secret, 3.1390, 101.6869, 30, nil)
}

func TestScanBadCodeMaps401(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	secret := []byte(strings.Repeat("A", 32))
	now := time.Now()
	mock.ExpectQuery("FROM qr_sessions").WithArgs(1).WillReturnRows(scanSessionRows(secret))

	bad := "000000"
	if totp.Generate(secret, now) == bad {
		bad = "111111"
	}

	srv := newTestServer(db, attend.NewMemLimiter(10, time.Minute))
	req := httptest.NewRequest("POST", "/v1/attendance/scan",
		strings.NewReader(`{"session_id":1,"code":"`+bad+`","lat":3.1390,"lng":101.6869}`))
	req.Header.Set("Authorization", "Bearer "+studentToken(t, "7"))
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "INVALID_CODE") {
		t.Fatalf("expected INVALID_CODE, got %s", rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestScanFarGPSMaps403WithDistance(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	secret := []byte(strings.Repeat("B", 32))
	mock.ExpectQuery("FROM qr_sessions").WithArgs(1).WillReturnRows(scanSessionRows(secret))

	srv := newTestServer(db, attend.NewMemLimiter(10, time.Minute))
	// Code must be valid at handler time; use a wide-lat valid code is time
	// dependent, so generate it right before the request.
	code := totp.Generate(secret, time.Now())
	req := httptest.NewRequest("POST", "/v1/attendance/scan",
		strings.NewReader(`{"session_id":1,"code":"`+code+`","lat":3.1480,"lng":101.6869}`))
	req.Header.Set("Authorization", "Bearer "+studentToken(t, "7"))
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "OUT_OF_GEOFENCE") {
		t.Fatalf("expected OUT_OF_GEOFENCE, got %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "distance_m") {
		t.Fatalf("expected distance_m, got %s", rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestScanRateLimitedMaps429(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	lim := attend.NewMemLimiter(1, time.Minute)
	if ok, _ := lim.Allow("scan:192.0.2.1"); !ok {
		t.Fatal("setup Allow should succeed")
	}

	srv := newTestServer(db, lim)
	req := httptest.NewRequest("POST", "/v1/attendance/scan",
		strings.NewReader(`{"session_id":1,"code":"000000","lat":3.1390,"lng":101.6869}`))
	req.Header.Set("Authorization", "Bearer "+studentToken(t, "7"))
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d body %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "RATE_LIMITED") {
		t.Fatalf("expected RATE_LIMITED, got %s", rec.Body.String())
	}
}

func TestRoleGateAndAuth(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	srv := newTestServer(db, attend.NewMemLimiter(10, time.Minute))
	h := srv.Routes()

	forbidden := httptest.NewRequest("POST", "/v1/admin/classes",
		strings.NewReader(`{"class_name":"X","geofence_lat":1,"geofence_lng":2,"geofence_radius_m":30}`))
	forbidden.Header.Set("Authorization", "Bearer "+studentToken(t, "7"))
	frec := httptest.NewRecorder()
	h.ServeHTTP(frec, forbidden)
	if frec.Code != http.StatusForbidden {
		t.Fatalf("student admin route: expected 403, got %d body %s", frec.Code, frec.Body.String())
	}
	if !strings.Contains(frec.Body.String(), "FORBIDDEN") {
		t.Fatalf("expected FORBIDDEN, got %s", frec.Body.String())
	}

	anon := httptest.NewRequest("POST", "/v1/admin/classes", strings.NewReader(`{}`))
	arec := httptest.NewRecorder()
	h.ServeHTTP(arec, anon)
	if arec.Code != http.StatusUnauthorized {
		t.Fatalf("no token: expected 401, got %d body %s", arec.Code, arec.Body.String())
	}
}

func TestAbsenceListGate(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	srv := newTestServer(db, attend.NewMemLimiter(10, time.Minute))
	h := srv.Routes()

	mkToken := func(role string) string {
		tok, err := auth.Token("9", role, testSecret)
		if err != nil {
			t.Fatal(err)
		}
		return tok
	}

	rep := httptest.NewRequest("GET", "/v1/absences", nil)
	rep.Header.Set("Authorization", "Bearer "+mkToken("class_rep"))
	repRec := httptest.NewRecorder()
	h.ServeHTTP(repRec, rep)
	if repRec.Code != http.StatusForbidden {
		t.Fatalf("class_rep absence list: expected 403, got %d body %s", repRec.Code, repRec.Body.String())
	}
	if !strings.Contains(repRec.Body.String(), "FORBIDDEN") {
		t.Fatalf("expected FORBIDDEN, got %s", repRec.Body.String())
	}
}

func TestMeReturnsRow(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery("FROM users").WithArgs(7).
		WillReturnRows(sqlmock.NewRows([]string{"id", "full_name", "email", "role", "class_id"}).
			AddRow(7, "Student One", "s1@atlas.local", "student", 3))

	srv := newTestServer(db, attend.NewMemLimiter(10, time.Minute))
	req := httptest.NewRequest("GET", "/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+studentToken(t, "7"))
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Email != "s1@atlas.local" || body.Role != "student" {
		t.Fatalf("unexpected me body %s", rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestTeacherCanOpenQRSessionGate(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	srv := newTestServer(db, attend.NewMemLimiter(10, time.Minute))
	h := srv.Routes()

	mkToken := func(role string) string {
		tok, err := auth.Token("11", role, testSecret)
		if err != nil {
			t.Fatal(err)
		}
		return tok
	}

	tr := httptest.NewRequest("POST", "/v1/qr/session", strings.NewReader(`{}`))
	tr.Header.Set("Authorization", "Bearer "+mkToken("teacher"))
	trRec := httptest.NewRecorder()
	h.ServeHTTP(trRec, tr)
	if trRec.Code == http.StatusForbidden {
		t.Fatalf("teacher open session: expected pass-through (400/200), got 403 %s", trRec.Body.String())
	}

	sr := httptest.NewRequest("POST", "/v1/qr/session", strings.NewReader(`{}`))
	sr.Header.Set("Authorization", "Bearer "+mkToken("student"))
	srRec := httptest.NewRecorder()
	h.ServeHTTP(srRec, sr)
	if srRec.Code != http.StatusForbidden {
		t.Fatalf("student open session: expected 403, got %d %s", srRec.Code, srRec.Body.String())
	}
}
