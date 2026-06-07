package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/haithamEldesouky/ledgerline/internal/config"
)

// clearEnv resets every variable Load reads so each case starts from defaults.
func clearEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"HOST", "PORT", "APP_ENV", "LOG_LEVEL", "STORAGE_DRIVER",
		"DATABASE_URL", "RATE_LIMIT_RPS", "RATE_LIMIT_BURST", "SHUTDOWN_TIMEOUT",
	} {
		t.Setenv(k, "")
	}
}

func TestLoadDefaults(t *testing.T) {
	clearEnv(t)

	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, "0.0.0.0", cfg.Host)
	assert.Equal(t, 8080, cfg.Port)
	assert.Equal(t, "development", cfg.Env)
	assert.Equal(t, "memory", cfg.StorageDriver)
	assert.Equal(t, 100.0, cfg.RateLimitRPS)
}

func TestLoadPostgresRequiresURL(t *testing.T) {
	clearEnv(t)
	t.Setenv("STORAGE_DRIVER", "postgres")

	_, err := config.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "DATABASE_URL")
}

func TestLoadPostgresWithURL(t *testing.T) {
	clearEnv(t)
	t.Setenv("STORAGE_DRIVER", "postgres")
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/db")

	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, "postgres", cfg.StorageDriver)
}

func TestLoadValidationErrors(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
	}{
		{"bad port", map[string]string{"PORT": "abc"}},
		{"port out of range", map[string]string{"PORT": "70000"}},
		{"bad driver", map[string]string{"STORAGE_DRIVER": "redis"}},
		{"bad env", map[string]string{"APP_ENV": "staging"}},
		{"bad log level", map[string]string{"LOG_LEVEL": "verbose"}},
		{"bad rate limit", map[string]string{"RATE_LIMIT_RPS": "0"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clearEnv(t)
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			_, err := config.Load()
			assert.Error(t, err)
		})
	}
}
