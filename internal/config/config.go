// Package config provides Viper-based configuration loading for Mozza.
// It reads settings from file, environment variables, and built-in defaults.
package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// DefaultServerPort is the default HTTP server port.
const DefaultServerPort = 8080

// Config holds Mozza application configuration.
type Config struct {
	// LogLevel controls the logging verbosity (debug, info, warn, error).
	LogLevel string `mapstructure:"log_level"`
	// RecipeFile is the path to the .mozza recipe file.
	RecipeFile string `mapstructure:"recipe_file"`
	// Server holds HTTP server configuration.
	Server ServerConfig `mapstructure:"server"`
	// Docker holds local Docker runtime configuration.
	Docker DockerConfig `mapstructure:"docker"`
	// Database holds SQLite database configuration.
	Database DatabaseConfig `mapstructure:"database"`
}

// DatabaseConfig holds SQLite settings.
type DatabaseConfig struct {
	// Path is the file path for the SQLite database.
	Path string `mapstructure:"path"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	// Host is the address to bind the server to.
	Host string `mapstructure:"host"`
	// Port is the port number for the server.
	Port int `mapstructure:"port"`
}

// DockerConfig holds Docker runtime settings.
type DockerConfig struct {
	// Host is the Docker daemon socket address.
	Host string `mapstructure:"host"`
	// Network is the Docker network name for containers.
	Network string `mapstructure:"network"`
}

// Option configures Load behavior.
type Option func(*loadOptions)

type loadOptions struct {
	configPaths []string
}

// WithConfigPath adds a directory to search for configuration files.
// When set, the default search paths (current dir, $HOME/.mozza/) are replaced.
func WithConfigPath(path string) Option {
	return func(o *loadOptions) {
		o.configPaths = append(o.configPaths, path)
	}
}

// Load reads configuration from file, environment, and defaults.
// By default it searches for .mozza.yaml in the current directory and $HOME/.mozza/.
// Environment variables are prefixed with MOZZA_ and use underscores as
// separators (e.g., MOZZA_SERVER_PORT).
func Load(opts ...Option) (*Config, error) {
	var lo loadOptions
	for _, opt := range opts {
		opt(&lo)
	}

	v := viper.New()

	setDefaults(v)

	v.SetConfigName(".mozza")
	v.SetConfigType("yaml")

	if len(lo.configPaths) > 0 {
		for _, p := range lo.configPaths {
			v.AddConfigPath(p)
		}
	} else {
		v.AddConfigPath(".")
		v.AddConfigPath("$HOME/.mozza/")
	}

	v.SetEnvPrefix("MOZZA")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			return nil, fmt.Errorf("Load: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("Load: %w", err)
	}

	return &cfg, nil
}

// setDefaults registers all default configuration values.
func setDefaults(v *viper.Viper) {
	v.SetDefault("log_level", "info")
	v.SetDefault("recipe_file", "app.mozza")
	v.SetDefault("server.host", "localhost")
	v.SetDefault("server.port", DefaultServerPort)
	v.SetDefault("docker.host", "unix:///var/run/docker.sock")
	v.SetDefault("docker.network", "mozza")
	v.SetDefault("database.path", "mozza.db")
}
