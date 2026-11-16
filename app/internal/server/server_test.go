package server

import (
	"bufio"
	"bytes"
	"context"
	json "encoding/json/v2"
	"errors"
	"html/template"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/aeciopires/go-sse/app/internal/config"
	"github.com/aeciopires/go-sse/app/internal/metrics"
)

// syncBuffer is a goroutine-safe bytes.Buffer for capturing logs.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

type testEnv struct {
	srv      *Server
	logs     *syncBuffer
	recorder *tracetest.SpanRecorder
}

func newTestServer(t *testing.T, mutate ...func(*config.Config)) testEnv {
	t.Helper()
	cfg := config.Config{
		Addr:            ":0",
		LogLevel:        slog.LevelDebug,
		EventInterval:   10 * time.Millisecond,
		ShutdownTimeout: 2 * time.Second,
		ServiceName:     "go-sse",
		ServiceVersion:  "test",
	}
	for _, m := range mutate {
		m(&cfg)
	}
	logs := &syncBuffer{}
	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	srv, err := New(Options{
		Config:         cfg,
		Logger:         slog.New(slog.NewJSONHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug})),
		Metrics:        metrics.New("test"),
		TracerProvider: tp,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return testEnv{srv: srv, logs: logs, recorder: rec}
}

func get(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestIndex(t *testing.T) {
	env := newTestServer(t)
	rec := get(t, env.srv.Handler(), "/")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET / status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q", ct)
	}
	body := rec.Body.String()
	for _, want := range []string{"go-sse", `data-theme="dark"`, "/static/app.js", "v test"} {
		if !strings.Contains(body, want) {
			t.Errorf("index body missing %q", want)
		}
	}
	for _, h := range []string{"X-Content-Type-Options", "X-Frame-Options", "Content-Security-Policy", "Referrer-Policy"} {
		if rec.Header().Get(h) == "" {
			t.Errorf("missing security header %s", h)
		}
	}
}

func TestIndexTemplateError(t *testing.T) {
	env := newTestServer(t)
	env.srv.tmpl = template.Must(template.New("broken").Parse(`{{.Missing}}`))
	rec := get(t, env.srv.Handler(), "/")
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	if !strings.Contains(env.logs.String(), "render index template") {
		t.Error("template error was not logged")
	}
}

func TestUnknownPathIs404(t *testing.T) {
	env := newTestServer(t)
	if rec := get(t, env.srv.Handler(), "/does-not-exist"); rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
	if rec := httptest.NewRecorder(); true {
		env.srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/sse", nil))
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("POST /sse status = %d, want 405", rec.Code)
		}
	}
}

func TestStaticAssets(t *testing.T) {
	env := newTestServer(t)
	for _, p := range []string{"/static/app.js", "/static/style.css", "/static/theme.js", "/static/favicon.svg"} {
		if rec := get(t, env.srv.Handler(), p); rec.Code != http.StatusOK {
			t.Errorf("GET %s status = %d", p, rec.Code)
		}
	}
}

func TestHealthEndpoints(t *testing.T) {
	env := newTestServer(t)
	h := env.srv.Handler()

	decode := func(rec *httptest.ResponseRecorder) map[string]any {
		t.Helper()
		var m map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
			t.Fatalf("invalid JSON %q: %v", rec.Body.String(), err)
		}
		return m
	}

	rec := get(t, h, "/health")
	body := decode(rec)
	if rec.Code != http.StatusOK || body["status"] != "ok" || body["service"] != "go-sse" || body["version"] != "test" {
		t.Errorf("/health = %d %v", rec.Code, body)
	}
	if rec.Header().Get("Content-Type") != "application/json" {
		t.Errorf("/health Content-Type = %q", rec.Header().Get("Content-Type"))
	}

	if rec := get(t, h, "/livez"); rec.Code != http.StatusOK || decode(rec)["status"] != "ok" {
		t.Errorf("/livez = %d %s", rec.Code, rec.Body)
	}

	if rec := get(t, h, "/readyz"); rec.Code != http.StatusServiceUnavailable || decode(rec)["status"] != "not_ready" {
		t.Errorf("/readyz before ready = %d %s", rec.Code, rec.Body)
	}
	env.srv.SetReady(true)
	if rec := get(t, h, "/readyz"); rec.Code != http.StatusOK || decode(rec)["status"] != "ready" {
		t.Errorf("/readyz when ready = %d %s", rec.Code, rec.Body)
	}
}

func TestMetricsEndpointAndRequestCounting(t *testing.T) {
	env := newTestServer(t)
	h := env.srv.Handler()
	get(t, h, "/health")
	get(t, h, "/health")

	if v := testutil.ToFloat64(env.srv.metrics.HTTPRequests.WithLabelValues("/health", "GET")); v != 2 {
		t.Errorf("sse_http_requests_total{path=/health} = %v, want 2", v)
	}
	rec := get(t, h, "/metrics")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `sse_http_request_duration_seconds_count{code="200",method="GET",path="/health"} 2`) {
		t.Errorf("/metrics missing request duration, got:\n%s", rec.Body.String())
	}
}

