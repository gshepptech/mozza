package gitdeploy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateSignature(t *testing.T) {
	secret := "test-secret-key"
	payload := []byte(`{"ref":"refs/heads/main","after":"abc123"}`)

	// Generate a valid signature.
	validSig := SignPayload(payload, secret)

	tests := []struct {
		name      string
		payload   []byte
		signature string
		secret    string
		want      bool
	}{
		{
			name:      "valid signature",
			payload:   payload,
			signature: validSig,
			secret:    secret,
			want:      true,
		},
		{
			name:      "invalid signature",
			payload:   payload,
			signature: "sha256=0000000000000000000000000000000000000000000000000000000000000000",
			secret:    secret,
			want:      false,
		},
		{
			name:      "wrong secret",
			payload:   payload,
			signature: validSig,
			secret:    "wrong-secret",
			want:      false,
		},
		{
			name:      "empty secret",
			payload:   payload,
			signature: validSig,
			secret:    "",
			want:      false,
		},
		{
			name:      "missing sha256 prefix",
			payload:   payload,
			signature: "abc123",
			secret:    secret,
			want:      false,
		},
		{
			name:      "tampered payload",
			payload:   []byte(`{"ref":"refs/heads/evil"}`),
			signature: validSig,
			secret:    secret,
			want:      false,
		},
		{
			name:      "invalid hex in signature",
			payload:   payload,
			signature: "sha256=zzzzzz",
			secret:    secret,
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidateSignature(tt.payload, tt.signature, tt.secret)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExtractBranch(t *testing.T) {
	tests := []struct {
		name string
		ref  string
		want string
	}{
		{
			name: "main branch",
			ref:  "refs/heads/main",
			want: "main",
		},
		{
			name: "feature branch",
			ref:  "refs/heads/feature/add-auth",
			want: "feature/add-auth",
		},
		{
			name: "tag ref returns empty",
			ref:  "refs/tags/v1.0.0",
			want: "",
		},
		{
			name: "empty ref",
			ref:  "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractBranch(tt.ref)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSignPayload(t *testing.T) {
	payload := []byte("test payload")
	secret := "my-secret"

	sig := SignPayload(payload, secret)

	assert.Contains(t, sig, "sha256=")
	assert.True(t, ValidateSignature(payload, sig, secret))
}
