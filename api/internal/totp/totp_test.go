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
