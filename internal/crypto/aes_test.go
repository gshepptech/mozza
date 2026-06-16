package crypto

import (
	"crypto/rand"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validKey(t *testing.T) []byte {
	t.Helper()
	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err)
	return key
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key := validKey(t)

	tests := []struct {
		name      string
		plaintext []byte
	}{
		{"empty", nil},
		{"short", []byte("hello")},
		{"longer", []byte("the quick brown fox jumps over the lazy dog")},
		{"binary", func() []byte { b := make([]byte, 256); rand.Read(b); return b }()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ct, err := Encrypt(key, tt.plaintext)
			require.NoError(t, err)
			assert.NotEqual(t, tt.plaintext, ct, "ciphertext should differ from plaintext")

			got, err := Decrypt(key, ct)
			require.NoError(t, err)
			assert.Equal(t, tt.plaintext, got)
		})
	}
}

func TestEncryptProducesDifferentCiphertexts(t *testing.T) {
	key := validKey(t)
	plain := []byte("deterministic?")

	ct1, err := Encrypt(key, plain)
	require.NoError(t, err)
	ct2, err := Encrypt(key, plain)
	require.NoError(t, err)

	assert.NotEqual(t, ct1, ct2, "each encryption should use a unique nonce")
}

func TestDecryptWrongKey(t *testing.T) {
	key1 := validKey(t)
	key2 := validKey(t)

	ct, err := Encrypt(key1, []byte("secret"))
	require.NoError(t, err)

	_, err = Decrypt(key2, ct)
	assert.Error(t, err)
}

func TestDecryptTruncatedCiphertext(t *testing.T) {
	key := validKey(t)
	_, err := Decrypt(key, []byte("short"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ciphertext too short")
}

func TestValidateKey(t *testing.T) {
	tests := []struct {
		name    string
		key     []byte
		wantErr bool
	}{
		{"valid 32 bytes", make([]byte, 32), false},
		{"too short", make([]byte, 16), true},
		{"too long", make([]byte, 64), true},
		{"empty", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateKey(tt.key)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "32 bytes")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestKeyFromBase64(t *testing.T) {
	raw := validKey(t)
	encoded := base64.StdEncoding.EncodeToString(raw)

	key, err := KeyFromBase64(encoded)
	require.NoError(t, err)
	assert.Equal(t, raw, key)
}

func TestKeyFromBase64InvalidEncoding(t *testing.T) {
	_, err := KeyFromBase64("not-valid-base64!!!")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "KeyFromBase64")
}

func TestKeyFromBase64WrongLength(t *testing.T) {
	short := base64.StdEncoding.EncodeToString(make([]byte, 16))
	_, err := KeyFromBase64(short)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "32 bytes")
}

func TestEncryptInvalidKey(t *testing.T) {
	_, err := Encrypt(make([]byte, 10), []byte("data"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "32 bytes")
}

func TestDecryptInvalidKey(t *testing.T) {
	_, err := Decrypt(make([]byte, 10), make([]byte, 50))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "32 bytes")
}
