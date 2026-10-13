package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"golang.org/x/crypto/argon2"
	"strings"
	"unicode/utf8"
)

var passwordSlots = make(chan struct{}, 2)

const hashPrefix = "$argon2id$v=19$m=19456,t=2,p=1$"

func validPassword(p string) bool { return len(p) >= 12 && len(p) <= 128 && utf8.ValidString(p) }
func acquirePassword(ctx context.Context) bool {
	select {
	case passwordSlots <- struct{}{}:
		return true
	case <-ctx.Done():
		return false
	}
}
func HashPassword(ctx context.Context, p string) (string, error) {
	if !validPassword(p) {
		return "", ErrPassword
	}
	if !acquirePassword(ctx) {
		return "", ctx.Err()
	}
	defer func() { <-passwordSlots }()
	salt := make([]byte, 16)
	if _, e := rand.Read(salt); e != nil {
		return "", e
	}
	hash := argon2.IDKey([]byte(p), salt, 2, 19*1024, 1, 32)
	return hashPrefix + base64.RawStdEncoding.EncodeToString(salt) + "$" + base64.RawStdEncoding.EncodeToString(hash), nil
}
func VerifyPassword(ctx context.Context, encoded, p string) bool {
	if !validPassword(p) || !strings.HasPrefix(encoded, hashPrefix) {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(encoded, hashPrefix), "$")
	if len(parts) != 2 {
		return false
	}
	salt, e := base64.RawStdEncoding.DecodeString(parts[0])
	if e != nil || len(salt) != 16 {
		return false
	}
	expected, e := base64.RawStdEncoding.DecodeString(parts[1])
	if e != nil || len(expected) != 32 {
		return false
	}
	if !acquirePassword(ctx) {
		return false
	}
	defer func() { <-passwordSlots }()
	actual := argon2.IDKey([]byte(p), salt, 2, 19*1024, 1, 32)
	return subtle.ConstantTimeCompare(actual, expected) == 1
}
