package logger

import (
	"io"
	"log/slog"
	"os"
)

// New builds a slog.Logger that writes to os.Stdout.
// Format "json" produces machine-readable structured output; "text" is human-readable.
// Unknown values fall back to JSON at info level.
func New(format, level string) *slog.Logger {
	return NewWithWriter(os.Stdout, format, level)
}

// NewDual builds a slog.Logger that writes to both os.Stdout and a file.
// If logPath is empty or the file cannot be opened, falls back to stdout-only
// and the error is silently swallowed (logging must not block startup).
// Returns the logger and an optional io.Closer for the log file; the caller
// should call Close() on app shutdown.
func NewDual(format, level, logPath string) (*slog.Logger, io.Closer) {
	if logPath == "" {
		return New(format, level), io.NopCloser(nil)
	}
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		return New(format, level), io.NopCloser(nil)
	}
	w := io.MultiWriter(os.Stdout, f)
	return NewWithWriter(w, format, level), f
}

// NewWithWriter is used in tests to redirect log output.
func NewWithWriter(w io.Writer, format, level string) *slog.Logger {
	lvl := parseLevel(level)
	opts := &slog.HandlerOptions{Level: lvl}
	var h slog.Handler
	if format == "text" {
		h = slog.NewTextHandler(w, opts)
	} else {
		h = slog.NewJSONHandler(w, opts)
	}
	return slog.New(h)
}

func parseLevel(s string) slog.Level {
	switch s {
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
