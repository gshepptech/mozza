package version

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVersionVarsHaveDefaults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		value    string
		wantNon  string
		wantType string
	}{
		{name: "Version has default", value: Version, wantNon: "", wantType: "string"},
		{name: "Commit has default", value: Commit, wantNon: "", wantType: "string"},
		{name: "Date has default", value: Date, wantNon: "", wantType: "string"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.NotEmpty(t, tt.value, "version variable should have a default value")
			// The default values set in version.go are "dev", "unknown", "unknown".
			assert.IsType(t, "", tt.value, "version variable should be a string")
		})
	}
}

func TestVersionDefaults(t *testing.T) {
	t.Parallel()

	// Verify the specific defaults set in version.go (before ldflags override).
	assert.Equal(t, "dev", Version)
	assert.Equal(t, "unknown", Commit)
	assert.Equal(t, "unknown", Date)
}
