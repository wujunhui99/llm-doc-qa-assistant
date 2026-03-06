package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const (
	defaultIterations = 120000
	saltSize          = 16
	keyLen            = 32
)

func HashPassword(password string) (string, error) {
	if len(password) < 8 {
		return "", errors.New("password must be at least 8 characters")
	}
	salt := make([]byte, saltSize)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	dk := pbkdf2SHA256([]byte(password), salt, defaultIterations, keyLen)
	return fmt.Sprintf("pbkdf2_sha256$%d$%s$%s", defaultIterations,
		base64.RawURLEncoding.EncodeToString(salt),
		base64.RawURLEncoding.EncodeToString(dk)), nil
}

func VerifyPassword(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2_sha256" {
		return false
	}
	iterations, err := strconv.Atoi(parts[1])
	if err != nil || iterations < 10000 {
		return false
	}
	salt, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return false
	}
	expected, err := base64.RawURLEncoding.DecodeString(parts[3])
	if err != nil {
		return false
	}
	actual := pbkdf2SHA256([]byte(password), salt, iterations, len(expected))
	return subtle.ConstantTimeCompare(actual, expected) == 1
}

func pbkdf2SHA256(password, salt []byte, iterations, keyLength int) []byte {
	hLen := 32
	numBlocks := (keyLength + hLen - 1) / hLen
	out := make([]byte, 0, numBlocks*hLen)

	for block := 1; block <= numBlocks; block++ {
		u := pbkdf2F(password, salt, iterations, block)
		out = append(out, u...)
	}
	return out[:keyLength]
}

func pbkdf2F(password, salt []byte, iterations, blockIndex int) []byte {
	u := make([]byte, 0, len(salt)+4)
	u = append(u, salt...)
	u = append(u,
		byte(blockIndex>>24),
		byte(blockIndex>>16),
		byte(blockIndex>>8),
		byte(blockIndex),
	)

	mac := hmac.New(sha256.New, password)
	mac.Write(u)
	uPrev := mac.Sum(nil)
	result := make([]byte, len(uPrev))
	copy(result, uPrev)

	for i := 1; i < iterations; i++ {
		mac = hmac.New(sha256.New, password)
		mac.Write(uPrev)
		uPrev = mac.Sum(nil)
		for j := range result {
			result[j] ^= uPrev[j]
		}
	}
	return result
}

func NewSessionToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
