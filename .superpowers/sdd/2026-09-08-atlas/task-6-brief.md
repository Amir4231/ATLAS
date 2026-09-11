# Task 6 Brief — Absence upload + review, TDD (from docs/superpowers/plans/2026-09-08-atlas.md)

## Global Constraints (bind this task)

- Uploads: PDF/JPEG/PNG only, max 5MB, MIME-sniffed (never trust extension);
  local-disk MVP behind a Storage interface with S3 env vars reserved.
- Reasons: sick|outreach|personal only. New requests start pending.
- Review is ONE SQL transaction: absence status + attendance status +
  notifications + audit all commit or all roll back. Approve flips attendance
  to absent_excused (creating the row if Close hasn't run yet).
- No live DB (arrives Task 9): tests use go-sqlmock (already in go.mod).
- Commits scoped to Desktop/attendance pathspecs only.

## Environment

- PowerShell 5.1, refresh PATH for `go` if needed.
- Work from `C:\Users\amirn\Desktop\attendance\api`.

## Task

**Files:**
- Create: `api/internal/storage/storage.go`
- Test: `api/internal/storage/storage_test.go`
- Create: `api/internal/absent/absent.go`
- Test: `api/internal/absent/absent_test.go`

**Interfaces (exact — Task 8 imports these):**
- `var ErrBadType, ErrTooLarge error` (storage)
- `const MaxUploadBytes = 5 << 20`
- `func CheckUpload(data []byte, filename string) error` — size > 5MB →
  ErrTooLarge; `http.DetectContentType(data[:min(512,len)])` not in
  {application/pdf, image/jpeg, image/png} → ErrBadType. Extension ignored.
- `type Storage interface { Save(filename string, data []byte) (url string, err error) }`
- `type LocalStorage struct { Dir, BaseURL string }` +
  `func (l LocalStorage) Save(filename string, data []byte) (string, error)` —
  MkdirAll Dir, reject path separators in filename, random 16-hex prefix,
  write 0600, return BaseURL + "/" + stored name. Re-runs CheckUpload first.
- `var ErrBadReason, ErrNotPending error` (absent)
- `func Submit(db *sql.DB, studentID, sessionID int, reason, proofURL string) (id int, err error)` — reason not in set → ErrBadReason (no queries); else: lookup homeroom teacher id via `SELECT classes.homeroom_teacher_id ... JOIN qr_sessions ... WHERE qr_sessions.id=$1`; `INSERT INTO absence_requests(student_id,session_id,reason_type,proof_file_url) VALUES(...) RETURNING id`; notification to teacher (`absence.pending`); audit (`absence.submit`). Return id.
- `func Review(db *sql.DB, absenceID int, approve bool) error` — tx: `SELECT student_id, session_id, status FROM absence_requests WHERE id=$1` (row missing → `sql.ErrNoRows` propagates); status != pending → ROLLBACK + ErrNotPending; `UPDATE absence_requests SET status=$1 WHERE id=$2`; if approve: `UPDATE attendance_records SET status='absent_excused' WHERE student_id=$1 AND session_id=$2`, if RowsAffected==0 then `INSERT (student_id, class_id, session_id, status)` with class_id looked up from session (SELECT class_id FROM qr_sessions WHERE id=$1); notification to student (`absence.reviewed`); audit (`absence.review`); COMMIT.

- [ ] **Step 1: Failing tests**

`storage_test.go`: `TestRejectsBadMIME` (EXE bytes + "evil.exe" → ErrBadType),
`TestRejectsOversize` (6MB zeros + "big.pdf" → ErrTooLarge), `TestAcceptsPDF`
(`%PDF-1.4...` → nil), `TestLocalSaveRoundTrip` (t.TempDir, save PNG-header
bytes, file exists, url has BaseURL prefix).
`absent_test.go` (sqlmock): `TestSubmitBadReason` (no queries expected);
`TestSubmitHappy` (teacher SELECT → INSERT RETURNING 9 → notif → audit);
`TestReviewApprove` (BEGIN → SELECT pending row → UPDATE approved →
UPDATE attendance rows=1 → notif → audit → COMMIT);
`TestReviewNotPending` (SELECT approved row → ROLLBACK, ErrNotPending).

- [ ] **Step 2: Run, verify FAIL**

Run: `go test ./internal/storage/ ./internal/absent/ -v`
Expected: FAIL "undefined: CheckUpload" (or Submit).

- [ ] **Step 3: Minimal impl** per interfaces above.

- [ ] **Step 4: Run PASS + commit**

Run: `go test ./internal/... -v` Expected: PASS.
Run: `gofmt -l .` clean, `go vet ./...` clean, `go build ./...` PASS.

```bash
git add Desktop/attendance/api/internal/storage Desktop/attendance/api/internal/absent
git commit -m "feat(atlas): absence pipeline with file gate"
```
