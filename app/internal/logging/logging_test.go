package logging

import (
	"bufio"
	"bytes"
	"context"
	json "encoding/json/v2"
	"log/slog"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

func decodeLines(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var out []map[string]any
	sc := bufio.NewScanner(buf)
	for sc.Scan() {
		var m map[string]any
		if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
			t.Fatalf("log line is not JSON: %q: %v", sc.Text(), err)
		}
		out = append(out, m)
	}
	return out
}

func TestNewWritesJSONWithServiceAttrs(t *testing.T) {
	var buf bytes.Buffer
	log := New(&buf, Options{ServiceName: "go-sse", ServiceVersion: "1.0.0"})
	log.Info("hello", "key", "value")

	lines := decodeLines(t, &buf)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	got := lines[0]
	for k, want := range map[string]any{
		"level": "INFO", "msg": "hello", "key": "value", "service": "go-sse", "version": "1.0.0",
	} {
		if got[k] != want {
			t.Errorf("%s = %v, want %v", k, got[k], want)
		}
	}
	if _, ok := got["time"]; !ok {
		t.Error("missing time field")
	}
}

func TestNewDefaultLevelIsInfo(t *testing.T) {
	var buf bytes.Buffer
	log := New(&buf, Options{})
	log.Debug("hidden")
	log.Info("shown")
	lines := decodeLines(t, &buf)
	if len(lines) != 1 || lines[0]["msg"] != "shown" {
		t.Fatalf("got %v, want only the INFO record", lines)
	}
	if _, ok := lines[0]["service"]; ok {
		t.Error("service attribute must be omitted when ServiceName is empty")
	}
}

func TestNewRespectsLevel(t *testing.T) {
	var buf bytes.Buffer
	log := New(&buf, Options{Level: slog.LevelWarn})
	log.Info("hidden")
	log.Warn("shown")
	if lines := decodeLines(t, &buf); len(lines) != 1 || lines[0]["level"] != "WARN" {
		t.Fatalf("got %v, want only the WARN record", lines)
	}
}

func TestTraceCorrelation(t *testing.T) {
	var buf bytes.Buffer
	log := New(&buf, Options{}).With("component", "test").WithGroup("g")

	traceID, _ := trace.TraceIDFromHex("0102030405060708090a0b0c0d0e0f10")
	spanID, _ := trace.SpanIDFromHex("0102030405060708")
	ctx := trace.ContextWithSpanContext(context.Background(), trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: traceID, SpanID: spanID, TraceFlags: trace.FlagsSampled,
	}))

	log.InfoContext(ctx, "with span", "a", 1)
	log.InfoContext(context.Background(), "without span")

	lines := decodeLines(t, &buf)
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}
	g, _ := lines[0]["g"].(map[string]any)
	if g["trace_id"] != traceID.String() || g["span_id"] != spanID.String() {
		t.Errorf("first record = %v, want trace_id/span_id inside group", lines[0])
	}
	if lines[0]["component"] != "test" {
		t.Errorf("component attribute lost: %v", lines[0])
	}
	if _, ok := lines[1]["g"]; ok {
		t.Errorf("record without span must not carry trace attributes: %v", lines[1])
	}
}

// recordingHandler stores records it receives.
type recordingHandler struct {
	records *[]slog.Record
	attrs   []slog.Attr
	group   string
}

func (h recordingHandler) Enabled(context.Context, slog.Level) bool { return true }
func (h recordingHandler) Handle(_ context.Context, r slog.Record) error {
	*h.records = append(*h.records, r)
	return nil
}
func (h recordingHandler) WithAttrs(a []slog.Attr) slog.Handler {
	h.attrs = append(h.attrs, a...)
	return h
}
func (h recordingHandler) WithGroup(name string) slog.Handler {
	h.group = name
	return h
}

func TestExtraHandlersAreLevelFiltered(t *testing.T) {
	var buf bytes.Buffer
	var recs []slog.Record
	log := New(&buf, Options{Level: slog.LevelInfo, Extra: []slog.Handler{recordingHandler{records: &recs}}})
	log = log.With("k", "v").WithGroup("grp")

	log.Debug("dropped")
	log.Info("kept")
	log.Error("kept too")

	if len(recs) != 2 {
		t.Fatalf("extra handler got %d records, want 2", len(recs))
	}
	if n := len(decodeLines(t, &buf)); n != 2 {
		t.Fatalf("json handler got %d records, want 2", n)
	}
}
