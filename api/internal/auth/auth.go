package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalid = errors.New("invalid token")
	ErrExpired = errors.New("token expired")
)

const tokenHeader = `{"alg":"HS256","typ":"JWT"}`

var b64 = base64.RawURLEncoding

type ctxKey string

const (
	ctxUID  ctxKey = "uid"
	ctxRole ctxKey = "role"
)

func Hash(pw string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(pw), 10)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func Check(pw, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) == nil
}

func Token(uid, role, secret string) (string, error) {
	exp := time.Now().Add(8 * time.Hour).Unix()
	headerB64 := b64.EncodeToString([]byte(tokenHeader))
	payloadB64 := b64.EncodeToString([]byte(uid + "|" + role + "|" + strconv.FormatInt(exp, 10)))
	signingInput := headerB64 + "." + payloadB64
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signingInput))
	sig := b64.EncodeToString(mac.Sum(nil))
	return signingInput + "." + sig, nil
}

func Parse(tok, secret string) (string, string, error) {
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		return "", "", ErrInvalid
	}
	signingInput := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signingInput))
	want := mac.Sum(nil)
	got, err := b64.DecodeString(parts[2])
	if err != nil {
		return "", "", ErrInvalid
	}
	if subtle.ConstantTimeCompare(got, want) != 1 {
		return "", "", ErrInvalid
	}
	payload, err := b64.DecodeString(parts[1])
	if err != nil {
		return "", "", ErrInvalid
	}
	fields := strings.SplitN(string(payload), "|", 3)
	if len(fields) != 3 {
		return "", "", ErrInvalid
	}
	exp, err := strconv.ParseInt(fields[2], 10, 64)
	if err != nil {
		return "", "", ErrInvalid
	}
	if time.Now().Unix() > exp {
		return "", "", ErrExpired
	}
	return fields[0], fields[1], nil
}

func Middleware(secret string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := r.Header.Get("Authorization")
		if !strings.HasPrefix(h, "Bearer ") {
			unauthorized(w)
			return
		}
		uid, role, err := Parse(strings.TrimPrefix(h, "Bearer "), secret)
		if err != nil {
			unauthorized(w)
			return
		}
		ctx := context.WithValue(r.Context(), ctxUID, uid)
		ctx = context.WithValue(ctx, ctxRole, role)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func unauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte(`{"error":"UNAUTHORIZED"}`))
}

func UID(r *http.Request) string {
	v, _ := r.Context().Value(ctxUID).(string)
	return v
}

func Role(r *http.Request) string {
	v, _ := r.Context().Value(ctxRole).(string)
	return v
}
