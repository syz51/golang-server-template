package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func isolatedConfig(t *testing.T) {
	t.Helper()
	t.Chdir(t.TempDir())
	// Do not let the developer's APP_* settings affect configuration fixtures.
	for _, key := range []string{"SERVER_PORT", "SERVER_HOST", "SERVER_MAX_BODY_BYTES", "SERVER_READ_HEADER_TIMEOUT", "SERVER_READ_TIMEOUT", "SERVER_WRITE_TIMEOUT", "SERVER_IDLE_TIMEOUT", "SERVER_SHUTDOWN_TIMEOUT", "SERVER_CORS_ALLOWED_ORIGINS", "LOGGER_LEVEL", "LOGGER_FORMAT", "APP_NAME", "APP_VERSION", "APP_ENVIRONMENT"} {
		key = "APP_" + key
		old, present := os.LookupEnv(key)
		require.NoError(t, os.Unsetenv(key))
		t.Cleanup(func() {
			if present {
				_ = os.Setenv(key, old)
			} else {
				_ = os.Unsetenv(key)
			}
		})
	}
}

func writeConfigFile(t *testing.T, path, contents string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(contents), 0600))
}

func TestLoadDefaults(t *testing.T) {
	isolatedConfig(t)
	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, ServerConfig{Port: 8080, Host: "0.0.0.0", MaxBodyBytes: 1048576, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, ShutdownTimeout: 30 * time.Second, CORSAllowedOrigins: []string{}}, cfg.Server)
	require.Equal(t, LoggerConfig{Level: "info", Format: "json"}, cfg.Logger)
	require.Equal(t, AppConfig{Name: "golang-server-template", Version: "1.0.0", Environment: "development"}, cfg.App)
}

func TestShippedConfigurationMatchesDefaults(t *testing.T) {
	yaml, err := os.ReadFile("../../configs/config.yaml")
	require.NoError(t, err)
	dotenv, err := os.ReadFile("../../.env.example")
	require.NoError(t, err)
	isolatedConfig(t)
	defaults, err := Load()
	require.NoError(t, err)
	require.NoError(t, os.Mkdir("configs", 0700))
	writeConfigFile(t, "configs/config.yaml", string(yaml))
	fromYAML, err := Load()
	require.NoError(t, err)
	require.Equal(t, defaults, fromYAML)
	writeConfigFile(t, ".env", string(dotenv))
	fromDotenv, err := Load()
	require.NoError(t, err)
	require.Equal(t, defaults, fromDotenv)
	require.Empty(t, fromDotenv.Server.CORSAllowedOrigins)
}

func TestLoadPrecedenceAndIsolation(t *testing.T) {
	isolatedConfig(t)
	require.NoError(t, os.Mkdir("configs", 0700))
	writeConfigFile(t, "configs/config.yaml", "server:\n  port: 8101\napp:\n  name: nested\n")
	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, 8101, cfg.Server.Port)
	writeConfigFile(t, "config.yaml", "server:\n  port: 8102\n  read_timeout: 12s\n  cors_allowed_origins: [https://file.example]\napp:\n  name: file\nlogger:\n  format: text\n")
	cfg, err = Load()
	require.NoError(t, err)
	require.Equal(t, 8102, cfg.Server.Port)
	require.Equal(t, 12*time.Second, cfg.Server.ReadTimeout)
	require.Equal(t, []string{"https://file.example"}, cfg.Server.CORSAllowedOrigins)
	writeConfigFile(t, ".env", "APP_SERVER_PORT=8103\nAPP_APP_NAME=dotenv\nAPP_SERVER_CORS_ALLOWED_ORIGINS=https://one.example,https://two.example\nAPP_SERVER_READ_TIMEOUT=13s\n")
	cfg, err = Load()
	require.NoError(t, err)
	require.Equal(t, 8103, cfg.Server.Port)
	require.Equal(t, "dotenv", cfg.App.Name)
	require.Equal(t, 13*time.Second, cfg.Server.ReadTimeout)
	require.Equal(t, []string{"https://one.example", "https://two.example"}, cfg.Server.CORSAllowedOrigins)
	_, exists := os.LookupEnv("APP_APP_NAME")
	require.False(t, exists, ".env must not mutate process environment")
	t.Setenv("APP_SERVER_PORT", "8104")
	t.Setenv("APP_SERVER_READ_TIMEOUT", "14s")
	cfg, err = Load()
	require.NoError(t, err)
	require.Equal(t, 8104, cfg.Server.Port)
	require.Equal(t, 14*time.Second, cfg.Server.ReadTimeout)
	require.NoError(t, os.Unsetenv("APP_SERVER_PORT"))
	require.NoError(t, os.Unsetenv("APP_SERVER_READ_TIMEOUT"))
	require.NoError(t, os.Remove(".env"))
	require.NoError(t, os.Remove("config.yaml"))
	require.NoError(t, os.Remove("configs/config.yaml"))
	home := t.TempDir()
	t.Setenv("HOME", home)
	require.NoError(t, os.Mkdir(filepath.Join(home, ".config"), 0700))
	writeConfigFile(t, filepath.Join(home, ".config/config.yaml"), "server:\n  port: 9999\n")
	cfg, err = Load()
	require.NoError(t, err)
	require.Equal(t, 8080, cfg.Server.Port)
	require.Equal(t, "golang-server-template", cfg.App.Name)
	require.Equal(t, "json", cfg.Logger.Format)
}

func TestLoadRejectsMalformedFiles(t *testing.T) {
	for _, file := range []string{"config.yaml", ".env"} {
		t.Run(file, func(t *testing.T) {
			isolatedConfig(t)
			writeConfigFile(t, file, "this is not valid: [\n")
			_, err := Load()
			require.Error(t, err)
		})
	}
}

func TestLoadRejectsInvalidSettings(t *testing.T) {
	for key, values := range map[string][]string{
		"SERVER_PORT":                {"0", "65536", "nope"},
		"SERVER_MAX_BODY_BYTES":      {"0", "-1", "nope"},
		"SERVER_READ_HEADER_TIMEOUT": {"0s", "-1s", "nope"},
		"SERVER_READ_TIMEOUT":        {"0s", "-1s", "nope"},
		"SERVER_WRITE_TIMEOUT":       {"0s", "-1s", "nope"},
		"SERVER_IDLE_TIMEOUT":        {"0s", "-1s", "nope"},
		"SERVER_SHUTDOWN_TIMEOUT":    {"0s", "-1s", "nope"},
		"LOGGER_LEVEL":               {"trace", "INFO", ""},
		"LOGGER_FORMAT":              {"xml", ""},
		"APP_NAME":                   {"", "   "},
		"APP_ENVIRONMENT":            {"test", ""},
	} {
		for _, value := range values {
			t.Run(key+"/"+value, func(t *testing.T) {
				isolatedConfig(t)
				t.Setenv("APP_"+key, value)
				_, err := Load()
				require.Error(t, err)
			})
		}
	}
}
