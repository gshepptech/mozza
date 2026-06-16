package detect_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gshepptech/mozza/internal/detect"
	// Register all framework detectors.
	_ "github.com/gshepptech/mozza/internal/detect/frameworks"
)

func TestScan_InvalidDir(t *testing.T) {
	t.Parallel()

	_, err := detect.Scan("/nonexistent/path")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "detect.Scan")
}

func TestScan_NotADirectory(t *testing.T) {
	t.Parallel()

	f, err := os.CreateTemp("", "detect-test-*")
	require.NoError(t, err)
	defer os.Remove(f.Name())
	f.Close()

	_, err = detect.Scan(f.Name())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a directory")
}

func TestScan_EmptyDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	results, err := detect.Scan(dir)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestScanBest_EmptyDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	result, err := detect.ScanBest(dir)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestScan_NextJS(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		files      map[string]string
		confidence detect.Confidence
		wantPort   int
	}{
		{
			name: "nextjs with config (high confidence)",
			files: map[string]string{
				"package.json":   `{"dependencies": {"next": "14.0.0", "react": "18.0.0"}}`,
				"next.config.js": `module.exports = { output: 'standalone' }`,
			},
			confidence: detect.ConfidenceHigh,
			wantPort:   3000,
		},
		{
			name: "nextjs without config (medium confidence)",
			files: map[string]string{
				"package.json": `{"dependencies": {"next": "14.0.0"}}`,
			},
			confidence: detect.ConfidenceMedium,
			wantPort:   3000,
		},
		{
			name: "nextjs export mode",
			files: map[string]string{
				"package.json":   `{"dependencies": {"next": "14.0.0"}}`,
				"next.config.js": `module.exports = { output: 'export' }`,
			},
			confidence: detect.ConfidenceHigh,
			wantPort:   80,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			writeFiles(t, dir, tc.files)

			results, err := detect.Scan(dir)
			require.NoError(t, err)
			require.NotEmpty(t, results)

			r := findFramework(results, "nextjs")
			require.NotNil(t, r, "nextjs should be detected")
			assert.Equal(t, tc.confidence, r.Confidence)
			assert.Equal(t, tc.wantPort, r.Port)
			assert.Equal(t, "javascript", r.Language)
			assert.NotEmpty(t, r.Dockerfile)
		})
	}
}

func TestScan_Django(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		files      map[string]string
		confidence detect.Confidence
		wantServer string
	}{
		{
			name: "django with manage.py (high confidence)",
			files: map[string]string{
				"requirements.txt": "django==5.0\ngunicorn==21.2.0\n",
				"manage.py":        "#!/usr/bin/env python\nimport django\n",
			},
			confidence: detect.ConfidenceHigh,
			wantServer: "gunicorn",
		},
		{
			name: "django without manage.py (medium confidence)",
			files: map[string]string{
				"requirements.txt": "django==5.0\n",
			},
			confidence: detect.ConfidenceMedium,
			wantServer: "",
		},
		{
			name: "django with uvicorn",
			files: map[string]string{
				"requirements.txt": "django==5.0\nuvicorn==0.30.0\n",
				"manage.py":        "#!/usr/bin/env python\n",
			},
			confidence: detect.ConfidenceHigh,
			wantServer: "uvicorn",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			writeFiles(t, dir, tc.files)

			results, err := detect.Scan(dir)
			require.NoError(t, err)

			r := findFramework(results, "django")
			require.NotNil(t, r, "django should be detected")
			assert.Equal(t, tc.confidence, r.Confidence)
			assert.Equal(t, 8000, r.Port)
			assert.Equal(t, "python", r.Language)
			if tc.wantServer != "" {
				assert.Equal(t, tc.wantServer, r.Details["wsgi_server"])
			}
			assert.NotEmpty(t, r.Dockerfile)
		})
	}
}

func TestScan_Rails(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		files      map[string]string
		confidence detect.Confidence
	}{
		{
			name: "rails with config.ru (high confidence)",
			files: map[string]string{
				"Gemfile":   "source 'https://rubygems.org'\ngem 'rails', '~> 7.1'\n",
				"config.ru": "require_relative 'config/environment'\nrun Rails.application\n",
			},
			confidence: detect.ConfidenceHigh,
		},
		{
			name: "rails gemfile only (medium confidence)",
			files: map[string]string{
				"Gemfile": "source 'https://rubygems.org'\ngem 'rails'\n",
			},
			confidence: detect.ConfidenceMedium,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			writeFiles(t, dir, tc.files)

			results, err := detect.Scan(dir)
			require.NoError(t, err)

			r := findFramework(results, "rails")
			require.NotNil(t, r, "rails should be detected")
			assert.Equal(t, tc.confidence, r.Confidence)
			assert.Equal(t, 3000, r.Port)
			assert.Equal(t, "ruby", r.Language)
			assert.NotEmpty(t, r.Dockerfile)
		})
	}
}

