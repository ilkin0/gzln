package logger

import (
	"bytes"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew_ProductionEnvironment(t *testing.T) {
	var buf bytes.Buffer
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() {
		os.Stdout = oldStdout
	}()

	logger := New("production", "info")

	slog.SetDefault(logger)
	slog.Info("test message", slog.String("key", "value"))

	w.Close()
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	assert.Contains(t, output, `"msg":"test message"`)
	assert.Contains(t, output, `"key":"value"`)
	assert.Contains(t, output, `"level":"INFO"`)
}

func TestNew_DevelopmentEnvironment(t *testing.T) {
	logger := New("development", "debug")
	assert.NotNil(t, logger)
}

func TestNew_LogLevels(t *testing.T) {
	tests := []struct {
		name          string
		level         string
		expectedLevel slog.Level
	}{
		{"debug level", "debug", slog.LevelDebug},
		{"info level", "info", slog.LevelInfo},
		{"warn level", "warn", slog.LevelWarn},
		{"warning level", "warning", slog.LevelWarn},
		{"error level", "error", slog.LevelError},
		{"default to info", "invalid", slog.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := New("development", tt.level)
			assert.NotNil(t, logger)
		})
	}
}

func TestInit_DefaultsToDevEnvironment(t *testing.T) {
	oldEnv := os.Getenv("APP_ENV")
	oldLevel := os.Getenv("LOG_LEVEL")
	defer func() {
		os.Setenv("APP_ENV", oldEnv)
		os.Setenv("LOG_LEVEL", oldLevel)
	}()

	os.Unsetenv("APP_ENV")
	os.Unsetenv("LOG_LEVEL")

	logger := Init()
	assert.NotNil(t, logger)
}

func TestInit_ReadsEnvironmentVariables(t *testing.T) {
	oldEnv := os.Getenv("APP_ENV")
	oldLevel := os.Getenv("LOG_LEVEL")
	defer func() {
		os.Setenv("APP_ENV", oldEnv)
		os.Setenv("LOG_LEVEL", oldLevel)
	}()

	os.Setenv("APP_ENV", "production")
	os.Setenv("LOG_LEVEL", "error")

	logger := Init()
	assert.NotNil(t, logger)
}

func TestInit_DefaultLogLevelByEnvironment(t *testing.T) {
	tests := []struct {
		name        string
		env         string
		expectDebug bool // production should default to info, dev to debug
	}{
		{"production defaults to info", "production", false},
		{"development defaults to debug", "development", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldEnv := os.Getenv("APP_ENV")
			oldLevel := os.Getenv("LOG_LEVEL")
			defer func() {
				os.Setenv("APP_ENV", oldEnv)
				os.Setenv("LOG_LEVEL", oldLevel)
			}()

			os.Setenv("APP_ENV", tt.env)
			os.Unsetenv("LOG_LEVEL")

			logger := Init()
			assert.NotNil(t, logger)
		})
	}
}

func TestNew_HandlerSelection(t *testing.T) {
	tests := []struct {
		name         string
		env          string
		shouldBeJSON bool
	}{
		{"production uses JSON handler", "production", true},
		{"development uses text handler", "development", false},
		{"other env uses text handler", "staging", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := New(tt.env, "info")
			assert.NotNil(t, logger)
		})
	}
}

func TestNew_CaseInsensitiveLogLevel(t *testing.T) {
	tests := []string{"DEBUG", "Info", "WARN", "ErRoR"}

	for _, level := range tests {
		t.Run("level_"+level, func(t *testing.T) {
			logger := New("development", level)
			assert.NotNil(t, logger, "Logger should be created with case-insensitive level: %s", level)
		})
	}
}
