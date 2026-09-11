# Task 2 Brief — TOTP + Haversine domain libs, TDD (from docs/superpowers/plans/2026-09-08-atlas.md)

## Global Constraints (bind this task)

- TOTP: RFC6238 SHA1, 6-digit, period 5s, accept +-1 step.
- Geofence math: Haversine, meters, fail-closed (pure function here).
- Go >= 1.23, module `atlas`. Stdlib only — NO external deps (no `go get`).
  `go.mod` stays dependency-free.
- All files under `Desktop/attendance/api/internal/...`. Commit scoped to
  `Desktop/attendance` pathspecs only.

## Environment

- OS win32, PowerShell 5.1. Refresh PATH first if `go` is missing:
  `$env:Path = [System.Environment]::GetEnvironmentVariable("Path","Machine") + ";" + [System.Environment]::GetEnvironmentVariable("Path","User")`
- Work from `C:\Users\amirn\Desktop\attendance\api` for `go test` commands.

## Task

TDD order matters: write each test FIRST, run to see it FAIL, then implement,
then re-run to PASS. Report RED and GREEN evidence per test.

**Files:**
- Create: `api/internal/totp/totp.go`
- Test: `api/internal/totp/totp_test.go`
- Create: `api/internal/geo/geo.go`
- Test: `api/internal/geo/geo_test.go`

**Interfaces (exact — later tasks import these):**
- `totp.Generate(secret []byte, t time.Time) string`
- `totp.Valid(secret []byte, code string, t time.Time) bool`
- `totp.Period == 5` (exported const, int)
- `geo.HaversineM(lat1, lng1, lat2, lng2 float64) float64` (meters)

- [ ] **Step 1: Write failing totp test** (`api/internal/totp/totp_test.go`)

```go
package totp

import (
	"testing"
	"time"
)

func TestGenerateSixDigits(t *testing.T) {
	c := Generate([]byte("test-secret-32-bytes-long-1234"), time.Unix(0, 0))
	if len(c) != 6 {
		t.Fatalf("want 6 digits got %q", c)
	}
}

func TestValidAcceptsWindow(t *testing.T) {
	s := []byte("another-32-byte-secret-for-test!")
	now := time.Now()
	code := Generate(s, now)
	if !Valid(s, code, now.Add(4*time.Second)) {
		t.Fatal("same-step code must validate")
	}
	if !Valid(s, Generate(s, now.Add(-5*time.Second)), now) {
		t.Fatal("previous-step code must validate")
	}
	if Valid(s, "000000", now) && Generate(s, now) != "000000" {
		t.Fatal("wrong code must not validate")
	}
}
```

- [ ] **Step 2: Run totp test, verify FAIL**

Run: `go test ./internal/totp/ -v` (from `api/`). Expected: FAIL with
"undefined: Generate".

- [ ] **Step 3: Minimal TOTP implementation** (`api/internal/totp/totp.go`)

```go
package totp

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"time"
)

const Period = 5

func counter(t time.Time) uint64 { return uint64(t.Unix() / Period) }

func hotp(secret []byte, c uint64) string {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], c)
	m := hmac.New(sha1.New, secret)
	m.Write(b[:])
	s := m.Sum(nil)
	o := s[len(s)-1] & 0x0f
	v := binary.BigEndian.Uint32(s[o:o+4]) & 0x7fffffff
	return fmt.Sprintf("%06d", v%1000000)
}

func Generate(secret []byte, t time.Time) string { return hotp(secret, counter(t)) }

func Valid(secret []byte, code string, t time.Time) bool {
	c := counter(t)
	return code == hotp(secret, c-1) || code == hotp(secret, c) || code == hotp(secret, c+1)
}
```

- [ ] **Step 4: Write geo test + impl, run all**

`api/internal/geo/geo_test.go`:

```go
package geo

import "testing"

func TestHaversineKnownDistance(t *testing.T) {
	d := HaversineM(3.1390, 101.6869, 3.1391, 101.6870)
	if d < 10 || d > 25 {
		t.Fatalf("want ~15m got %v", d)
	}
	if HaversineM(3.1, 101.6, 3.1, 101.6) != 0 {
		t.Fatal("same point must be 0")
	}
}
```

`api/internal/geo/geo.go`:

```go
package geo

import "math"

func HaversineM(lat1, lng1, lat2, lng2 float64) float64 {
	const r = 6371000.0
	dLat := (lat2 - lat1) * math.Pi / 180
	dLng := (lng2 - lng1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*math.Sin(dLng/2)*math.Sin(dLng/2)
	return 2 * r * math.Asin(math.Sqrt(a))
}
```

Run: `go test ./internal/... -v` Expected: PASS. Also run `gofmt -l .`
Expected: clean (no output).

- [ ] **Step 5: Commit** (repo root `C:\Users\amirn`)

```bash
git add Desktop/attendance/api/internal/totp Desktop/attendance/api/internal/geo
git commit -m "feat(atlas): totp 5s engine and haversine gate"
```
