package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"atlas/internal/absent"
	"atlas/internal/attend"
	"atlas/internal/auth"
	"atlas/internal/report"
	"atlas/internal/storage"
)

type Server struct {
	DB        *sql.DB
	Limiter   attend.Limiter
	JWTSecret string
	Store     storage.Storage
}

func NewServer(db *sql.DB, lim attend.Limiter, secret string, st storage.Storage) *Server {
	return &Server{DB: db, Limiter: lim, JWTSecret: secret, Store: st}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func (s *Server) prot(h http.HandlerFunc, allowed ...string) http.Handler {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		role := auth.Role(r)
		for _, a := range allowed {
			if role == a {
				h.ServeHTTP(w, r)
				return
			}
		}
		writeErr(w, http.StatusForbidden, "FORBIDDEN")
	})
	if len(allowed) == 0 {
		return auth.Middleware(s.JWTSecret, h)
	}
	return auth.Middleware(s.JWTSecret, inner)
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /v1/auth/login", s.handleLogin)

	mux.Handle("GET /v1/me", s.prot(s.handleMe))
	mux.Handle("POST /v1/qr/session", s.prot(s.handleOpenSession, "class_rep", "rep", "admin"))
	mux.Handle("GET /v1/qr/session/{id}/code", s.prot(s.handleGetCode, "class_rep", "rep", "admin"))
	mux.Handle("POST /v1/attendance/scan", s.prot(s.handleScan, "student"))
	mux.Handle("POST /v1/qr/session/{id}/close", s.prot(s.handleCloseSession, "class_rep", "rep", "admin"))
	mux.Handle("GET /v1/attendance", s.prot(s.handleAttendanceList))
	mux.Handle("POST /v1/absences", s.prot(s.handleAbsenceSubmit, "student"))
	mux.Handle("GET /v1/absences", s.prot(s.handleAbsenceList))
	mux.Handle("PATCH /v1/absences/{id}/review", s.prot(s.handleAbsenceReview, "teacher", "admin"))
	mux.Handle("POST /v1/admin/classes", s.prot(s.handleAdminCreateClass, "admin"))
	mux.Handle("GET /v1/classes", s.prot(s.handleClassesList))
	mux.Handle("GET /v1/analytics/class", s.prot(s.handleAnalyticsClass, "teacher", "admin"))
	mux.Handle("GET /v1/analytics/college", s.prot(s.handleAnalyticsCollege, "admin"))
	mux.Handle("GET /v1/notifications", s.prot(s.handleNotificationsList))
	mux.Handle("PATCH /v1/notifications/{id}/read", s.prot(s.handleNotificationRead))
	mux.Handle("GET /v1/audit-logs", s.prot(s.handleAuditLogs, "admin"))

	return mux
}

func clientIP(r *http.Request) string {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "BAD_REQUEST")
		return
	}
	var id int
	var hash, role string
	err := s.DB.QueryRow(
		`SELECT id, password_hash, role FROM users WHERE email=$1`,
		req.Email,
	).Scan(&id, &hash, &role)
	if err != nil || !auth.Check(req.Password, hash) {
		writeErr(w, http.StatusUnauthorized, "INVALID_CREDENTIALS")
		return
	}
	tok, err := auth.Token(strconv.Itoa(id), role, s.JWTSecret)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"token": tok})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	uid, err := strconv.Atoi(auth.UID(r))
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "UNAUTHORIZED")
		return
	}
	var me struct {
		ID       int    `json:"id"`
		FullName string `json:"full_name"`
		Email    string `json:"email"`
		Role     string `json:"role"`
		ClassID  *int   `json:"class_id"`
	}
	var classID sql.NullInt64
	err = s.DB.QueryRow(
		`SELECT id, full_name, email, role, class_id FROM users WHERE id=$1`,
		uid,
	).Scan(&me.ID, &me.FullName, &me.Email, &me.Role, &classID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeErr(w, http.StatusNotFound, "NOT_FOUND")
			return
		}
		writeErr(w, http.StatusInternalServerError, "INTERNAL")
		return
	}
	if classID.Valid {
		v := int(classID.Int64)
		me.ClassID = &v
	}
	writeJSON(w, http.StatusOK, me)
}

