package k8s

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gshepptech/mozza/internal/plan"
)

func TestParseImageRef(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		image    string
		registry string
		repo     string
		tag      string
	}{
		{
			name:     "library image no tag",
			image:    "nginx",
			registry: "registry-1.docker.io",
			repo:     "library/nginx",
			tag:      "latest",
		},
		{
			name:     "library image with tag",
			image:    "postgres:16",
			registry: "registry-1.docker.io",
			repo:     "library/postgres",
			tag:      "16",
		},
		{
			name:     "docker hub org image",
			image:    "myorg/api:v1.2.3",
			registry: "registry-1.docker.io",
			repo:     "myorg/api",
			tag:      "v1.2.3",
		},
		{
			name:     "custom registry",
			image:    "ghcr.io/myorg/api:latest",
			registry: "ghcr.io",
			repo:     "myorg/api",
			tag:      "latest",
		},
		{
			name:     "localhost registry with port",
			image:    "localhost:5000/myimage:dev",
			registry: "localhost:5000",
			repo:     "myimage",
			tag:      "dev",
		},
		{
			name:     "no tag defaults to latest",
			image:    "myorg/myimage",
			registry: "registry-1.docker.io",
			repo:     "myorg/myimage",
			tag:      "latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			registry, repo, tag := parseImageRef(tt.image)
			assert.Equal(t, tt.registry, registry)
			assert.Equal(t, tt.repo, repo)
			assert.Equal(t, tt.tag, tag)
		})
	}
}

func TestValidateImages(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/myorg/api/manifests/v1":
			w.WriteHeader(http.StatusOK)
		case "/v2/myorg/api/manifests/missing":
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	// Override the httpClient to use the test server's TLS client.
	origClient := httpClient
	httpClient = srv.Client()
	defer func() { httpClient = origClient }()

	// Extract host from test server (e.g., "127.0.0.1:port").
	host := srv.Listener.Addr().String()

	t.Run("all images valid", func(t *testing.T) {
		p := &plan.AppPlan{
			Name: "test-app",
			Slices: []plan.Slice{
				{Name: "api", Image: host + "/myorg/api:v1"},
			},
		}
		err := ValidateImages(context.Background(), p)
		require.NoError(t, err)
	})

	t.Run("missing image", func(t *testing.T) {
		p := &plan.AppPlan{
			Name: "test-app",
			Slices: []plan.Slice{
				{Name: "api", Image: host + "/myorg/api:missing"},
			},
		}
		err := ValidateImages(context.Background(), p)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "image validation failed")
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("skip slices without image", func(t *testing.T) {
		p := &plan.AppPlan{
			Name: "test-app",
			Slices: []plan.Slice{
				{Name: "db", Image: ""},
			},
		}
		err := ValidateImages(context.Background(), p)
		require.NoError(t, err)
	})
}
