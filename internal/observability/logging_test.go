package observability_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/haithamEldesouky/ledgerline/internal/observability"
)

func TestNewLogger(t *testing.T) {
	t.Parallel()

	for _, env := range []string{"development", "production", "test"} {
		for _, level := range []string{"debug", "info", "warn", "error", "unknown"} {
			logger := observability.NewLogger(env, level)
			require.NotNil(t, logger)
			// Must not panic when used.
			logger.Info("startup", "env", env, "level", level)
		}
	}
}