func (s *Server) handleOpenSession(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ClassID int `json:"class_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ClassID == 0 {
		writeErr(w, http.StatusBadRequest, "BAD_REQUEST")
		return
	}
	id, _, err := attend.OpenSession(s.DB, req.ClassID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL")
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"session_id": id})
}

func (s *Server) handleGetCode(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "BAD_REQUEST")
		return
	}
	secret, err := attend.SessionSecret(s.DB, id)
	if err != nil {
		if errors.Is(err, attend.ErrSessionClosed) {
			writeErr(w, http.StatusGone, "SESSION_CLOSED")
			return
		}
		writeErr(w, http.StatusInternalServerError, "INTERNAL")
		return
	}
	code, exp := attend.Code(secret, time.Now())
	writeJSON(w, http.StatusOK, map[string]any{"code": code, "exp": exp})
}
func (s *Server) handleScan(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SessionID int     `json:"session_id"`
		Code      string  `json:"code"`
		Lat       float64 `json:"lat"`
		Lng       float64 `json:"lng"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.SessionID == 0 {
		writeErr(w, http.StatusBadRequest, "BAD_REQUEST")
		return
	}
	studentID, err := strconv.Atoi(auth.UID(r))
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "UNAUTHORIZED")
		return
	}
	status, dist, err := attend.Scan(s.DB, s.Limiter, clientIP(r), req.SessionID, studentID, req.Code, req.Lat, req.Lng, time.Now())
	switch {
	case errors.Is(err, attend.ErrInvalidCode):
		writeErr(w, http.StatusUnauthorized, "INVALID_CODE")
		return
	case errors.Is(err, attend.ErrOutOfFence):
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "OUT_OF_GEOFENCE", "distance_m": dist})
		return
	case errors.Is(err, attend.ErrRateLimited):
		writeErr(w, http.StatusTooManyRequests, "RATE_LIMITED")
		return
	case errors.Is(err, attend.ErrSessionClosed):
		writeErr(w, http.StatusGone, "SESSION_CLOSED")
		return
	case err != nil:
		writeErr(w, http.StatusInternalServerError, "INTERNAL")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": status, "distance_m": dist})
}

func (s *Server) handleCloseSession(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "BAD_REQUEST")
		return
	}
	marked, err := attend.Close(s.DB, id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL")
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"marked": marked})
}

type attendanceRecord struct {
	ID         int      `json:"id"`
	StudentID  int      `json:"student_id"`
	Status     string   `json:"status"`
	ScanLat    *float64 `json:"scan_lat"`
	ScanLng    *float64 `json:"scan_lng"`
	RecordedAt string   `json:"recorded_at"`
}

func (s *Server) handleAttendanceList(w http.ResponseWriter, r *http.Request) {
	sessionID, err := strconv.Atoi(r.URL.Query().Get("session_id"))
	if err != nil || sessionID == 0 {
		writeErr(w, http.StatusBadRequest, "BAD_REQUEST")
		return
	}
	query := `SELECT id, student_id, status, scan_lat, scan_lng, recorded_at FROM attendance_records WHERE session_id=$1`
	args := []any{sessionID}
	if auth.Role(r) == "student" {
		studentID, err := strconv.Atoi(auth.UID(r))
		if err != nil {
			writeErr(w, http.StatusUnauthorized, "UNAUTHORIZED")
			return
		}
		query += ` AND student_id=$2`
		args = append(args, studentID)
	}
	query += ` ORDER BY id`
	rows, err := s.DB.Query(query, args...)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL")
		return
	}
	defer rows.Close()
	records := []attendanceRecord{}
	for rows.Next() {
		var rec attendanceRecord
		var lat, lng sql.NullFloat64
		var recorded time.Time
		if err := rows.Scan(&rec.ID, &rec.StudentID, &rec.Status, &lat, &lng, &recorded); err != nil {
			writeErr(w, http.StatusInternalServerError, "INTERNAL")
			return
		}
		if lat.Valid {
			v := lat.Float64
			rec.ScanLat = &v
		}
		if lng.Valid {
			v := lng.Float64
			rec.ScanLng = &v
		}
		rec.RecordedAt = recorded.Format(time.RFC3339)
		records = append(records, rec)
	}
	if err := rows.Err(); err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"records": records})
}

