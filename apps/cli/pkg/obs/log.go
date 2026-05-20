// Package obs (observability) provides a thin wrapper around log/slog so the
// CLI can switch between human-readable text logs (default) and structured
// JSON logs (--log-format=json) without each pkg site needing to know which is
// active. All non-CLI code should call obs.Logger() or the package-level
// helpers; programmatic callers that don't init get a discard logger.
package obs

import (
	"context"
	"io"
	"log/slog"
	"os"
	"sync"
)

var (
	mu     sync.RWMutex
	logger = slog.New(slog.NewTextHandler(io.Discard, nil))
)

// Format selects the log output format.
type Format string

const (
	FormatText Format = "text"
	FormatJSON Format = "json"
)

// Init configures the package-level logger. dest defaults to os.Stderr when nil.
// verbose=true sets the level to Debug; otherwise Info. Safe to call multiple times.
func Init(format Format, verbose bool, dest io.Writer) {
	if dest == nil {
		dest = os.Stderr
	}
	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}
	opts := &slog.HandlerOptions{Level: level}
	var h slog.Handler
	switch format {
	case FormatJSON:
		h = slog.NewJSONHandler(dest, opts)
	default:
		h = slog.NewTextHandler(dest, opts)
	}
	mu.Lock()
	logger = slog.New(h)
	mu.Unlock()
}

// Logger returns the configured logger. Always non-nil.
func Logger() *slog.Logger {
	mu.RLock()
	defer mu.RUnlock()
	return logger
}

// Helper wrappers that read the live logger each call so tests can swap it.

func Info(msg string, args ...any)  { Logger().Info(msg, args...) }
func Warn(msg string, args ...any)  { Logger().Warn(msg, args...) }
func Error(msg string, args ...any) { Logger().Error(msg, args...) }
func Debug(msg string, args ...any) { Logger().Debug(msg, args...) }

// InfoCtx / WarnCtx / etc. pass a context so handlers can extract trace IDs.
func InfoCtx(ctx context.Context, msg string, args ...any)  { Logger().InfoContext(ctx, msg, args...) }
func WarnCtx(ctx context.Context, msg string, args ...any)  { Logger().WarnContext(ctx, msg, args...) }
func ErrorCtx(ctx context.Context, msg string, args ...any) { Logger().ErrorContext(ctx, msg, args...) }
func DebugCtx(ctx context.Context, msg string, args ...any) { Logger().DebugContext(ctx, msg, args...) }
