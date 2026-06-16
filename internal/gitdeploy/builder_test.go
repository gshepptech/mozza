package gitdeploy

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/gshepptech/mozza/internal/detect"
)

func TestExtractAppName(t *testing.T) {
	tests := []struct {
		name    string
		repoURL string
		want    string
	}{
		{
			name:    "standard github url",
			repoURL: "https://github.com/user/my-app",
			want:    "my-app",
		},
		{
			name:    "with .git suffix",
			repoURL: "https://github.com/user/my-app.git",
			want:    "my-app",
		},
		{
			name:    "with trailing slash",
			repoURL: "https://github.com/user/my-app/",
			want:    "my-app",
		},
		{
			name:    "uppercase is lowered",
			repoURL: "https://github.com/User/MyApp",
			want:    "myapp",
		},
		{
			name:    "special characters sanitized",
			repoURL: "https://github.com/user/my_app@v2",
			want:    "my-app-v2",
		},
		{
			name:    "empty url",
			repoURL: "",
			want:    "app",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractAppName(tt.repoURL)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGenerateMinimalDockerfile(t *testing.T) {
	t.Run("nil result uses alpine", func(t *testing.T) {
		df := generateMinimalDockerfile(nil)
		assert.Contains(t, df, "FROM alpine:3.19")
		assert.Contains(t, df, "EXPOSE 8080")
	})

	t.Run("with detection result", func(t *testing.T) {
		result := &detect.Result{
			BaseImage: "node:20-alpine",
			Port:      3000,
			BuildCmd:  "npm run build",
			StartCmd:  `["npm", "start"]`,
		}
		df := generateMinimalDockerfile(result)
		assert.Contains(t, df, "FROM node:20-alpine")
		assert.Contains(t, df, "RUN npm run build")
		assert.Contains(t, df, "EXPOSE 3000")
		assert.Contains(t, df, `CMD ["npm", "start"]`)
	})
}

func TestQueueLen(t *testing.T) {
	q := NewQueue(nil)
	assert.Equal(t, 0, q.Len())

	q.Enqueue(BuildJob{BuildID: 1, RepoURL: "test", CommitSHA: "abc", Branch: "main"})
	assert.Equal(t, 1, q.Len())
}