func (s *Server) handleAbsenceSubmit(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 6<<20)
	if err := r.ParseMultipartForm(6 << 20); err != nil {
		writeErr(w, http.StatusBadRequest, "BAD_REQUEST")
		return
	}
	sessionID, err := strconv.Atoi(r.FormValue("session_id"))
	if err != nil || sessionID == 0 {
		writeErr(w, http.StatusBadRequest, "BAD_REQUEST")
		return
	}
	reason := r.FormValue("reason_type")
	f, hdr, err := r.FormFile("file")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "BAD_REQUEST")
		return
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "BAD_REQUEST")
		return
	}
	studentID, err := strconv.Atoi(auth.UID(r))
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "UNAUTHORIZED")
		return
	}
	proofURL, err := s.Store.Save(hdr.Filename, data)
	if err != nil {
		switch {
		case errors.Is(err, storage.ErrTooLarge):
			writeErr(w, http.StatusRequestEntityTooLarge, "FILE_TOO_LARGE")
		case errors.Is(err, storage.ErrBadType):
			writeErr(w, http.StatusBadRequest, "INVALID_FILE_TYPE")
		default:
			writeErr(w, http.StatusBadRequest, "BAD_REQUEST")
		}
		return
	}
	id, err := absent.Submit(s.DB, studentID, sessionID, reason, proofURL)
	if err != nil {
		if errors.Is(err, absent.ErrBadReason) {
			writeErr(w, http.StatusBadRequest, "BAD_REASON")
			return
		}
		writeErr(w, http.StatusInternalServerError, "INTERNAL")
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"id": id})
}

type absenceRequest struct {
	ID           int    `json:"id"`
	StudentID    int    `json:"student_id"`
	SessionID    int    `json:"session_id"`
	ReasonType   string `json:"reason_type"`
	ProofFileURL string `json:"proof_file_url"`
	Status       string `json:"status"`
	CreatedAt    string `json:"created_at"`
}

func (s *Server) handleAbsenceList(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	query := `SELECT id, student_id, session_id, reason_type, proof_file_url, status, created_at FROM absence_requests`
	var conds []string
	var args []any
	if auth.Role(r) == "student" {
		studentID, err := strconv.Atoi(auth.UID(r))
		if err != nil {
			writeErr(w, http.StatusUnauthorized, "UNAUTHORIZED")
			return
		}
		args = append(args, studentID)
		conds = append(conds, `student_id=$`+strconv.Itoa(len(args)))
	}
	if status != "" {
		args = append(args, status)
		conds = append(conds, `status=$`+strconv.Itoa(len(args)))
	}
	if len(conds) > 0 {
		query += ` WHERE ` + strings.Join(conds, " AND ")
	}
	query += ` ORDER BY id DESC`
	rows, err := s.DB.Query(query, args...)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL")
		return
	}
	defer rows.Close()
	requests := []absenceRequest{}
	for rows.Next() {
		var ar absenceRequest
		var created time.Time
		if err := rows.Scan(&ar.ID, &ar.StudentID, &ar.SessionID, &ar.ReasonType, &ar.ProofFileURL, &ar.Status, &created); err != nil {
			writeErr(w, http.StatusInternalServerError, "INTERNAL")
			return
		}
		ar.CreatedAt = created.Format(time.RFC3339)
		requests = append(requests, ar)
	}
	if err := rows.Err(); err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"requests": requests})
}

func (s *Server) handleAbsenceReview(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "BAD_REQUEST")
		return
	}
	var req struct {
		Decision string `json:"decision"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "BAD_REQUEST")
		return
	}
	var approve bool
	var status string
	switch req.Decision {
	case "approved":
		approve, status = true, "approved"
	case "rejected":
		approve, status = false, "rejected"
	default:
		writeErr(w, http.StatusBadRequest, "BAD_REQUEST")
		return
	}
	if err := absent.Review(s.DB, id, approve); err != nil {
		if errors.Is(err, absent.ErrNotPending) {
			writeErr(w, http.StatusConflict, "NOT_PENDING")
			return
		}
		writeErr(w, http.StatusInternalServerError, "INTERNAL")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": status})
}

func (s *Server) handleAdminCreateClass(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ClassName         string  `json:"class_name"`
		HomeroomTeacherID *int    `json:"homeroom_teacher_id"`
		GeofenceLat       float64 `json:"geofence_lat"`
		GeofenceLng       float64 `json:"geofence_lng"`
		GeofenceRadiusM   int     `json:"geofence_radius_m"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ClassName == "" {
		writeErr(w, http.StatusBadRequest, "BAD_REQUEST")
		return
	}
	var teacherID any
	if req.HomeroomTeacherID != nil {
		teacherID = *req.HomeroomTeacherID
	}
	var id int
	err := s.DB.QueryRow(
		`INSERT INTO classes(class_name, homeroom_teacher_id, geofence_lat, geofence_lng, geofence_radius_m) VALUES ($1,$2,$3,$4,$5) RETURNING id`,
		req.ClassName, teacherID, req.GeofenceLat, req.GeofenceLng, req.GeofenceRadiusM,
	).Scan(&id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL")
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"id": id})
}

