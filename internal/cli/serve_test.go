package cli

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsLoopback(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		host string
		want bool
	}{
		{name: "localhost", host: "localhost", want: true},
		{name: "ipv4 loopback", host: "127.0.0.1", want: true},
		{name: "ipv6 loopback", host: "::1", want: true},
		{name: "empty string", host: "", want: true},
		{name: "all interfaces", host: "0.0.0.0", want: false},
		{name: "private ip", host: "192.168.1.1", want: false},
		{name: "public ip", host: "8.8.8.8", want: false},
		{name: "hostname", host: "myhost.local", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := isLoopback(tt.host)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestResolveDSN(t *testing.T) {
	tests := []struct {
		name   string
		dbURL  string
		dbPath string
		envURL string
		want   string
	}{
		{
			name:  "explicit db-url wins",
			dbURL: "postgres://localhost/mozza",
			want:  "postgres://localhost/mozza",
		},
		{
			name:   "db-url wins over env",
			dbURL:  "postgres://flag",
			envURL: "postgres://env",
			want:   "postgres://flag",
		},
		{
			name:   "env wins over db flag",
			envURL: "postgres://env",
			dbPath: "/tmp/custom.db",
			want:   "postgres://env",
		},
		{
			name:   "db flag used when no url or env",
			dbPath: "/tmp/custom.db",
			want:   "/tmp/custom.db",
		},
		{
			name: "default when nothing set",
			want: "mozza.db",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set/clear the environment variable for each test case.
			if tt.envURL != "" {
				t.Setenv("MOZZA_DATABASE_URL", tt.envURL)
			} else {
				t.Setenv("MOZZA_DATABASE_URL", "")
			}

			got := resolveDSN(tt.dbURL, tt.dbPath)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestResolveDSN_EnvFallback(t *testing.T) {
	// Verify that MOZZA_DATABASE_URL is actually read from the environment.
	t.Setenv("MOZZA_DATABASE_URL", "postgres://from-env:5432/db")

	got := resolveDSN("", "")
	require.Equal(t, "postgres://from-env:5432/db", got)

	// Clear env and verify fallback to default.
	require.NoError(t, os.Unsetenv("MOZZA_DATABASE_URL"))
	got = resolveDSN("", "")
	// Should fall through to config or default; we just verify it doesn't panic.
	assert.NotEmpty(t, got)
}
