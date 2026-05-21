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
	mu        sync.RWMutex
	logger    = slog.New(slog.NewTextHandler(io.Discard, nil))
	curFormat = FormatText
)

// Format selects the log output format.
type Format string

const (
	FormatText Format = "text"
	FormatJSON Format = "json"
)

// Init configures the package-level logger. dest defaults to os.Stderr when nil.
//
// Level policy:
//   - verbose=true:  Debug — every event is emitted, regardless of format.
//   - format=json:   Info — full structured stream for machine consumption.
//   - format=text:   Warn — drops INFO chatter so the CLI's human progress lines
//                    aren't duplicated by a parallel `time=... level=INFO msg=...`
//                    stream on stderr. Errors and warnings still surface.
//
// Safe to call multiple times.
func Init(format Format, verbose bool, dest io.Writer) {
	if dest == nil {
		dest = os.Stderr
	}
	var level slog.Level
	switch {
	case verbose:
		level = slog.LevelDebug
	case format == FormatJSON:
		level = slog.LevelInfo
	default: // text mode without verbose
		level = slog.LevelWarn
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
	curFormat = format
	if curFormat == "" {
		curFormat = FormatText
	}
	mu.Unlock()
}

// CurrentFormat returns the log format selected by the last Init call. Defaults
// to FormatText before Init runs. Useful for callers that want to suppress their
// own human-readable output when JSON logging is active.
func CurrentFormat() Format {
	mu.RLock()
	defer mu.RUnlock()
	return curFormat
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
