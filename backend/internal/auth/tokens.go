package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
)

var ErrInvalidToken = errors.New("invalid token")

func GenerateToken() (string, error) {
	return generateTokenWithPrefix("paa_")
}

func GenerateAgentToken() (string, error) {
	return generateTokenWithPrefix("paa_agent_")
}

func generateTokenWithPrefix(prefix string) (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return prefix + base64.RawURLEncoding.EncodeToString(buf), nil
}

func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func VerifyTokenHash(token, expectedHash string) bool {
	if token == "" || expectedHash == "" {
		return false
	}
	actual := HashToken(token)
	return subtle.ConstantTimeCompare([]byte(actual), []byte(expectedHash)) == 1
}
