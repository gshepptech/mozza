package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// tokenBytes is the number of random bytes in a session token.
const tokenBytes = 32

// GenerateToken creates a cryptographically random session token.
func GenerateToken() (string, error) {
	b := make([]byte, tokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("GenerateToken: %w", err)
	}
	return hex.EncodeToString(b), nil
}
