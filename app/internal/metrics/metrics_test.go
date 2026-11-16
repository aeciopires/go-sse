package metrics

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"go.opentelemetry.io/otel/trace"
)

func TestNewRegistersAllMetrics(t *testing.T) {
	m := New("1.2.3")
	m.HTTPRequests.WithLabelValues("/", "GET").Inc()
	m.HTTPDuration.WithLabelValues("/", "GET", "200").Observe(0.1)
	m.ActiveConnections.WithLabelValues("/sse").Set(3)
	m.EventsSent.WithLabelValues("update").Inc()
	m.EventDuration.WithLabelValues("update").Observe(0.001)
	m.SessionDuration.Observe(10)

	families, err := m.Registry().Gather()
	if err != nil {
		t.Fatalf("Gather() error = %v", err)
	}
	got := map[string]bool{}
	for _, f := range families {
		got[f.GetName()] = true
	}
	for _, name := range []string{
		"sse_http_requests_total", "sse_http_request_duration_seconds", "sse_active_connections",
		"sse_events_sent_total", "sse_event_duration_seconds", "sse_session_duration_seconds",
		"sse_build_info", "go_goroutines",
	} {
		if !got[name] {
			t.Errorf("metric %s not registered", name)
		}
	}

	if v := testutil.ToFloat64(m.ActiveConnections.WithLabelValues("/sse")); v != 3 {
		t.Errorf("sse_active_connections = %v, want 3", v)
	}
}

func TestHandlerServesOpenMetricsWithExemplars(t *testing.T) {
	m := New("test")

	traceID, _ := trace.TraceIDFromHex("0102030405060708090a0b0c0d0e0f10")
	spanID, _ := trace.SpanIDFromHex("0102030405060708")
	sampled := trace.ContextWithSpanContext(context.Background(), trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: traceID, SpanID: spanID, TraceFlags: trace.FlagsSampled,
	}))
	Observe(sampled, m.EventDuration.WithLabelValues("update"), 2*time.Millisecond)
	Observe(context.Background(), m.EventDuration.WithLabelValues("welcome"), time.Millisecond)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Set("Accept", "application/openmetrics-text; version=1.0.0")
	rec := httptest.NewRecorder()
	m.Handler().ServeHTTP(rec, req)

	body, _ := io.ReadAll(rec.Body)
	text := string(body)
	if !strings.Contains(text, `trace_id="`+traceID.String()+`"`) {
		t.Errorf("exemplar with trace_id not found in output:\n%s", text)
	}
	if !strings.Contains(text, `sse_event_duration_seconds_count{event_type="welcome"} 1`) {
		t.Errorf("welcome observation not found in output")
	}
}

// plainObserver does not implement prometheus.ExemplarObserver.
type plainObserver struct{ v float64 }

func (p *plainObserver) Observe(v float64) { p.v = v }

func TestObserveWithoutExemplarSupport(t *testing.T) {
	traceID, _ := trace.TraceIDFromHex("0102030405060708090a0b0c0d0e0f10")
	spanID, _ := trace.SpanIDFromHex("0102030405060708")
	ctx := trace.ContextWithSpanContext(context.Background(), trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: traceID, SpanID: spanID, TraceFlags: trace.FlagsSampled,
	}))
	var o plainObserver
	Observe(ctx, &o, 1500*time.Millisecond)
	if o.v != 1.5 {
		t.Errorf("observed %v, want 1.5", o.v)
	}
}
