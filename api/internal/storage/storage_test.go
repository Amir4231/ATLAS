package storage

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRejectsBadMIME(t *testing.T) {
	data := append([]byte("MZ"), bytes.Repeat([]byte{0x90}, 100)...)
	if err := CheckUpload(data, "evil.exe"); !errors.Is(err, ErrBadType) {
		t.Fatalf("expected ErrBadType, got %v", err)
	}
}

func TestRejectsOversize(t *testing.T) {
	data := make([]byte, 6<<20)
	copy(data, []byte("%PDF-1.4"))
	if err := CheckUpload(data, "big.pdf"); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("expected ErrTooLarge, got %v", err)
	}
}

func TestAcceptsPDF(t *testing.T) {
	data := []byte("%PDF-1.4 fake pdf content for sniffing")
	if err := CheckUpload(data, "doc.pdf"); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestLocalSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	l := LocalStorage{Dir: dir, BaseURL: "http://x/files"}
	data := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0}, 100)...)

	url, err := l.Save("proof.png", data)
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if !strings.HasPrefix(url, "http://x/files/") {
		t.Fatalf("url missing BaseURL prefix: %q", url)
	}
	name := strings.TrimPrefix(url, "http://x/files/")
	if strings.ContainsAny(name, `/\`) {
		t.Fatalf("stored name has separator: %q", name)
	}
	got, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Fatal("round-trip bytes differ")
	}
}

func TestSaveRejectsPathTraversal(t *testing.T) {
	l := LocalStorage{Dir: t.TempDir(), BaseURL: "http://x/files"}
	data := []byte("%PDF-1.4 hi")
	if _, err := l.Save("../evil.pdf", data); err == nil {
		t.Fatal("expected error for separator in filename")
	}
}
