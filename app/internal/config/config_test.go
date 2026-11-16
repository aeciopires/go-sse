package config

import (
	"log/slog"
	"testing"
	"time"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load(env(nil), "")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	want := Config{
		Addr:            ":8080",
		LogLevel:        slog.LevelInfo,
		EventInterval:   2 * time.Second,
		ShutdownTimeout: 15 * time.Second,
		ServiceName:     "go-sse",
		ServiceVersion:  "dev",
	}
	if cfg != want {
		t.Errorf("Load() = %+v, want %+v", cfg, want)
	}
}

func TestLoadOverrides(t *testing.T) {
	cfg, err := Load(env(map[string]string{
		"PORT":                        "9090",
		"LOG_LEVEL":                   "debug",
		"SSE_EVENT_INTERVAL":          "500ms",
		"SHUTDOWN_DELAY":              "5s",
		"SHUTDOWN_TIMEOUT":            "30s",
		"OTEL_SERVICE_NAME":           "custom",
		"OTEL_EXPORTER_OTLP_ENDPOINT": "http://collector:4318",
	}), "1.2.3")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	want := Config{
		Addr:             ":9090",
		LogLevel:         slog.LevelDebug,
		EventInterval:    500 * time.Millisecond,
		ShutdownDelay:    5 * time.Second,
		ShutdownTimeout:  30 * time.Second,
		ServiceName:      "custom",
		ServiceVersion:   "1.2.3",
		TelemetryEnabled: true,
	}
	if cfg != want {
		t.Errorf("Load() = %+v, want %+v", cfg, want)
	}
}

func TestLoadTelemetry(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want bool
	}{
		{"no endpoint", nil, false},
		{"generic endpoint", map[string]string{"OTEL_EXPORTER_OTLP_ENDPOINT": "http://x"}, true},
		{"traces endpoint", map[string]string{"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT": "http://x"}, true},
		{"logs endpoint", map[string]string{"OTEL_EXPORTER_OTLP_LOGS_ENDPOINT": "http://x"}, true},
		{"disabled", map[string]string{"OTEL_EXPORTER_OTLP_ENDPOINT": "http://x", "OTEL_SDK_DISABLED": "true"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := Load(env(tt.env), "v")
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if cfg.TelemetryEnabled != tt.want {
				t.Errorf("TelemetryEnabled = %v, want %v", cfg.TelemetryEnabled, tt.want)
			}
		})
	}
}

func TestLoadInvalid(t *testing.T) {
	tests := map[string]map[string]string{
		"log level":         {"LOG_LEVEL": "verbose"},
		"interval syntax":   {"SSE_EVENT_INTERVAL": "soon"},
		"interval zero":     {"SSE_EVENT_INTERVAL": "0s"},
		"negative delay":    {"SHUTDOWN_DELAY": "-1s"},
		"timeout syntax":    {"SHUTDOWN_TIMEOUT": "x"},
		"port not a number": {"PORT": "http"},
		"port out of range": {"PORT": "70000"},
	}
	for name, e := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := Load(env(e), "v"); err == nil {
				t.Error("Load() error = nil, want error")
			}
		})
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		in      string
		want    slog.Level
		wantErr bool
	}{
		{"", slog.LevelInfo, false},
		{"DEBUG", slog.LevelDebug, false},
		{" info ", slog.LevelInfo, false},
		{"Warn", slog.LevelWarn, false},
		{"WARNING", slog.LevelWarn, false},
		{"error", slog.LevelError, false},
		{"trace", slog.LevelInfo, true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := ParseLevel(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseLevel(%q) error = %v, wantErr %v", tt.in, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("ParseLevel(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
