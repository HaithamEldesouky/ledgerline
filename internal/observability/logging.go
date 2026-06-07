// Package observability wires up structured logging and is the home for
// cross-cutting telemetry concerns.
package observability

import (
	"io"
	"log/slog"
	"os"
)

// NewLogger builds a structured slog.Logger.
//
//   - production: line-delimited JSON on stdout, ready for a log pipeline.
//   - development: human-readable text on stdout.
//   - test: output is discarded to keep test runs quiet.
func NewLogger(env, level string) *slog.Logger {
	var handler slog.Handler

	opts := &slog.HandlerOptions{Level: parseLevel(level)}

	switch env {
	case "test":
		handler = slog.NewTextHandler(io.Discard, opts)
	case "production":
		handler = slog.NewJSONHandler(os.Stdout, opts)
	default:
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	return slog.New(handler).With(slog.String("service", "ledgerline"))
}

func parseLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
