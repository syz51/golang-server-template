package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
	"github.com/subosito/gotenv"
)

// Config contains the application's runtime configuration.
type Config struct {
	Server ServerConfig `mapstructure:"server"`
	Logger LoggerConfig `mapstructure:"logger"`
	App    AppConfig    `mapstructure:"app"`
}

type ServerConfig struct {
	Port               int           `mapstructure:"port"`
	Host               string        `mapstructure:"host"`
	MaxBodyBytes       int64         `mapstructure:"max_body_bytes"`
	ReadHeaderTimeout  time.Duration `mapstructure:"read_header_timeout"`
	ReadTimeout        time.Duration `mapstructure:"read_timeout"`
	WriteTimeout       time.Duration `mapstructure:"write_timeout"`
	IdleTimeout        time.Duration `mapstructure:"idle_timeout"`
	ShutdownTimeout    time.Duration `mapstructure:"shutdown_timeout"`
	CORSAllowedOrigins []string      `mapstructure:"cors_allowed_origins"`
}

type LoggerConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

type AppConfig struct {
	Name        string `mapstructure:"name"`
	Version     string `mapstructure:"version"`
	Environment string `mapstructure:"environment"`
}

// Load reads process environment > cwd/.env > config.yaml > defaults.
// Only cwd/config.yaml or cwd/configs/config.yaml is considered (in that order).
// Environment keys use APP_ followed by the upper-case dotted key with underscores;
// durations use Go duration strings and origin lists use comma-separated values.
// Each call is isolated and never changes the process environment.
func Load() (*Config, error) {
	v := viper.New()
	defaults := map[string]any{
		"server.port":                 8080,
		"server.host":                 "0.0.0.0",
		"server.max_body_bytes":       int64(1048576),
		"server.read_header_timeout":  5 * time.Second,
		"server.read_timeout":         30 * time.Second,
		"server.write_timeout":        30 * time.Second,
		"server.idle_timeout":         60 * time.Second,
		"server.shutdown_timeout":     30 * time.Second,
		"server.cors_allowed_origins": []string{},
		"logger.level":                "info",
		"logger.format":               "json",
		"app.name":                    "golang-server-template",
		"app.version":                 "1.0.0",
		"app.environment":             "development",
	}
	for key, value := range defaults {
		v.SetDefault(key, value)
	}
	for _, path := range []string{"config.yaml", filepath.Join("configs", "config.yaml")} {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return nil, fmt.Errorf("failed to inspect config file: %w", err)
		}
		v.SetConfigFile(path)
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		break
	}

	dotenv, err := gotenv.Read(".env")
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to read .env: %w", err)
	}
	for key := range defaults {
		envKey := "APP_" + strings.ToUpper(strings.ReplaceAll(key, ".", "_"))
		if value, ok := os.LookupEnv(envKey); ok {
			v.Set(key, value)
		} else if value, ok := dotenv[envKey]; ok {
			v.Set(key, value)
		}
	}
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}
	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}
	return &cfg, nil
}

func validate(cfg *Config) error {
	if cfg.Server.Port <= 0 || cfg.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", cfg.Server.Port)
	}
	if cfg.Server.MaxBodyBytes <= 0 {
		return fmt.Errorf("server max_body_bytes must be positive")
	}
	for name, value := range map[string]time.Duration{
		"read_header_timeout": cfg.Server.ReadHeaderTimeout,
		"read_timeout":        cfg.Server.ReadTimeout,
		"write_timeout":       cfg.Server.WriteTimeout,
		"idle_timeout":        cfg.Server.IdleTimeout,
		"shutdown_timeout":    cfg.Server.ShutdownTimeout,
	} {
		if value <= 0 {
			return fmt.Errorf("server %s must be positive", name)
		}
	}
	switch cfg.Logger.Level {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("invalid logger level: %s", cfg.Logger.Level)
	}
	switch cfg.Logger.Format {
	case "json", "text":
	default:
		return fmt.Errorf("invalid logger format: %s", cfg.Logger.Format)
	}
	if strings.TrimSpace(cfg.App.Name) == "" {
		return fmt.Errorf("app name cannot be empty")
	}
	switch cfg.App.Environment {
	case "development", "staging", "production":
	default:
		return fmt.Errorf("invalid environment: %s", cfg.App.Environment)
	}
	return nil
}
