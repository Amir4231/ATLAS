package absent

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestSubmitBadReason(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err := Submit(db, 7, 1, "vacation", "http://x/f.pdf"); !errors.Is(err, ErrBadReason) {
		t.Fatalf("expected ErrBadReason, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expected NO queries, got %v", err)
	}
}

func TestSubmitHappy(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT classes.homeroom_teacher_id").WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"homeroom_teacher_id"}).AddRow(42))
	mock.ExpectQuery("INSERT INTO absence_requests").WithArgs(7, 1, "sick", "http://x/f.pdf").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(9))
	mock.ExpectExec("INSERT INTO notifications").WithArgs(42).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO audit_logs").WithArgs(7, 9).
		WillReturnResult(sqlmock.NewResult(1, 1))

	id, err := Submit(db, 7, 1, "sick", "http://x/f.pdf")
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if id != 9 {
		t.Fatalf("expected id 9, got %d", id)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestReviewApprove(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT student_id, session_id, status FROM absence_requests").WithArgs(9).
		WillReturnRows(sqlmock.NewRows([]string{"student_id", "session_id", "status"}).AddRow(7, 1, "pending"))
	mock.ExpectExec("UPDATE absence_requests SET status").WithArgs("approved", 9).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("UPDATE attendance_records SET status").WithArgs(7, 1).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO notifications").WithArgs(7).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO audit_logs").WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	if err := Review(db, 9, true); err != nil {
		t.Fatalf("review: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestReviewApproveInsertsWhenNoAttendanceRow(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT student_id, session_id, status FROM absence_requests").WithArgs(9).
		WillReturnRows(sqlmock.NewRows([]string{"student_id", "session_id", "status"}).AddRow(7, 1, "pending"))
	mock.ExpectExec("UPDATE absence_requests SET status").WithArgs("approved", 9).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("UPDATE attendance_records SET status").WithArgs(7, 1).
		WillReturnResult(sqlmock.NewResult(1, 0))
	mock.ExpectQuery("SELECT class_id FROM qr_sessions").WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"class_id"}).AddRow(3))
	mock.ExpectExec("INSERT INTO attendance_records").WithArgs(7, 3, 1).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO notifications").WithArgs(7).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO audit_logs").WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	if err := Review(db, 9, true); err != nil {
		t.Fatalf("review: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestReviewDecline(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT student_id, session_id, status FROM absence_requests").WithArgs(9).
		WillReturnRows(sqlmock.NewRows([]string{"student_id", "session_id", "status"}).AddRow(7, 1, "pending"))
	mock.ExpectExec("UPDATE absence_requests SET status").WithArgs("rejected", 9).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO notifications").WithArgs(7).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO audit_logs").WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	if err := Review(db, 9, false); err != nil {
		t.Fatalf("review: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestReviewNotPending(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT student_id, session_id, status FROM absence_requests").WithArgs(9).
		WillReturnRows(sqlmock.NewRows([]string{"student_id", "session_id", "status"}).AddRow(7, 1, "approved"))
	mock.ExpectRollback()

	if err := Review(db, 9, true); !errors.Is(err, ErrNotPending) {
		t.Fatalf("expected ErrNotPending, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestReviewMissingRow(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT student_id, session_id, status FROM absence_requests").WithArgs(99).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()

	if err := Review(db, 99, true); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected sql.ErrNoRows, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
