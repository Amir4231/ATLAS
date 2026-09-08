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
