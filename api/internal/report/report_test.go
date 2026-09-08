package report

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestSessionRate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery(`FROM attendance_records WHERE session_id`).
		WithArgs(9).
		WillReturnRows(sqlmock.NewRows([]string{"total", "present"}).AddRow(4, 3))

	got, err := SessionRate(db, 9)
	if err != nil {
		t.Fatal(err)
	}
	if got.Total != 4 || got.Present != 3 || got.Pct != 75 {
		t.Fatalf("got %+v, want total=4 present=3 pct=75", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSessionRateEmpty(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery(`FROM attendance_records WHERE session_id`).
		WithArgs(10).
		WillReturnRows(sqlmock.NewRows([]string{"total", "present"}).AddRow(0, 0))

	got, err := SessionRate(db, 10)
	if err != nil {
		t.Fatal(err)
	}
	if got.Total != 0 || got.Present != 0 || got.Pct != 0 {
		t.Fatalf("got %+v, want zeros", got)
	}
	if got.Pct != got.Pct {
		t.Fatal("Pct is NaN")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCollegeRate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery(`FROM attendance_records`).
		WillReturnRows(sqlmock.NewRows([]string{"total", "present"}).AddRow(10, 7))

	got, err := CollegeRate(db)
	if err != nil {
		t.Fatal(err)
	}
	if got.Total != 10 || got.Present != 7 || got.Pct != 70 {
		t.Fatalf("got %+v, want total=10 present=7 pct=70", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestChronicThreshold(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery(`FROM attendance_records`).
		WillReturnRows(sqlmock.NewRows([]string{"student_id", "class_id", "total", "unexcused"}).
			AddRow(1, 2, 10, 2).
			AddRow(3, 4, 10, 1))

	rows, err := Chronic(db, 15)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1: %+v", len(rows), rows)
	}
	if rows[0].StudentID != 1 || rows[0].ClassID != 2 || rows[0].Unexcused != 2 || rows[0].Total != 10 || rows[0].Pct != 20 {
		t.Fatalf("got %+v, want student=1 class=2 unexcused=2 total=10 pct=20", rows[0])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCategoryBreakdown(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery(`FROM absence_requests WHERE session_id`).
		WithArgs(5).
		WillReturnRows(sqlmock.NewRows([]string{"reason_type", "count"}).
			AddRow("sick", 2).
			AddRow("family", 1))

	rows, err := CategoryBreakdown(db, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].Reason != "sick" || rows[0].Count != 2 || rows[1].Reason != "family" || rows[1].Count != 1 {
		t.Fatalf("got %+v", rows)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestNotifications(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	now := time.Now()
	mock.ExpectQuery(`FROM notifications`).
		WithArgs(7).
		WillReturnRows(sqlmock.NewRows([]string{"id", "kind", "title", "body", "read_at", "created_at"}).
			AddRow(2, "info", "T2", "B2", nil, now).
			AddRow(1, "info", "T1", "B1", nil, now.Add(-time.Hour)))

	got, err := ListNotifications(db, 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != 2 || got[1].ID != 1 {
		t.Fatalf("got %+v, want newest-first ids [2 1]", got)
	}

	mock.ExpectExec(`UPDATE notifications SET read_at`).
		WithArgs(2, 7).
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := MarkRead(db, 2, 7); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestListAudit(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	now := time.Now()
	mock.ExpectQuery(`FROM audit_logs`).
		WithArgs(200).
		WillReturnRows(sqlmock.NewRows([]string{"id", "actor_id", "action", "entity", "entity_id", "created_at"}).
			AddRow(1, 3, "scan", "attendance", "5", now))

	rows, err := ListAudit(db, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].ID != 1 || rows[0].ActorID == nil || *rows[0].ActorID != 3 {
		t.Fatalf("got %+v", rows)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
