package attend

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"sync"
	"time"

	"atlas/internal/geo"
	"atlas/internal/totp"

	"github.com/redis/go-redis/v9"
)

var (
	ErrInvalidCode   = errors.New("invalid code")
	ErrOutOfFence    = errors.New("outside geofence")
	ErrRateLimited   = errors.New("rate limited")
	ErrSessionClosed = errors.New("session closed")
)

type Limiter interface {
	Allow(key string) (bool, error)
}

type MemLimiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	hits   map[string][]int64
}

func NewMemLimiter(limit int, window time.Duration) *MemLimiter {
	return &MemLimiter{limit: limit, window: window, hits: make(map[string][]int64)}
}

func (m *MemLimiter) Allow(key string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UnixNano()
	cutoff := now - m.window.Nanoseconds()
	kept := m.hits[key][:0]
	for _, ts := range m.hits[key] {
		if ts > cutoff {
			kept = append(kept, ts)
		}
	}
	if len(kept) >= m.limit {
		m.hits[key] = kept
		return false, nil
	}
	m.hits[key] = append(kept, now)
	return true, nil
}

type RedisLimiter struct {
	rdb    *redis.Client
	limit  int
	window time.Duration
}

func NewRedisLimiter(rdb *redis.Client, limit int, window time.Duration) *RedisLimiter {
	return &RedisLimiter{rdb: rdb, limit: limit, window: window}
}

func (r *RedisLimiter) Allow(key string) (bool, error) {
	ctx := context.Background()
	n, err := r.rdb.Incr(ctx, key).Result()
	if err != nil {
		return false, err
	}
	if n == 1 {
		if err := r.rdb.Expire(ctx, key, r.window).Err(); err != nil {
			return false, err
		}
	}
	return n <= int64(r.limit), nil
}

func OpenSession(db *sql.DB, classID int) (id int, secret []byte, err error) {
	secret = make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return 0, nil, err
	}
	if err := db.QueryRow(
		`INSERT INTO qr_sessions(class_id, totp_secret) VALUES ($1, $2) RETURNING id`,
		classID, secret,
	).Scan(&id); err != nil {
		return 0, nil, err
	}
	return id, secret, nil
}

func Code(secret []byte, t time.Time) (code string, exp int64) {
	return totp.Generate(secret, t), (t.Unix()/5 + 1) * 5
}

func Scan(db *sql.DB, lim Limiter, ip string, sessionID int, studentID int, code string, lat, lng float64, t time.Time) (status string, distM float64, err error) {
	ok, err := lim.Allow("scan:" + ip)
	if err != nil {
		return "", 0, err
	}
	if !ok {
		return "", 0, ErrRateLimited
	}

	var secret []byte
	var flat, flng float64
	var radius int
	var sessionEnd sql.NullTime
	err = db.QueryRow(
		`SELECT totp_secret, geofence_lat, geofence_lng, geofence_radius_m, session_end FROM qr_sessions JOIN classes ON classes.id = qr_sessions.class_id WHERE qr_sessions.id = $1`,
		sessionID,
	).Scan(&secret, &flat, &flng, &radius, &sessionEnd)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", 0, ErrSessionClosed
		}
		return "", 0, err
	}
	if sessionEnd.Valid {
		return "", 0, ErrSessionClosed
	}

	if !totp.Valid(secret, code, t) {
		return "", 0, ErrInvalidCode
	}

	distM = geo.HaversineM(flat, flng, lat, lng)
	if distM > float64(radius) {
		return "", distM, ErrOutOfFence
	}

	var recordID int
	err = db.QueryRow(
		`INSERT INTO attendance_records(student_id, class_id, session_id, status, scan_lat, scan_lng) SELECT $1, class_id, $2, 'present', $3, $4 FROM qr_sessions WHERE id = $2 ON CONFLICT(student_id, session_id) DO NOTHING RETURNING id`,
		studentID, sessionID, lat, lng,
	).Scan(&recordID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "present", distM, nil
		}
		return "", distM, err
	}

	if _, err := db.Exec(
		`INSERT INTO audit_logs(actor_id, action, entity, entity_id) VALUES ($1, 'attendance.scan', 'attendance_record', $2)`,
		studentID, recordID,
	); err != nil {
		return "", distM, err
	}
	if _, err := db.Exec(
		`INSERT INTO notifications(user_id, kind, title, body) VALUES ($1, 'attendance.present', 'Attendance marked', 'You were marked present')`,
		studentID,
	); err != nil {
		return "", distM, err
	}

	return "present", distM, nil
}

func Close(db *sql.DB, sessionID int) (marked int64, err error) {
	if _, err := db.Exec(`UPDATE qr_sessions SET session_end = now() WHERE id = $1`, sessionID); err != nil {
		return 0, err
	}
	res, err := db.Exec(
		`INSERT INTO attendance_records(student_id, class_id, session_id, status) SELECT u.id, q.class_id, q.id, 'absent_unexcused' FROM qr_sessions q JOIN users u ON u.class_id = q.class_id WHERE q.id = $1 ON CONFLICT(student_id, session_id) DO NOTHING`,
		sessionID,
	)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
