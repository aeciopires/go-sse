package app

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

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

func envFrom(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func freePort(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
}

func TestRunVersion(t *testing.T) {
	var out bytes.Buffer
	err := Run(t.Context(), Options{Args: []string{"-version"}, Getenv: envFrom(nil), Stdout: &out, Stderr: io.Discard, Version: "9.9.9"})
	if err != nil || strings.TrimSpace(out.String()) != "9.9.9" {
		t.Errorf("Run(-version) = %v, output %q", err, out.String())
	}
}

func TestRunBadFlag(t *testing.T) {
	err := Run(t.Context(), Options{Args: []string{"-nope"}, Getenv: envFrom(nil), Stdout: io.Discard, Stderr: io.Discard})
	if err == nil {
		t.Error("Run(-nope) error = nil")
	}
}

func TestRunInvalidConfig(t *testing.T) {
	err := Run(t.Context(), Options{Getenv: envFrom(map[string]string{"LOG_LEVEL": "loud"}), Stdout: io.Discard, Stderr: io.Discard})
	if err == nil || !strings.Contains(err.Error(), "LOG_LEVEL") {
		t.Errorf("Run() error = %v, want LOG_LEVEL error", err)
	}
}

func TestRunListenError(t *testing.T) {
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
	err = Run(t.Context(), Options{Getenv: envFrom(map[string]string{"PORT": port}), Stdout: io.Discard, Stderr: io.Discard})
	if err == nil || !strings.Contains(err.Error(), "listen") {
		t.Errorf("Run() error = %v, want listen error", err)
	}
}

func TestRunServesAndStops(t *testing.T) {
	port := freePort(t)
	collector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(collector.Close)

	// The OTLP exporters read the real process environment.
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", collector.URL)

	var out syncBuffer
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, Options{
			Getenv: envFrom(map[string]string{
				"PORT":                        port,
				"OTEL_EXPORTER_OTLP_ENDPOINT": collector.URL,
			}),
			Stdout:  &out,
			Stderr:  io.Discard,
			Version: "test",
		})
	}()

	url := "http://127.0.0.1:" + port + "/livez"
	deadline := time.Now().Add(5 * time.Second)
	for Healthcheck(t.Context(), url) != nil {
		if time.Now().After(deadline) {
			t.Fatal("server did not become healthy")
		}
		time.Sleep(10 * time.Millisecond)
	}

	// The -healthcheck flag probes the same endpoint.
	err := Run(t.Context(), Options{
		Args:   []string{"-healthcheck"},
		Getenv: envFrom(map[string]string{"PORT": port}),
		Stdout: io.Discard, Stderr: io.Discard,
	})
	if err != nil {
		t.Errorf("Run(-healthcheck) error = %v", err)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Run() did not return after cancel")
	}

	logs := out.String()
	for _, want := range []string{`"msg":"starting go-sse"`, `"level":"INFO"`, `"telemetry_enabled":true`, `"msg":"go-sse stopped"`} {
		if !strings.Contains(logs, want) {
			t.Errorf("logs missing %s:\n%s", want, logs)
		}
	}
}

func TestHealthcheck(t *testing.T) {
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	t.Cleanup(ok.Close)
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(bad.Close)

	if err := Healthcheck(t.Context(), ok.URL); err != nil {
		t.Errorf("Healthcheck(ok) = %v", err)
	}
	if err := Healthcheck(t.Context(), bad.URL); err == nil {
		t.Error("Healthcheck(503) = nil, want error")
	}
	if err := Healthcheck(t.Context(), "http://127.0.0.1:1/livez"); err == nil {
		t.Error("Healthcheck(closed port) = nil, want error")
	}
	if err := Healthcheck(t.Context(), "://bad"); err == nil {
		t.Error("Healthcheck(bad url) = nil, want error")
	}
}