type classRow struct {
	ID                int     `json:"id"`
	ClassName         string  `json:"class_name"`
	HomeroomTeacherID *int    `json:"homeroom_teacher_id"`
	GeofenceLat       float64 `json:"geofence_lat"`
	GeofenceLng       float64 `json:"geofence_lng"`
	GeofenceRadiusM   int     `json:"geofence_radius_m"`
}

func (s *Server) handleClassesList(w http.ResponseWriter, r *http.Request) {
	rows, err := s.DB.Query(
		`SELECT id, class_name, homeroom_teacher_id, geofence_lat, geofence_lng, geofence_radius_m FROM classes ORDER BY id`,
	)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL")
		return
	}
	defer rows.Close()
	classes := []classRow{}
	for rows.Next() {
		var c classRow
		var teacherID sql.NullInt64
		if err := rows.Scan(&c.ID, &c.ClassName, &teacherID, &c.GeofenceLat, &c.GeofenceLng, &c.GeofenceRadiusM); err != nil {
			writeErr(w, http.StatusInternalServerError, "INTERNAL")
			return
		}
		if teacherID.Valid {
			v := int(teacherID.Int64)
			c.HomeroomTeacherID = &v
		}
		classes = append(classes, c)
	}
	if err := rows.Err(); err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"classes": classes})
}
func (s *Server) handleAnalyticsClass(w http.ResponseWriter, r *http.Request) {
	sessionID, err := strconv.Atoi(r.URL.Query().Get("session_id"))
	if err != nil || sessionID == 0 {
		writeErr(w, http.StatusBadRequest, "BAD_REQUEST")
		return
	}
	rate, err := report.SessionRate(s.DB, sessionID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL")
		return
	}
	cats, err := report.CategoryBreakdown(s.DB, sessionID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL")
		return
	}
	type category struct {
		Reason string `json:"reason"`
		Count  int    `json:"count"`
	}
	out := []category{}
	for _, c := range cats {
		out = append(out, category{Reason: c.Reason, Count: c.Count})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"rate":       map[string]any{"present": rate.Present, "total": rate.Total, "pct": rate.Pct},
		"categories": out,
	})
}

func (s *Server) handleAnalyticsCollege(w http.ResponseWriter, r *http.Request) {
	rate, err := report.CollegeRate(s.DB)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"present": rate.Present, "total": rate.Total, "pct": rate.Pct})
}

type notificationRow struct {
	ID        int     `json:"id"`
	Kind      string  `json:"kind"`
	Title     string  `json:"title"`
	Body      string  `json:"body"`
	ReadAt    *string `json:"read_at"`
	CreatedAt string  `json:"created_at"`
}

func (s *Server) handleNotificationsList(w http.ResponseWriter, r *http.Request) {
	userID, err := strconv.Atoi(auth.UID(r))
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "UNAUTHORIZED")
		return
	}
	notifs, err := report.ListNotifications(s.DB, userID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL")
		return
	}
	out := []notificationRow{}
	for _, n := range notifs {
		row := notificationRow{
			ID: n.ID, Kind: n.Kind, Title: n.Title, Body: n.Body,
			CreatedAt: n.Created.Format(time.RFC3339),
		}
		if n.Read != nil {
			v := n.Read.Format(time.RFC3339)
			row.ReadAt = &v
		}
		out = append(out, row)
	}
	writeJSON(w, http.StatusOK, map[string]any{"notifications": out})
}

func (s *Server) handleNotificationRead(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "BAD_REQUEST")
		return
	}
	userID, err := strconv.Atoi(auth.UID(r))
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "UNAUTHORIZED")
		return
	}
	if err := report.MarkRead(s.DB, id, userID); err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

type auditRow struct {
	ID       int    `json:"id"`
	ActorID  *int   `json:"actor_id"`
	Action   string `json:"action"`
	Entity   string `json:"entity"`
	EntityID string `json:"entity_id"`
	Created  string `json:"created_at"`
}

func (s *Server) handleAuditLogs(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "BAD_REQUEST")
			return
		}
		limit = n
	}
	logs, err := report.ListAudit(s.DB, limit)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL")
		return
	}
	out := []auditRow{}
	for _, l := range logs {
		out = append(out, auditRow{
			ID: l.ID, ActorID: l.ActorID, Action: l.Action,
			Entity: l.Entity, EntityID: l.EntityID,
			Created: l.Created.Format(time.RFC3339),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"logs": out})
}
