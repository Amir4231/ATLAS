package absent

import (
	"database/sql"
	"errors"
)

var (
	ErrBadReason  = errors.New("invalid reason")
	ErrNotPending = errors.New("absence request is not pending")
)

func validReason(reason string) bool {
	switch reason {
	case "sick", "outreach", "personal":
		return true
	}
	return false
}

func Submit(db *sql.DB, studentID, sessionID int, reason, proofURL string) (id int, err error) {
	if !validReason(reason) {
		return 0, ErrBadReason
	}

	var teacherID int
	if err := db.QueryRow(
		`SELECT classes.homeroom_teacher_id FROM classes JOIN qr_sessions ON qr_sessions.class_id = classes.id WHERE qr_sessions.id = $1`,
		sessionID,
	).Scan(&teacherID); err != nil {
		return 0, err
	}

	if err := db.QueryRow(
		`INSERT INTO absence_requests(student_id, session_id, reason_type, proof_file_url) VALUES ($1, $2, $3, $4) RETURNING id`,
		studentID, sessionID, reason, proofURL,
	).Scan(&id); err != nil {
		return 0, err
	}

	if _, err := db.Exec(
		`INSERT INTO notifications(user_id, kind, title, body) VALUES ($1, 'absence.pending', 'Absence request pending', 'A student submitted an absence request')`,
		teacherID,
	); err != nil {
		return 0, err
	}
	if _, err := db.Exec(
		`INSERT INTO audit_logs(actor_id, action, entity, entity_id) VALUES ($1, 'absence.submit', 'absence_request', $2)`,
		studentID, id,
	); err != nil {
		return 0, err
	}
	return id, nil
}

func Review(db *sql.DB, absenceID int, approve bool) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	rollback := func(err error) error {
		_ = tx.Rollback()
		return err
	}

	var studentID, sessionID int
	var status string
	if err := tx.QueryRow(
		`SELECT student_id, session_id, status FROM absence_requests WHERE id = $1`,
		absenceID,
	).Scan(&studentID, &sessionID, &status); err != nil {
		return rollback(err)
	}
	if status != "pending" {
		return rollback(ErrNotPending)
	}

	newStatus := "rejected"
	if approve {
		newStatus = "approved"
	}
	if _, err := tx.Exec(
		`UPDATE absence_requests SET status = $1 WHERE id = $2`,
		newStatus, absenceID,
	); err != nil {
		return rollback(err)
	}

	if approve {
		res, err := tx.Exec(
			`UPDATE attendance_records SET status = 'absent_excused' WHERE student_id = $1 AND session_id = $2`,
			studentID, sessionID,
		)
		if err != nil {
			return rollback(err)
		}
		n, err := res.RowsAffected()
		if err != nil {
			return rollback(err)
		}
		if n == 0 {
			var classID int
			if err := tx.QueryRow(
				`SELECT class_id FROM qr_sessions WHERE id = $1`,
				sessionID,
			).Scan(&classID); err != nil {
				return rollback(err)
			}
			if _, err := tx.Exec(
				`INSERT INTO attendance_records(student_id, class_id, session_id, status) VALUES ($1, $2, $3, 'absent_excused')`,
				studentID, classID, sessionID,
			); err != nil {
				return rollback(err)
			}
		}
	}

	if _, err := tx.Exec(
		`INSERT INTO notifications(user_id, kind, title, body) VALUES ($1, 'absence.reviewed', 'Absence reviewed', 'Your absence request was reviewed')`,
		studentID,
	); err != nil {
		return rollback(err)
	}
	if _, err := tx.Exec(
		`INSERT INTO audit_logs(actor_id, action, entity, entity_id) VALUES ($1, 'absence.review', 'absence_request', $2)`,
		studentID, absenceID,
	); err != nil {
		return rollback(err)
	}
	return tx.Commit()
}
