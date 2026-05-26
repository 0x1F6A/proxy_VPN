// Package logger wraps log/slog with sane production defaults so the rest
// of the codebase depends on a stable interface.
package logger

import (
	"log/slog"
	"os"
	"strings"
)

type Logger = slog.Logger

// New returns a slog logger configured with the given level and format
// (json|text). Unknown levels default to info; unknown formats default to
// json.
func New(level, format string) *Logger {
	lvl := parseLevel(level)
	opts := &slog.HandlerOptions{Level: lvl, AddSource: lvl == slog.LevelDebug}

	var handler slog.Handler
	switch strings.ToLower(format) {
	case "text":
		handler = slog.NewTextHandler(os.Stdout, opts)
	default:
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}
	return slog.New(handler)
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
