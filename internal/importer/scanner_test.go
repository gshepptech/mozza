package importer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gshepptech/mozza/internal/recipe"
)

// mustParseRecipe ensures the generated recipe is valid .mozza syntax.
func mustParseRecipe(t *testing.T, source string) {
	t.Helper()
	p := recipe.NewParser(source)
	r, err := p.Parse()
	require.NoError(t, err, "generated recipe should parse without error:\n%s", source)
	require.NotNil(t, r, "parsed recipe should not be nil")
	assert.NotEmpty(t, r.Name, "recipe should have an app name")
}

// --- ParseGitHubURL ---

func TestParseGitHubURL(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		wantOwner string
		wantRepo  string
	}{
		{
			name:      "full HTTPS URL",
			url:       "https://github.com/hobbyfarm/gargantua",
			wantOwner: "hobbyfarm",
			wantRepo:  "gargantua",
		},
		{
			name:      "URL with .git suffix",
			url:       "https://github.com/hobbyfarm/gargantua.git",
			wantOwner: "hobbyfarm",
			wantRepo:  "gargantua",
		},
		{
			name:      "bare github.com path",
			url:       "github.com/owner/repo",
			wantOwner: "owner",
			wantRepo:  "repo",
		},
		{
			name:      "URL with tree/main path",
			url:       "https://github.com/owner/repo/tree/main",
			wantOwner: "owner",
			wantRepo:  "repo",
		},
		{
			name:      "URL with tree/main and subpath",
			url:       "https://github.com/owner/repo/tree/main/src",
			wantOwner: "owner",
			wantRepo:  "repo",
		},
		{
			name:      "URL with trailing slash",
			url:       "https://github.com/owner/repo/",
			wantOwner: "owner",
			wantRepo:  "repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, repo, err := ParseGitHubURL(tt.url)
			require.NoError(t, err)
			assert.Equal(t, tt.wantOwner, owner)
			assert.Equal(t, tt.wantRepo, repo)
		})
	}
}

func TestParseGitHubURL_Invalid(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{name: "empty URL", url: ""},
		{name: "not GitHub", url: "https://gitlab.com/owner/repo"},
		{name: "no repo path", url: "https://github.com/owner"},
		{name: "just github.com", url: "https://github.com/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := ParseGitHubURL(tt.url)
			assert.Error(t, err)
		})
	}
}

// --- ComposeToRecipe ---

func TestComposeToRecipe_Basic(t *testing.T) {
	compose := `
services:
  wordpress:
    image: wordpress:6-apache
    ports:
      - "8080:80"
    depends_on:
      - db
    environment:
      WORDPRESS_DB_HOST: "db:3306"
  db:
    image: mysql:8
    environment:
      MYSQL_ROOT_PASSWORD: "secret"
`
	result, err := ComposeToRecipe(compose)
	require.NoError(t, err)

	assert.Contains(t, result, "App:")
	assert.Contains(t, result, "Wordpress:")
	assert.Contains(t, result, "from image wordpress:6-apache")
	assert.Contains(t, result, "open to the public on port 80")
	assert.Contains(t, result, "needs db")
	assert.Contains(t, result, "Db:")
	assert.Contains(t, result, "mysql 8")

	mustParseRecipe(t, result)
}

func TestComposeToRecipe_EngineShorthand(t *testing.T) {
	compose := `
services:
  db:
    image: postgres:16-alpine
    environment:
      POSTGRES_PASSWORD: "secret"
`
	result, err := ComposeToRecipe(compose)
	require.NoError(t, err)

	assert.Contains(t, result, "postgres 16, 10Gi")
	assert.NotContains(t, result, "from image")

	mustParseRecipe(t, result)
}

func TestComposeToRecipe_Ports(t *testing.T) {
	compose := `
services:
  web:
    image: nginx:alpine
    ports:
      - "8080:80"
      - "443"
`
	result, err := ComposeToRecipe(compose)
	require.NoError(t, err)

	assert.Contains(t, result, "open to the public on port 80")
	assert.Contains(t, result, "on port 443")

	mustParseRecipe(t, result)
}

func TestComposeToRecipe_DependsOn(t *testing.T) {
	compose := `
services:
  web:
    image: myapp:latest
    depends_on:
      - db
      - cache
  db:
    image: postgres:16
  cache:
    image: redis:7
`
	result, err := ComposeToRecipe(compose)
	require.NoError(t, err)

	assert.Contains(t, result, "needs db and cache")

	mustParseRecipe(t, result)
}

func TestComposeToRecipe_DependsOnMap(t *testing.T) {
	compose := `
services:
  web:
    image: myapp:latest
    depends_on:
      db:
        condition: service_healthy
  db:
    image: postgres:16
`
	result, err := ComposeToRecipe(compose)
	require.NoError(t, err)

	assert.Contains(t, result, "needs db")

	mustParseRecipe(t, result)
}

func TestComposeToRecipe_EnvList(t *testing.T) {
	compose := `
services:
  web:
    image: myapp:latest
    environment:
      - "DB_HOST=localhost"
      - "DB_PORT=5432"
`
	result, err := ComposeToRecipe(compose)
	require.NoError(t, err)

	assert.Contains(t, result, `set DB_HOST to "localhost"`)
	assert.Contains(t, result, `set DB_PORT to "5432"`)

	mustParseRecipe(t, result)
}

func TestComposeToRecipe_Replicas(t *testing.T) {
	compose := `
services:
  web:
    image: nginx:alpine
    deploy:
      replicas: 3
`
	result, err := ComposeToRecipe(compose)
	require.NoError(t, err)

	assert.Contains(t, result, "run 3 copies")

	mustParseRecipe(t, result)
}

