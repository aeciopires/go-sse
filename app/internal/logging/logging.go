// Package logging builds the application's structured JSON logger.
//
// Every record is written as one JSON object per line to the given writer
// (stdout in production), which is the format expected by log collectors such
// as Grafana Alloy, Promtail and the OpenTelemetry Collector before shipping
// to Loki. When the context carries an OpenTelemetry span, the record gets
// trace_id and span_id attributes so logs can be correlated with traces.
package logging

import (
	"context"
	"io"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
)

// Options configures New.
type Options struct {
	// Level is the minimum level written to every handler.
	Level slog.Leveler
	// ServiceName and ServiceVersion are attached to every JSON record.
	ServiceName    string
	ServiceVersion string
	// Extra handlers (e.g. an OpenTelemetry log bridge) also receive records
	// at or above Level.
	Extra []slog.Handler
}

// New returns a logger that writes JSON records to w.
func New(w io.Writer, opts Options) *slog.Logger {
	level := opts.Level
	if level == nil {
		level = slog.LevelInfo
	}

	var jsonHandler slog.Handler = slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level:     level,
		AddSource: false,
	})
	if opts.ServiceName != "" {
		jsonHandler = jsonHandler.WithAttrs([]slog.Attr{
			slog.String("service", opts.ServiceName),
			slog.String("version", opts.ServiceVersion),
		})
	}

	handlers := []slog.Handler{traceHandler{jsonHandler}}
	for _, h := range opts.Extra {
		handlers = append(handlers, levelHandler{level: level, Handler: h})
	}
	if len(handlers) == 1 {
		return slog.New(handlers[0])
	}
	return slog.New(slog.NewMultiHandler(handlers...))
}

// traceHandler adds trace_id/span_id from the record's context.
type traceHandler struct{ slog.Handler }

func (h traceHandler) Handle(ctx context.Context, r slog.Record) error {
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		r.AddAttrs(
			slog.String("trace_id", sc.TraceID().String()),
			slog.String("span_id", sc.SpanID().String()),
		)
	}
	return h.Handler.Handle(ctx, r)
}

func (h traceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return traceHandler{h.Handler.WithAttrs(attrs)}
}

func (h traceHandler) WithGroup(name string) slog.Handler {
	return traceHandler{h.Handler.WithGroup(name)}
}

// levelHandler filters records below level before delegating.
type levelHandler struct {
	level slog.Leveler
	slog.Handler
}

func (h levelHandler) Enabled(ctx context.Context, l slog.Level) bool {
	return l >= h.level.Level() && h.Handler.Enabled(ctx, l)
}

func (h levelHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return levelHandler{h.level, h.Handler.WithAttrs(attrs)}
}

func (h levelHandler) WithGroup(name string) slog.Handler {
	return levelHandler{h.level, h.Handler.WithGroup(name)}
}