// sseEvent is a parsed text/event-stream message.
type sseEvent struct {
	id, event, data, retry string
}

func readEvent(t *testing.T, r *bufio.Reader) sseEvent {
	t.Helper()
	var e sseEvent
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			t.Fatalf("read stream: %v", err)
		}
		line = strings.TrimSuffix(line, "\n")
		if line == "" {
			return e
		}
		k, v, _ := strings.Cut(line, ": ")
		switch k {
		case "id":
			e.id = v
		case "event":
			e.event = v
		case "data":
			e.data = v
		case "retry":
			e.retry = v
		}
	}
}

func TestSSEStream(t *testing.T) {
	env := newTestServer(t)
	ts := httptest.NewServer(env.srv.Handler())
	t.Cleanup(ts.Close)

	ctx, cancel := context.WithCancel(t.Context())
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/sse", nil)
	req.Header.Set("Last-Event-ID", "41")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /sse: %v", err)
	}
	defer resp.Body.Close()

	for k, want := range map[string]string{
		"Content-Type":      "text/event-stream",
		"Cache-Control":     "no-cache",
		"X-Accel-Buffering": "no",
	} {
		if got := resp.Header.Get(k); got != want {
			t.Errorf("header %s = %q, want %q", k, got, want)
		}
	}

	r := bufio.NewReader(resp.Body)
	welcome := readEvent(t, r)
	if welcome.event != "welcome" || welcome.id != "42" || welcome.retry != "3000" {
		t.Errorf("welcome = %+v, want event=welcome id=42 retry=3000", welcome)
	}
	var wp welcomePayload
	if err := json.Unmarshal([]byte(welcome.data), &wp); err != nil || wp.ClientID == "" || wp.IntervalMS != 10 {
		t.Errorf("welcome payload = %+v (err %v)", wp, err)
	}

	for i := range 3 {
		ev := readEvent(t, r)
		var up updatePayload
		if err := json.Unmarshal([]byte(ev.data), &up); err != nil {
			t.Fatalf("update payload: %v", err)
		}
		wantID := uint64(43 + i)
		if ev.event != "update" || up.EventID != wantID || up.Connections != 1 || up.ClientID != wp.ClientID {
			t.Errorf("update %d = %+v / %+v", i, ev, up)
		}
		if up.CPUUsage != 25+(i+1)%50 || up.MemoryUsage != 40+(i+1)%40 {
			t.Errorf("update %d simulated values = %d/%d", i, up.CPUUsage, up.MemoryUsage)
		}
	}

	if v := testutil.ToFloat64(env.srv.metrics.ActiveConnections.WithLabelValues("/sse")); v != 1 {
		t.Errorf("active connections gauge = %v, want 1", v)
	}

	cancel()
	waitFor(t, func() bool { return env.srv.active.Load() == 0 })
	waitFor(t, func() bool { return strings.Contains(env.logs.String(), "client disconnected") })

	if v := testutil.ToFloat64(env.srv.metrics.EventsSent.WithLabelValues("welcome")); v != 1 {
		t.Errorf("welcome events = %v, want 1", v)
	}
	if v := testutil.ToFloat64(env.srv.metrics.EventsSent.WithLabelValues("update")); v < 3 {
		t.Errorf("update events = %v, want >= 3", v)
	}
	if n := testutil.CollectAndCount(env.srv.metrics.SessionDuration); n != 1 {
		t.Errorf("session histogram series = %d", n)
	}

	var sendSpans, httpSpans int
	for _, s := range env.recorder.Ended() {
		switch {
		case strings.HasPrefix(s.Name(), "sse.send"):
			sendSpans++
			if len(s.Links()) != 1 {
				t.Errorf("span %s has %d links, want 1", s.Name(), len(s.Links()))
			}
		case s.Name() == "GET /sse":
			httpSpans++
		}
	}
	if sendSpans < 4 || httpSpans != 1 {
		t.Errorf("spans: sse.send=%d GET /sse=%d", sendSpans, httpSpans)
	}
	if !strings.Contains(env.logs.String(), `"trace_id"`) && !strings.Contains(env.logs.String(), `"msg":"sse event sent"`) {
		t.Error("debug event logs not written")
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition not met before deadline")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// noFlushWriter does not implement http.Flusher.
type noFlushWriter struct {
	header http.Header
	code   int
	body   bytes.Buffer
}

func (w *noFlushWriter) Header() http.Header         { return w.header }
func (w *noFlushWriter) Write(b []byte) (int, error) { return w.body.Write(b) }
func (w *noFlushWriter) WriteHeader(code int)        { w.code = code }

func TestSSEStreamingNotSupported(t *testing.T) {
	env := newTestServer(t)
	w := &noFlushWriter{header: http.Header{}}
	env.srv.handleSSE(w, httptest.NewRequest(http.MethodGet, "/sse", nil))
	if w.code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.code)
	}
}

