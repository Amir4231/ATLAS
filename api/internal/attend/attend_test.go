package attend

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"atlas/internal/totp"

	"github.com/DATA-DOG/go-sqlmock"
)

func fixedSecret() []byte {
	return bytes.Repeat([]byte{0xAB}, 32)
}

const sessionQuery = "SELECT totp_secret"

func sessionRows(secret []byte) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"totp_secret", "geofence_lat", "geofence_lng", "geofence_radius_m", "session_end"}).
		AddRow(secret, 3.1390, 101.6869, 30, nil)
}

func TestScanRejectsBadCode(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	secret := fixedSecret()
	now := time.Now()
	mock.ExpectQuery(sessionQuery).WithArgs(1).
		WillReturnRows(sessionRows(secret))

	bad := "000000"
	if totp.Generate(secret, now) == bad {
		bad = "111111"
	}

	_, _, err = Scan(db, NewMemLimiter(10, time.Minute), "1.2.3.4", 1, 7, bad, 3.1390, 101.6869, now)
	if !errors.Is(err, ErrInvalidCode) {
		t.Fatalf("expected ErrInvalidCode, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestScanRejectsFarGPS(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	secret := fixedSecret()
	now := time.Now()
	mock.ExpectQuery(sessionQuery).WithArgs(1).
		WillReturnRows(sessionRows(secret))

	code := totp.Generate(secret, now)

	_, dist, err := Scan(db, NewMemLimiter(10, time.Minute), "1.2.3.4", 1, 7, code, 3.1390+0.009, 101.6869, now)
	if !errors.Is(err, ErrOutOfFence) {
		t.Fatalf("expected ErrOutOfFence, got %v", err)
	}
	if dist <= 900 {
		t.Fatalf("expected distM > 900, got %v", dist)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestScanRateLimited(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	lim := NewMemLimiter(1, time.Minute)
	if ok, _ := lim.Allow("scan:9.9.9.9"); !ok {
		t.Fatal("setup Allow should succeed")
	}

	_, _, err = Scan(db, lim, "9.9.9.9", 1, 7, "000000", 3.1390, 101.6869, time.Now())
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("expected ErrRateLimited, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expected NO db queries, got %v", err)
	}
}

func TestScanHappyPathIdempotent(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	secret := fixedSecret()
	now := time.Now()
	code := totp.Generate(secret, now)
	lim := NewMemLimiter(10, time.Minute)

	mock.ExpectQuery(sessionQuery).WithArgs(1).WillReturnRows(sessionRows(secret))
	mock.ExpectQuery("INSERT INTO attendance_records").WithArgs(7, 1, 3.1390, 101.6869).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(5))
	mock.ExpectExec("INSERT INTO audit_logs").WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO notifications").WithArgs(sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	status, _, err := Scan(db, lim, "1.2.3.4", 1, 7, code, 3.1390, 101.6869, now)
	if err != nil {
		t.Fatalf("first scan: %v", err)
	}
	if status != "present" {
		t.Fatalf("expected present, got %q", status)
	}

	mock.ExpectQuery(sessionQuery).WithArgs(1).WillReturnRows(sessionRows(secret))
	mock.ExpectQuery("INSERT INTO attendance_records").WithArgs(7, 1, 3.1390, 101.6869).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	status, _, err = Scan(db, lim, "1.2.3.4", 1, 7, code, 3.1390, 101.6869, now)
	if err != nil {
		t.Fatalf("second scan: %v", err)
	}
	if status != "present" {
		t.Fatalf("expected present, got %q", status)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMemLimiter(t *testing.T) {
	lim := NewMemLimiter(2, time.Minute)
	if ok, _ := lim.Allow("k"); !ok {
		t.Fatal("first Allow should pass")
	}
	if ok, _ := lim.Allow("k"); !ok {
		t.Fatal("second Allow should pass")
	}
	if ok, _ := lim.Allow("k"); ok {
		t.Fatal("third Allow should fail")
	}
}

func TestCodeBoundary(t *testing.T) {
	_, exp := Code(fixedSecret(), time.Unix(7, 0))
	if exp != 10 {
		t.Fatalf("expected exp 10, got %d", exp)
	}
}
