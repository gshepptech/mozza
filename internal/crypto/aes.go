package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
)

// Encrypt encrypts plaintext using AES-256-GCM with the given 32-byte key.
// The returned ciphertext has the nonce prepended.
func Encrypt(key, plaintext []byte) ([]byte, error) {
	if err := ValidateKey(key); err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("Encrypt: %w", err)
	}
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("Encrypt: %w", err)
	}
	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("Encrypt: %w", err)
	}
	return aesGCM.Seal(nonce, nonce, plaintext, nil), nil
}

// Decrypt decrypts ciphertext produced by Encrypt using the given 32-byte key.
func Decrypt(key, ciphertext []byte) ([]byte, error) {
	if err := ValidateKey(key); err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("Decrypt: %w", err)
	}
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("Decrypt: %w", err)
	}
	nonceSize := aesGCM.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("Decrypt: ciphertext too short")
	}
	return aesGCM.Open(nil, ciphertext[:nonceSize], ciphertext[nonceSize:], nil)
}

// ValidateKey checks that key is exactly 32 bytes (AES-256).
func ValidateKey(key []byte) error {
	if len(key) != 32 {
		return fmt.Errorf("ValidateKey: key must be 32 bytes, got %d", len(key))
	}
	return nil
}

// KeyFromBase64 decodes a base64-encoded string and validates it as a 32-byte key.
func KeyFromBase64(encoded string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("KeyFromBase64: %w", err)
	}
	if err := ValidateKey(key); err != nil {
		return nil, err
	}
	return key, nil
}