// failingWriter flushes fine but fails every Write after the first n bytes.
type failingWriter struct {
	*httptest.ResponseRecorder
	writes int
	failAt int
}

func (w *failingWriter) Write(b []byte) (int, error) {
	w.writes++
	if w.writes >= w.failAt {
		return 0, errors.New("broken pipe")
	}
	return w.ResponseRecorder.Write(b)
}

func TestSSEWriteFailures(t *testing.T) {
	for name, failAt := range map[string]int{"welcome": 1, "update": 2} {
		t.Run(name, func(t *testing.T) {
			env := newTestServer(t)
			w := &failingWriter{ResponseRecorder: httptest.NewRecorder(), failAt: failAt}
			done := make(chan struct{})
			go func() {
				env.srv.handleSSE(w, httptest.NewRequest(http.MethodGet, "/sse", nil))
				close(done)
			}()
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				t.Fatal("handler did not return after write failure")
			}
			if !strings.Contains(env.logs.String(), "sse write failed") {
				t.Error("write failure not logged")
			}
			if env.srv.active.Load() != 0 {
				t.Error("active connection counter leaked")
			}
		})
	}
}

func TestWriteEvent(t *testing.T) {
	var buf bytes.Buffer
	err := writeEvent(&buf, Event{ID: 7, Name: "update", Data: map[string]int{"a": 1}, Retry: 1500 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	want := "retry: 1500\nid: 7\nevent: update\ndata: {\"a\":1}\n\n"
	if buf.String() != want {
		t.Errorf("writeEvent() = %q, want %q", buf.String(), want)
	}

	if err := writeEvent(&buf, Event{Data: make(chan int)}); err == nil {
		t.Error("writeEvent() with unencodable data: error = nil")
	}
}

func TestParseLastEventID(t *testing.T) {
	tests := []struct {
		in   string
		want uint64
	}{{"", 0}, {"10", 10}, {"\t5 ", 5}, {"abc", 0}, {"-1", 0}}
	for _, tt := range tests {
		if got := parseLastEventID(tt.in); got != tt.want {
			t.Errorf("parseLastEventID(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestServeGracefulShutdownClosesStreams(t *testing.T) {
	env := newTestServer(t, func(c *config.Config) { c.ShutdownDelay = 20 * time.Millisecond })
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	serveErr := make(chan error, 1)
	go func() { serveErr <- env.srv.Serve(ctx, ln) }()

	// No keep-alive: a pooled-but-unused connection stays in StateNew, which
	// http.Server.Shutdown only treats as idle after 5s.
	client := &http.Client{Transport: &http.Transport{DisableKeepAlives: true}}
	base := "http://" + ln.Addr().String()
	waitFor(t, func() bool {
		resp, err := client.Get(base + "/readyz")
		if err != nil {
			return false
		}
		resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	})

	resp, err := client.Get(base + "/sse")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	readEvent(t, bufio.NewReader(resp.Body))

	cancel()
	select {
	case err := <-serveErr:
		if err != nil {
			t.Fatalf("Serve() error = %v\nlogs:\n%s", err, env.logs.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve() did not return after shutdown")
	}
	if _, err := io.ReadAll(resp.Body); err != nil {
		t.Logf("stream ended with: %v", err)
	}
	logs := env.logs.String()
	for _, want := range []string{"shutdown started", "server shutting down", "shutdown completed"} {
		if !strings.Contains(logs, want) {
			t.Errorf("logs missing %q", want)
		}
	}
	if env.srv.ready.Load() {
		t.Error("server still reports ready after shutdown")
	}
}

func TestServeListenerError(t *testing.T) {
	env := newTestServer(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ln.Close() // Serve fails immediately on a closed listener
	if err := env.srv.Serve(t.Context(), ln); err == nil {
		t.Error("Serve() error = nil, want error")
	}
}

func TestServeShutdownTimeout(t *testing.T) {
	env := newTestServer(t, func(c *config.Config) { c.ShutdownTimeout = time.Nanosecond })
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	// A connection that never sends a full request keeps the server busy.
	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(t.Context())
	serveErr := make(chan error, 1)
	go func() { serveErr <- env.srv.Serve(ctx, ln) }()
	if _, err := conn.Write([]byte("GET / HTTP/1.1\r\n")); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	cancel()
	if err := <-serveErr; err == nil {
		t.Error("Serve() error = nil, want shutdown timeout error")
	}
}

func TestNoopTracerProvider(t *testing.T) {
	_, err := New(Options{
		Config:         config.Config{EventInterval: time.Second},
		Logger:         slog.New(slog.DiscardHandler),
		Metrics:        metrics.New("x"),
		TracerProvider: noop.NewTracerProvider(),
	})
	if err != nil {
		t.Fatal(err)
	}
}
