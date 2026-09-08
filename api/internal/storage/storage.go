package storage

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

var (
	ErrBadType  = errors.New("unsupported file type")
	ErrTooLarge = errors.New("file too large")
)

const MaxUploadBytes = 5 << 20

var allowedTypes = map[string]bool{
	"application/pdf": true,
	"image/jpeg":      true,
	"image/png":       true,
}

func CheckUpload(data []byte, filename string) error {
	if len(data) > MaxUploadBytes {
		return ErrTooLarge
	}
	sniffLen := len(data)
	if sniffLen > 512 {
		sniffLen = 512
	}
	if !allowedTypes[http.DetectContentType(data[:sniffLen])] {
		return ErrBadType
	}
	return nil
}

type Storage interface {
	Save(filename string, data []byte) (url string, err error)
}

type LocalStorage struct {
	Dir     string
	BaseURL string
}

func (l LocalStorage) Save(filename string, data []byte) (string, error) {
	if err := CheckUpload(data, filename); err != nil {
		return "", err
	}
	if strings.ContainsAny(filename, `/\`) {
		return "", errors.New("invalid filename")
	}
	if err := os.MkdirAll(l.Dir, 0o755); err != nil {
		return "", err
	}
	var prefix [8]byte
	if _, err := rand.Read(prefix[:]); err != nil {
		return "", err
	}
	name := hex.EncodeToString(prefix[:]) + "-" + filename
	if err := os.WriteFile(filepath.Join(l.Dir, name), data, 0o644); err != nil {
		return "", err
	}
	return strings.TrimSuffix(l.BaseURL, "/") + "/" + name, nil
}
