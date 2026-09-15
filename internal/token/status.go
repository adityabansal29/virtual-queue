package token

import (
	"crypto/sha256"
	"crypto/subtle"
)

// HashStatusSecret stores only a digest; the raw status capability never enters Redis.
func HashStatusSecret(secret string) string {
	h := sha256.Sum256([]byte(secret))
	return string(h[:])
}

// StatusSecretMatches uses constant-time comparison for the queue status capability.
func StatusSecretMatches(stored, supplied string) bool {
	return stored != "" && subtle.ConstantTimeCompare([]byte(stored), []byte(HashStatusSecret(supplied))) == 1
}
