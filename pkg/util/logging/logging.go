package logging

import (
	"context"
	"log/slog"
	"os"
)

// Standard attribute keys
const (
	AttrComponent = "component"
	AttrTraceID   = "trace_id"
	AttrGroveID   = "grove_id"
	AttrAgentID   = "agent_id"
	AttrRequestID = "request_id"
	AttrUserID    = "user_id"
)

// Setup initializes the global logger.
// component is the name of the service (e.g., "hub", "runtimehost").
// debug enables DEBUG level logging.
// useGCP formats logs for Google Cloud Logging.
func Setup(component string, debug bool, useGCP bool) {
	level := slog.LevelInfo
	if debug || os.Getenv("SCION_LOG_LEVEL") == "debug" {
		level = slog.LevelDebug
	}

	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level: level,
	}

	if useGCP {
		handler = NewGCPHandler(os.Stdout, opts, component)
	} else {
		// Default to JSON handler for structured logging
		opts.ReplaceAttr = func(groups []string, a slog.Attr) slog.Attr {
			// Ensure component is always present if not already
			return a
		}
		handler = slog.NewJSONHandler(os.Stdout, opts).WithAttrs([]slog.Attr{
			slog.String(AttrComponent, component),
		})
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)
}

// WithMetadata returns a context with the provided metadata attached as slog attributes.
func WithMetadata(ctx context.Context, attrs ...slog.Attr) context.Context {
	// This is a placeholder for context-based logging if needed.
	// For now, we can just use slog.With() on the logger.
	return ctx
}

// Logger returns a logger with contextual metadata from the context.
// Currently it just returns the default logger.
func Logger(ctx context.Context) *slog.Logger {
	return slog.Default()
}

// Handler with component name
type componentHandler struct {
	slog.Handler
	component string
}

func (h *componentHandler) Handle(ctx context.Context, r slog.Record) error {
	r.AddAttrs(slog.String(AttrComponent, h.component))
	return h.Handler.Handle(ctx, r)
}
