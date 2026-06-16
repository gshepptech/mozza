package gitdeploy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeRepoURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "strips .git suffix",
			url:  "https://github.com/user/repo.git",
			want: "https://github.com/user/repo",
		},
		{
			name: "strips trailing slash",
			url:  "https://github.com/user/repo/",
			want: "https://github.com/user/repo",
		},
		{
			name: "no change needed",
			url:  "https://github.com/user/repo",
			want: "https://github.com/user/repo",
		},
		{
			name: "strips both .git and slash",
			url:  "https://github.com/user/repo.git/",
			want: "https://github.com/user/repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeRepoURL(tt.url)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGenerateWebhookSecret(t *testing.T) {
	secret1, err := generateWebhookSecret()
	require.NoError(t, err)
	assert.Len(t, secret1, 64) // 32 bytes = 64 hex chars

	secret2, err := generateWebhookSecret()
	require.NoError(t, err)
	assert.NotEqual(t, secret1, secret2, "secrets should be unique")
}

func TestMinInt(t *testing.T) {
	assert.Equal(t, 3, minInt(3, 5))
	assert.Equal(t, 3, minInt(5, 3))
	assert.Equal(t, 3, minInt(3, 3))
}
