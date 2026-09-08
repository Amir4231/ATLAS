# ATLAS Design Spec (2026-09-08)

Scope: full backend + minimal UI. PRD-exact stack. Approved by user.

## Decisions
- Backend: Go stdlib net/http API, JWT HS256 8h, bcrypt cost 10.
- Infra: Docker Compose — Postgres 16 (1GB), Go API (100MB), Redis 7 (128MB), Nginx (32MB), all with mem_limit.
- Frontend: static HTML+JS PWA in web/, served by Nginx, app-shell caching via service worker.
- Storage: local uploads/ behind Storage interface; S3/R2 env vars reserved (S3_ENDPOINT, S3_BUCKET, S3_ACCESS_KEY, S3_SECRET_KEY) for later swap.
- Notifications: in-app notifications table + GET API; push/email deferred.
- Seed: admin, teacher, class_rep, 2 students, 1 class with 30m geofence.
- Toolchain: install Go + Docker on Windows via winget during implementation.

## Architecture
Nginx :443/:80 -> /v1/* to go-api:8080, /* to static web/. Go -> Postgres (pgx/stdlib), Redis (totp cache, rate limit). TOTP secrets persisted in qr_sessions, cached in Redis with 5s granularity.

## Data model
- users(id, full_name, email unique, password_hash, role, class_id)
- classes(id, class_name, homeroom_teacher_id, geofence_lat, geofence_lng, geofence_radius_m default 30)
- qr_sessions(id, class_id, totp_secret, session_start, session_end nullable)
- attendance_records(id, student_id, class_id, session_id, status present|absent_unexcused|absent_excused, scan_lat, scan_lng, recorded_at, unique(student_id, session_id))
- absence_requests(id, student_id, attendance_id nullable, session_id, reason_type sick|outreach|personal, proof_file_url, status pending|approved|rejected, created_at)
- notifications(id, user_id, kind, title, body, read_at nullable, created_at)
- audit_logs(id, actor_id, action, entity, entity_id, created_at)
Indexes on (class_id, session_id), (student_id), users.email.

## API
- POST /v1/auth/login {email,password} -> {token}
- GET /v1/me
- POST /v1/qr/session {class_id} (rep) -> {session_id}
- GET /v1/qr/session/{id}/code (rep, polled every 5s) -> {code, exp}
- POST /v1/attendance/scan {session_id, code, lat, lng} (student)
- POST /v1/qr/session/{id}/close (rep) -> marks unmarked as absent_unexcused
- GET /v1/attendance?session_id= / class_id=
- POST /v1/absences (multipart: session_id, reason_type, file) (student)
- GET /v1/absences?status=pending (teacher)
- PATCH /v1/absences/{id}/review {decision: approved|rejected} (teacher, transaction)
- POST /v1/admin/classes (admin)
- GET /v1/classes
- GET /v1/analytics/class?class_id= GET /v1/analytics/college (teacher/admin)
- GET /v1/notifications PATCH /v1/notifications/{id}/read
- GET /v1/audit-logs (admin)

## Anti-proxy engine
- TOTP RFC6238 SHA1 6-digit period 5s, secret 32B base32 per session. QR payload {sid, code, exp}. Accept +-1 step.
- Haversine validation, reject if d > radius with 403 + distance_m.
- Rate limit 10/min/IP via Redis INCR+EXPIRE on scan endpoint.
- Idempotent re-scan returns 200 same record.

## Frontend (web/)
- /index.html role router + login; /rep.html QR canvas refresh 5s + live feed polling; /student.html manual code entry + geolocation GPS lock badge + absence form; /teacher.html rate cards + pending queue + doc iframe + approve/reject + charts; /admin.html class form + geofence + audit table. manifest.json + sw.js.

## Error handling
Fail-closed 401 INVALID_CODE, 403 OUT_OF_GEOFENCE, 409/200 duplicate idempotent, 400 file gate, 429 rate limited. Audit log every mutation.

## Testing
go test: haversine vectors, totp vectors/expiry/window, scan validator cases, file gate. Integration happy path via Compose. PWA smoke checklist. go vet + gofmt clean.

## Non-goals (deferred)
Web push, email digest 4pm, real S3 upload, Cloudflare WAF, SvelteKit/Vue migration, month-over-month graphs beyond basic breakdowns.
