// Package server implements the go-sse HTTP server: frontend, SSE stream,
// health checks and Prometheus metrics.
package server

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/trace"

	"github.com/aeciopires/go-sse/app/internal/config"
	"github.com/aeciopires/go-sse/app/internal/metrics"
	"github.com/aeciopires/go-sse/app/web"
)

// HTTP server timeouts. WriteTimeout is lifted per connection for /sse.
const (
	readHeaderTimeout = 10 * time.Second
	readTimeout       = 30 * time.Second
	writeTimeout      = 30 * time.Second
	idleTimeout       = 120 * time.Second
)

// Options are the dependencies of a Server.
type Options struct {
	Config         config.Config
	Logger         *slog.Logger
	Metrics        *metrics.Metrics
	TracerProvider trace.TracerProvider
}

// Server is the go-sse HTTP server.
type Server struct {
	cfg     config.Config
	log     *slog.Logger
	metrics *metrics.Metrics
	tp      trace.TracerProvider
	tracer  trace.Tracer
	tmpl    *template.Template
	handler http.Handler
	started time.Time

	ready  atomic.Bool  // reported by /readyz
	active atomic.Int64 // open SSE streams

	// streamsCtx is canceled when shutdown starts, ending every SSE stream
	// (http.Server.Shutdown does not cancel in-flight request contexts).
	streamsCtx  context.Context
	stopStreams context.CancelFunc
}

// New builds a Server and its routes.
func New(opts Options) (*Server, error) {
	tmpl, err := template.ParseFS(web.Templates(), "index.html")
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}
	s := &Server{
		cfg:     opts.Config,
		log:     opts.Logger,
		metrics: opts.Metrics,
		tp:      opts.TracerProvider,
		tracer:  opts.TracerProvider.Tracer("github.com/aeciopires/go-sse/app/internal/server"),
		tmpl:    tmpl,
		started: time.Now(),
	}
	s.streamsCtx, s.stopStreams = context.WithCancel(context.Background())
	s.handler = s.routes()
	return s, nil
}

// Handler returns the root HTTP handler.
func (s *Server) Handler() http.Handler { return s.handler }

// SetReady changes the readiness reported by /readyz.
func (s *Server) SetReady(ready bool) { s.ready.Store(ready) }

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	// "{$}" matches only "/" exactly; unknown paths get a 404 instead of the
	// index page (which also avoids unbounded metric label cardinality).
	s.handle(mux, "GET /{$}", "/", true, http.HandlerFunc(s.handleIndex))
	s.handle(mux, "GET /static/", "/static", false,
		http.StripPrefix("/static/", http.FileServerFS(web.Static())))
	s.handle(mux, "GET /sse", "/sse", true, http.HandlerFunc(s.handleSSE))
	s.handle(mux, "GET /health", "/health", false, http.HandlerFunc(s.handleHealth))
	s.handle(mux, "GET /livez", "/livez", false, http.HandlerFunc(s.handleLive))
	s.handle(mux, "GET /readyz", "/readyz", false, http.HandlerFunc(s.handleReady))
	s.handle(mux, "GET /metrics", "/metrics", false, s.metrics.Handler())
	return securityHeaders(mux)
}

// handle registers h under pattern with metrics and, optionally, tracing.
// path is the low-cardinality label used in metrics and span names.
func (s *Server) handle(mux *http.ServeMux, pattern, path string, traced bool, h http.Handler) {
	h = s.instrument(path, h)
	if traced {
		h = otelhttp.NewHandler(h, path,
			otelhttp.WithTracerProvider(s.tp),
			otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
				return r.Method + " " + path
			}),
		)
	}
	mux.Handle(pattern, h)
}

// Serve accepts connections on ln until ctx is canceled, then shuts down
// gracefully: readiness turns false, the server waits ShutdownDelay so load
// balancers stop sending traffic, SSE streams are closed and in-flight
// requests get up to ShutdownTimeout to finish.
func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	srv := &http.Server{
		Handler:           s.handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		ErrorLog:          slog.NewLogLogger(s.log.Handler(), slog.LevelWarn),
	}

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(ln) }()
	s.SetReady(true)
	s.log.InfoContext(ctx, "http server listening", "addr", ln.Addr().String())

	select {
	case err := <-errCh:
		s.SetReady(false)
		s.stopStreams()
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
	}

	s.log.InfoContext(ctx, "shutdown started", "delay", s.cfg.ShutdownDelay.String(), "timeout", s.cfg.ShutdownTimeout.String())
	s.SetReady(false)
	if s.cfg.ShutdownDelay > 0 {
		time.Sleep(s.cfg.ShutdownDelay)
	}
	s.stopStreams()

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.cfg.ShutdownTimeout)
	defer cancel()
	err := srv.Shutdown(shutdownCtx)
	if serveErr := <-errCh; !errors.Is(serveErr, http.ErrServerClosed) {
		err = errors.Join(err, serveErr)
	}
	if err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	s.log.InfoContext(ctx, "shutdown completed")
	return nil
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	var buf bytes.Buffer
	data := struct{ Version string }{Version: s.cfg.ServiceVersion}
	if err := s.tmpl.Execute(&buf, data); err != nil {
		s.log.ErrorContext(r.Context(), "render index template", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if _, err := buf.WriteTo(w); err != nil {
		s.log.DebugContext(r.Context(), "write index response", "error", err)
	}
}
