package telemetry

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestSetupDisabled(t *testing.T) {
	tel, err := Setup(t.Context(), Options{Enabled: false})
	if err != nil {
		t.Fatalf("Setup() error = %v", err)
	}
	if tel.LogHandler != nil {
		t.Error("LogHandler must be nil when telemetry is disabled")
	}
	_, span := tel.TracerProvider.Tracer("test").Start(t.Context(), "span")
	if span.SpanContext().IsValid() {
		t.Error("no-op tracer must not produce valid spans")
	}
	span.End()
	if err := tel.Shutdown(t.Context()); err != nil {
		t.Errorf("Shutdown() error = %v", err)
	}
}

func TestSetupEnabledExportsTracesAndLogs(t *testing.T) {
	var (
		mu    sync.Mutex
		paths = map[string]int{}
	)
	collector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths[r.URL.Path]++
		mu.Unlock()
		w.Header().Set("Content-Type", "application/x-protobuf")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(collector.Close)

	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", collector.URL)
	t.Setenv("OTEL_EXPORTER_OTLP_INSECURE", "true")

	tel, err := Setup(t.Context(), Options{Enabled: true, ServiceName: "go-sse", ServiceVersion: "test"})
	if err != nil {
		t.Fatalf("Setup() error = %v", err)
	}
	if tel.LogHandler == nil {
		t.Fatal("LogHandler must be set when telemetry is enabled")
	}

	ctx, span := tel.TracerProvider.Tracer("test").Start(t.Context(), "span")
	if !span.SpanContext().IsValid() {
		t.Error("enabled tracer must produce valid spans")
	}
	slog.New(tel.LogHandler).InfoContext(ctx, "exported")
	span.End()

	if err := tel.Shutdown(context.WithoutCancel(t.Context())); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if paths["/v1/traces"] == 0 {
		t.Errorf("no trace export received, paths = %v", paths)
	}
	if paths["/v1/logs"] == 0 {
		t.Errorf("no log export received, paths = %v", paths)
	}
}

func TestSetupExporterErrors(t *testing.T) {
	boom := errors.New("boom")
	origTrace, origLog := newTraceExporter, newLogExporter
	t.Cleanup(func() { newTraceExporter, newLogExporter = origTrace, origLog })

	newTraceExporter = func(context.Context) (sdktrace.SpanExporter, error) { return nil, boom }
	if _, err := Setup(t.Context(), Options{Enabled: true, ServiceName: "x"}); !errors.Is(err, boom) {
		t.Errorf("Setup() error = %v, want %v", err, boom)
	}

	newTraceExporter = origTrace
	newLogExporter = func(context.Context) (sdklog.Exporter, error) { return nil, boom }
	if _, err := Setup(t.Context(), Options{Enabled: true, ServiceName: "x"}); !errors.Is(err, boom) {
		t.Errorf("Setup() error = %v, want %v", err, boom)
	}
}
