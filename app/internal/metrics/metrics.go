// Package metrics defines the Prometheus metrics exposed on /metrics.
package metrics

import (
	"context"
	"net/http"
	"runtime"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel/trace"
)

// Metrics groups every collector exposed by the application.
type Metrics struct {
	registry *prometheus.Registry

	// HTTPRequests counts requests by route path and method.
	HTTPRequests *prometheus.CounterVec
	// HTTPDuration observes request latency by route path, method and status code.
	HTTPDuration *prometheus.HistogramVec
	// ActiveConnections is the number of open SSE streams.
	ActiveConnections *prometheus.GaugeVec
	// EventsSent counts SSE events by type.
	EventsSent *prometheus.CounterVec
	// EventDuration observes how long it takes to encode and flush one event.
	EventDuration *prometheus.HistogramVec
	// SessionDuration observes how long SSE clients stay connected.
	SessionDuration prometheus.Histogram
}

// New creates the metrics and registers them, together with the Go runtime
// and process collectors, in a dedicated registry.
func New(version string) *Metrics {
	m := &Metrics{
		registry: prometheus.NewRegistry(),
		HTTPRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "sse_http_requests_total",
			Help: "Total number of HTTP requests",
		}, []string{"path", "method"}),
		HTTPDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "sse_http_request_duration_seconds",
			Help:    "Duration of HTTP requests in seconds (SSE streams excluded)",
			Buckets: prometheus.DefBuckets,
		}, []string{"path", "method", "code"}),
		ActiveConnections: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "sse_active_connections",
			Help: "Number of active SSE connections",
		}, []string{"endpoint"}),
		EventsSent: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "sse_events_sent_total",
			Help: "Total number of events sent via SSE",
		}, []string{"event_type"}),
		EventDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "sse_event_duration_seconds",
			Help:    "Duration of SSE event processing (encode + write + flush) in seconds",
			Buckets: []float64{.00001, .00005, .0001, .00025, .0005, .001, .0025, .005, .01, .025, .05, .1},
		}, []string{"event_type"}),
		SessionDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "sse_session_duration_seconds",
			Help:    "Duration of SSE client sessions in seconds",
			Buckets: []float64{1, 5, 15, 30, 60, 120, 300, 600, 1800, 3600},
		}),
	}

	buildInfo := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "sse_build_info",
		Help: "Build information of the go-sse application (always 1)",
	}, []string{"version", "goversion"})
	buildInfo.WithLabelValues(version, runtime.Version()).Set(1)

	m.registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		buildInfo,
		m.HTTPRequests,
		m.HTTPDuration,
		m.ActiveConnections,
		m.EventsSent,
		m.EventDuration,
		m.SessionDuration,
	)
	return m
}

// Registry returns the underlying registry (useful for tests).
func (m *Metrics) Registry() *prometheus.Registry { return m.registry }

// Handler serves the metrics in the Prometheus/OpenMetrics text formats.
// OpenMetrics is required for exemplars (links from metrics to traces).
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
		Registry:          m.registry,
	})
}

// Observe records d in obs, attaching the trace ID of ctx as an exemplar
// when the current span is sampled.
func Observe(ctx context.Context, obs prometheus.Observer, d time.Duration) {
	sc := trace.SpanContextFromContext(ctx)
	if eo, ok := obs.(prometheus.ExemplarObserver); ok && sc.IsSampled() {
		eo.ObserveWithExemplar(d.Seconds(), prometheus.Labels{"trace_id": sc.TraceID().String()})
		return
	}
	obs.Observe(d.Seconds())
}
