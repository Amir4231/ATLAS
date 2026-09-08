package store

import (
	"database/sql"

	"golang.org/x/crypto/bcrypt"
)

func Seed(db *sql.DB) error {
	hash, err := bcrypt.GenerateFromPassword([]byte("Atlas123!"), 10)
	if err != nil {
		return err
	}

	users := []struct {
		fullName string
		email    string
		role     string
	}{
		{"System Admin", "admin@atlas.local", "admin"},
		{"Homeroom Teacher", "teacher@atlas.local", "teacher"},
		{"Class Rep", "rep@atlas.local", "class_rep"},
		{"Student One", "s1@atlas.local", "student"},
		{"Student Two", "s2@atlas.local", "student"},
	}

	for _, u := range users {
		if _, err := db.Exec(
			`INSERT INTO users(full_name, email, password_hash, role) VALUES ($1, $2, $3, $4) ON CONFLICT (email) DO NOTHING`,
			u.fullName, u.email, string(hash), u.role,
		); err != nil {
			return err
		}
	}

	var teacherID int
	if err := db.QueryRow(`SELECT id FROM users WHERE email = 'teacher@atlas.local'`).Scan(&teacherID); err != nil {
		return err
	}

	var classID int
	err = db.QueryRow(`SELECT id FROM classes WHERE class_name = 'CS-101'`).Scan(&classID)
	if err == sql.ErrNoRows {
		if err := db.QueryRow(
			`INSERT INTO classes(class_name, homeroom_teacher_id, geofence_lat, geofence_lng, geofence_radius_m) VALUES ('CS-101', $1, 3.1390, 101.6869, 30) RETURNING id`,
			teacherID,
		).Scan(&classID); err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		if _, err := db.Exec(`UPDATE classes SET homeroom_teacher_id = $1, geofence_lat = 3.1390, geofence_lng = 101.6869, geofence_radius_m = 30 WHERE id = $2`, teacherID, classID); err != nil {
			return err
		}
	}

	for _, email := range []string{"rep@atlas.local", "s1@atlas.local", "s2@atlas.local"} {
		if _, err := db.Exec(`UPDATE users SET class_id = $1 WHERE email = $2`, classID, email); err != nil {
			return err
		}
	}

	return nil
}