func TestScan_Laravel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		files      map[string]string
		confidence detect.Confidence
		hasOctane  bool
	}{
		{
			name: "laravel with artisan (high confidence)",
			files: map[string]string{
				"composer.json": `{"require": {"laravel/framework": "^11.0"}}`,
				"artisan":       "#!/usr/bin/env php\n",
			},
			confidence: detect.ConfidenceHigh,
		},
		{
			name: "laravel with octane",
			files: map[string]string{
				"composer.json": `{"require": {"laravel/framework": "^11.0", "laravel/octane": "^2.0"}}`,
				"artisan":       "#!/usr/bin/env php\n",
			},
			confidence: detect.ConfidenceHigh,
			hasOctane:  true,
		},
		{
			name: "laravel without artisan (medium confidence)",
			files: map[string]string{
				"composer.json": `{"require": {"laravel/framework": "^11.0"}}`,
			},
			confidence: detect.ConfidenceMedium,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			writeFiles(t, dir, tc.files)

			results, err := detect.Scan(dir)
			require.NoError(t, err)

			r := findFramework(results, "laravel")
			require.NotNil(t, r, "laravel should be detected")
			assert.Equal(t, tc.confidence, r.Confidence)
			assert.Equal(t, 8000, r.Port)
			assert.Equal(t, "php", r.Language)
			if tc.hasOctane {
				assert.Equal(t, "enabled", r.Details["octane"])
			}
			assert.NotEmpty(t, r.Dockerfile)
		})
	}
}

func TestScan_Go(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		files   map[string]string
		wantCGO string
	}{
		{
			name: "go project without CGO",
			files: map[string]string{
				"go.mod":  "module example.com/myapp\n\ngo 1.24\n",
				"main.go": "package main\n\nfunc main() {}\n",
			},
			wantCGO: "disabled",
		},
		{
			name: "go project with CGO",
			files: map[string]string{
				"go.mod":  "module example.com/myapp\n\ngo 1.24\n",
				"main.go": "package main\n\nimport \"C\"\n\nfunc main() {}\n",
			},
			wantCGO: "enabled",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			writeFiles(t, dir, tc.files)

			results, err := detect.Scan(dir)
			require.NoError(t, err)

			r := findFramework(results, "go")
			require.NotNil(t, r, "go should be detected")
			assert.Equal(t, detect.ConfidenceHigh, r.Confidence)
			assert.Equal(t, 8080, r.Port)
			assert.Equal(t, "go", r.Language)
			assert.Equal(t, tc.wantCGO, r.Details["cgo"])
			assert.NotEmpty(t, r.Dockerfile)
		})
	}
}

func TestScan_SortsByConfidence(t *testing.T) {
	t.Parallel()

	// A project with both go.mod (high) and package.json with next (medium).
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"go.mod":       "module example.com/myapp\n\ngo 1.24\n",
		"main.go":      "package main\n\nfunc main() {}\n",
		"package.json": `{"dependencies": {"next": "14.0.0"}}`,
	})

	results, err := detect.Scan(dir)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(results), 2)

	// First result should be high confidence.
	assert.Equal(t, detect.ConfidenceHigh, results[0].Confidence)
}

func TestScan_NotDetectedWhenWrongDeps(t *testing.T) {
	t.Parallel()

	// package.json without next/react/etc.
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"package.json": `{"dependencies": {"express": "4.0.0"}}`,
	})

	results, err := detect.Scan(dir)
	require.NoError(t, err)

	// Should not detect nextjs.
	r := findFramework(results, "nextjs")
	assert.Nil(t, r)
}

func TestGenerateRecipe(t *testing.T) {
	t.Parallel()

	r := &detect.Result{
		Framework:  "go",
		Language:   "go",
		Confidence: detect.ConfidenceHigh,
		Port:       8080,
		HealthPath: "/healthz",
	}

	rec := detect.GenerateRecipe("myapp", r)
	assert.Equal(t, "myapp", rec.Name)
	require.Len(t, rec.Slices, 1)
	assert.Equal(t, 8080, rec.Slices[0].Port)
	assert.Equal(t, "/healthz", rec.Slices[0].Health)
	assert.Equal(t, "web", rec.Slices[0].Kind)
	assert.True(t, rec.Slices[0].Public)
}

func TestGenerateRecipeText(t *testing.T) {
	t.Parallel()

	r := &detect.Result{
		Framework:  "django",
		Confidence: detect.ConfidenceHigh,
		Port:       8000,
		HealthPath: "/health/",
	}

	text := detect.GenerateRecipeText("myapp", r)
	assert.Contains(t, text, "App: myapp")
	assert.Contains(t, text, "port 8000")
	assert.Contains(t, text, "/health/")
	assert.Contains(t, text, "django")
}

// writeFiles creates files in a directory from a map of relative path -> content.
func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(dir, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	}
}

// findFramework looks for a specific framework in a results slice.
func findFramework(results []detect.Result, name string) *detect.Result {
	for i := range results {
		if results[i].Framework == name {
			return &results[i]
		}
	}
	return nil
}
