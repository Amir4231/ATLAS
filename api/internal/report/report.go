package report

import (
	"database/sql"
	"time"
)

type ClassRate struct {
	Present int
	Total   int
	Pct     float64
}

func SessionRate(db *sql.DB, sessionID int) (ClassRate, error) {
	var total, present int
	err := db.QueryRow(`SELECT COUNT(*) total, COUNT(*) FILTER (WHERE status='present') present FROM attendance_records WHERE session_id=$1`, sessionID).Scan(&total, &present)
	if err != nil {
		return ClassRate{}, err
	}
	var pct float64
	if total > 0 {
		pct = 100 * float64(present) / float64(total)
	}
	return ClassRate{Present: present, Total: total, Pct: pct}, nil
}

func CollegeRate(db *sql.DB) (ClassRate, error) {
	var total, present int
	err := db.QueryRow(`SELECT COUNT(*) total, COUNT(*) FILTER (WHERE status='present') present FROM attendance_records`).Scan(&total, &present)
	if err != nil {
		return ClassRate{}, err
	}
	var pct float64
	if total > 0 {
		pct = 100 * float64(present) / float64(total)
	}
	return ClassRate{Present: present, Total: total, Pct: pct}, nil
}

type ChronicRow struct {
	StudentID int
	ClassID   int
	Unexcused int
	Total     int
	Pct       float64
}

func Chronic(db *sql.DB, thresholdPct float64) ([]ChronicRow, error) {
	rows, err := db.Query(`SELECT student_id, class_id, COUNT(*) total, COUNT(*) FILTER (WHERE status='absent_unexcused') unexcused FROM attendance_records GROUP BY student_id, class_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ChronicRow
	for rows.Next() {
		var r ChronicRow
		if err := rows.Scan(&r.StudentID, &r.ClassID, &r.Total, &r.Unexcused); err != nil {
			return nil, err
		}
		if r.Total > 0 {
			r.Pct = 100 * float64(r.Unexcused) / float64(r.Total)
		}
		if r.Pct > thresholdPct {
			out = append(out, r)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

type CategoryRow struct {
	Reason string
	Count  int
}

func CategoryBreakdown(db *sql.DB, sessionID int) ([]CategoryRow, error) {
	rows, err := db.Query(`SELECT reason_type, COUNT(*) FROM absence_requests WHERE session_id=$1 GROUP BY reason_type`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CategoryRow
	for rows.Next() {
		var r CategoryRow
		if err := rows.Scan(&r.Reason, &r.Count); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

type Notification struct {
	ID      int
	Kind    string
	Title   string
	Body    string
	Read    *time.Time
	Created time.Time
}

func ListNotifications(db *sql.DB, userID int) ([]Notification, error) {
	rows, err := db.Query(`SELECT id, kind, title, body, read_at, created_at FROM notifications WHERE user_id=$1 ORDER BY created_at DESC, id DESC LIMIT 50`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Notification
	for rows.Next() {
		var n Notification
		var read sql.NullTime
		if err := rows.Scan(&n.ID, &n.Kind, &n.Title, &n.Body, &read, &n.Created); err != nil {
			return nil, err
		}
		if read.Valid {
			t := read.Time
			n.Read = &t
		}
		out = append(out, n)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func MarkRead(db *sql.DB, id, userID int) error {
	_, err := db.Exec(`UPDATE notifications SET read_at=now() WHERE id=$1 AND user_id=$2`, id, userID)
	return err
}

type AuditRow struct {
	ID       int
	ActorID  *int
	Action   string
	Entity   string
	EntityID string
	Created  time.Time
}

func ListAudit(db *sql.DB, limit int) ([]AuditRow, error) {
	if limit < 1 {
		limit = 1
	}
	if limit > 200 {
		limit = 200
	}
	rows, err := db.Query(`SELECT id, actor_id, action, entity, entity_id, created_at FROM audit_logs ORDER BY id DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AuditRow
	for rows.Next() {
		var r AuditRow
		var actor sql.NullInt64
		if err := rows.Scan(&r.ID, &actor, &r.Action, &r.Entity, &r.EntityID, &r.Created); err != nil {
			return nil, err
		}
		if actor.Valid {
			v := int(actor.Int64)
			r.ActorID = &v
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