func TestComposeToRecipe_RestartPolicy(t *testing.T) {
	compose := `
services:
  web:
    image: nginx:alpine
    restart: always
`
	result, err := ComposeToRecipe(compose)
	require.NoError(t, err)

	assert.Contains(t, result, "restart always")

	mustParseRecipe(t, result)
}

func TestComposeToRecipe_Redis(t *testing.T) {
	compose := `
services:
  cache:
    image: redis:7-alpine
`
	result, err := ComposeToRecipe(compose)
	require.NoError(t, err)

	assert.Contains(t, result, "redis 7")
	assert.NotContains(t, result, "from image")

	mustParseRecipe(t, result)
}

func TestComposeToRecipe_NoServices(t *testing.T) {
	_, err := ComposeToRecipe("version: '3'\n")
	assert.Error(t, err)
}

// --- DockerfileToRecipe ---

func TestDockerfileToRecipe(t *testing.T) {
	dockerfile := `FROM golang:1.22 AS builder
WORKDIR /app
COPY . .
RUN go build -o server .

FROM alpine:3.19
COPY --from=builder /app/server /server
EXPOSE 3000
CMD ["/server"]
`
	result, err := DockerfileToRecipe(dockerfile, "myapp")
	require.NoError(t, err)

	assert.Contains(t, result, "App: myapp")
	assert.Contains(t, result, "open to the public on port 3000")
	assert.Contains(t, result, "health check /healthz")

	mustParseRecipe(t, result)
}

func TestDockerfileToRecipe_NoExpose(t *testing.T) {
	dockerfile := `FROM alpine:3.19
CMD ["/server"]
`
	result, err := DockerfileToRecipe(dockerfile, "myapp")
	require.NoError(t, err)

	assert.Contains(t, result, "open to the public on port 8080")

	mustParseRecipe(t, result)
}

func TestDockerfileToRecipe_Empty(t *testing.T) {
	_, err := DockerfileToRecipe("", "myapp")
	assert.Error(t, err)
}

// --- HelmToRecipe ---

func TestHelmToRecipe_Basic(t *testing.T) {
	values := `
image:
  repository: myorg/myapp
  tag: "1.0.0"
replicaCount: 3
service:
  port: 8080
`
	result, err := HelmToRecipe(values, "myapp")
	require.NoError(t, err)

	assert.Contains(t, result, "App: myapp")
	assert.Contains(t, result, "from image myorg/myapp:1.0.0")
	assert.Contains(t, result, "on port 8080")
	assert.Contains(t, result, "run 3 copies")

	mustParseRecipe(t, result)
}

func TestHelmToRecipe_WithIngress(t *testing.T) {
	values := `
image:
  repository: myorg/myapp
  tag: "2.0"
service:
  port: 80
ingress:
  enabled: true
`
	result, err := HelmToRecipe(values, "webapp")
	require.NoError(t, err)

	assert.Contains(t, result, "open to the public on port 80")

	mustParseRecipe(t, result)
}

func TestHelmToRecipe_NoImage(t *testing.T) {
	values := `
replicaCount: 1
`
	result, err := HelmToRecipe(values, "unknown")
	require.NoError(t, err)

	// Should fall back to chart name.
	assert.Contains(t, result, "from image unknown:latest")

	mustParseRecipe(t, result)
}

// --- detectSources ---

func TestDetectSources(t *testing.T) {
	files := []string{
		"README.md",
		"docker-compose.yml",
		"Dockerfile",
		"charts/",
		"src/",
	}

	result := &ScanResult{}
	detectSources(result, files)

	require.Len(t, result.Sources, 3)

	// Verify priorities.
	var composeFound, helmFound, dockerfileFound bool
	for _, s := range result.Sources {
		switch s.Type {
		case "compose":
			composeFound = true
			assert.Equal(t, 1, s.Priority)
		case "helm":
			helmFound = true
			assert.Equal(t, 2, s.Priority)
		case "dockerfile":
			dockerfileFound = true
			assert.Equal(t, 4, s.Priority)
		}
	}
	assert.True(t, composeFound, "should detect compose")
	assert.True(t, helmFound, "should detect helm")
	assert.True(t, dockerfileFound, "should detect dockerfile")
}

func TestDetectSources_K8sManifests(t *testing.T) {
	files := []string{"manifests/", "README.md"}
	result := &ScanResult{}
	detectSources(result, files)

	require.Len(t, result.Sources, 1)
	assert.Equal(t, "k8s-manifests", result.Sources[0].Type)
	assert.Equal(t, 3, result.Sources[0].Priority)
}

func TestDetectSources_NothingDeployable(t *testing.T) {
	files := []string{"README.md", "main.go", "go.mod"}
	result := &ScanResult{}
	detectSources(result, files)

	assert.Empty(t, result.Sources)
}

// --- parseEngineImage ---

func TestParseEngineImage(t *testing.T) {
	tests := []struct {
		image       string
		wantEngine  string
		wantVersion string
	}{
		{"postgres:16", "postgres", "16"},
		{"postgres:16-alpine", "postgres", "16"},
		{"redis:7-alpine", "redis", "7"},
		{"mysql:8", "mysql", "8"},
		{"mariadb:11", "mysql", "11"},
		{"nginx:alpine", "", ""},
		{"myorg/myapp:1.0", "", ""},
		{"postgres:latest", "postgres", "16"},
	}

	for _, tt := range tests {
		t.Run(tt.image, func(t *testing.T) {
			engine, version := parseEngineImage(tt.image)
			assert.Equal(t, tt.wantEngine, engine)
			if tt.wantEngine != "" {
				assert.Equal(t, tt.wantVersion, version)
			}
		})
	}
}
