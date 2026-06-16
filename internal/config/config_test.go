package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gshepptech/mozza/internal/config"
)

func TestLoad_Defaults(t *testing.T) {
	t.Parallel()

	// Point at an empty dir so no config file is found.
	dir := t.TempDir()
	cfg, err := config.Load(config.WithConfigPath(dir))
	require.NoError(t, err)

	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, "app.mozza", cfg.RecipeFile)
	assert.Equal(t, "localhost", cfg.Server.Host)
	assert.Equal(t, config.DefaultServerPort, cfg.Server.Port)
	assert.Equal(t, "unix:///var/run/docker.sock", cfg.Docker.Host)
	assert.Equal(t, "mozza", cfg.Docker.Network)
}

func TestLoad_EnvOverride(t *testing.T) {
	// Cannot use t.Parallel here because subtests use t.Setenv.
	tests := []struct {
		name    string
		envKey  string
		envVal  string
		assertF func(t *testing.T, cfg *config.Config)
	}{
		{
			name:   "log level override",
			envKey: "MOZZA_LOG_LEVEL",
			envVal: "debug",
			assertF: func(t *testing.T, cfg *config.Config) {
				t.Helper()
				assert.Equal(t, "debug", cfg.LogLevel)
			},
		},
		{
			name:   "recipe file override",
			envKey: "MOZZA_RECIPE_FILE",
			envVal: "custom.mozza",
			assertF: func(t *testing.T, cfg *config.Config) {
				t.Helper()
				assert.Equal(t, "custom.mozza", cfg.RecipeFile)
			},
		},
		{
			name:   "server port override",
			envKey: "MOZZA_SERVER_PORT",
			envVal: "9090",
			assertF: func(t *testing.T, cfg *config.Config) {
				t.Helper()
				assert.Equal(t, 9090, cfg.Server.Port)
			},
		},
		{
			name:   "docker network override",
			envKey: "MOZZA_DOCKER_NETWORK",
			envVal: "custom-net",
			assertF: func(t *testing.T, cfg *config.Config) {
				t.Helper()
				assert.Equal(t, "custom-net", cfg.Docker.Network)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// t.Setenv is incompatible with t.Parallel.
			dir := t.TempDir()
			t.Setenv(tt.envKey, tt.envVal)

			cfg, err := config.Load(config.WithConfigPath(dir))
			require.NoError(t, err)
			tt.assertF(t, cfg)
		})
	}
}

func TestLoad_FileConfig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgContent := `
log_level: warn
recipe_file: custom.mozza
server:
  host: 0.0.0.0
  port: 3000
docker:
  host: tcp://localhost:2375
  network: test-net
`
	writeCfgFile(t, dir, cfgContent)

	cfg, err := config.Load(config.WithConfigPath(dir))
	require.NoError(t, err)

	assert.Equal(t, "warn", cfg.LogLevel)
	assert.Equal(t, "custom.mozza", cfg.RecipeFile)
	assert.Equal(t, "0.0.0.0", cfg.Server.Host)
	assert.Equal(t, 3000, cfg.Server.Port)
	assert.Equal(t, "tcp://localhost:2375", cfg.Docker.Host)
	assert.Equal(t, "test-net", cfg.Docker.Network)
}

func TestLoad_MissingFileUsesDefaults(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg, err := config.Load(config.WithConfigPath(dir))
	require.NoError(t, err)

	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, config.DefaultServerPort, cfg.Server.Port)
}

func TestLoad_PartialFileUsesDefaults(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgContent := `
log_level: error
`
	writeCfgFile(t, dir, cfgContent)

	cfg, err := config.Load(config.WithConfigPath(dir))
	require.NoError(t, err)

	assert.Equal(t, "error", cfg.LogLevel)
	assert.Equal(t, "app.mozza", cfg.RecipeFile)
	assert.Equal(t, "localhost", cfg.Server.Host)
	assert.Equal(t, config.DefaultServerPort, cfg.Server.Port)
}

// writeCfgFile writes a .mozza.yaml config file into dir.
func writeCfgFile(t *testing.T, dir, content string) {
	t.Helper()

	err := os.WriteFile(filepath.Join(dir, ".mozza.yaml"), []byte(content), 0o644)
	require.NoError(t, err)
}
