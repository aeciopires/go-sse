// Package config loads the application configuration from environment variables.
package config

import (
	"cmp"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"
)

// Default values used when the corresponding environment variable is not set.
const (
	DefaultPort            = "8080"
	DefaultLogLevel        = slog.LevelInfo
	DefaultEventInterval   = 2 * time.Second
	DefaultShutdownTimeout = 15 * time.Second
	DefaultServiceName     = "go-sse"
)

// Config holds every runtime setting of the application.
type Config struct {
	// Addr is the TCP address the HTTP server listens on (e.g. ":8080").
	Addr string
	// LogLevel is the minimum level written to the logs.
	LogLevel slog.Level
	// EventInterval is the interval between two "update" SSE events.
	EventInterval time.Duration
	// ShutdownDelay is how long the server keeps answering requests after
	// readiness turns false, so load balancers can stop routing traffic.
	ShutdownDelay time.Duration
	// ShutdownTimeout bounds the graceful shutdown of the HTTP server.
	ShutdownTimeout time.Duration
	// ServiceName identifies the service in logs, traces and metrics.
	ServiceName string
	// ServiceVersion is the application version (set at build time).
	ServiceVersion string
	// TelemetryEnabled reports whether traces and logs are exported via OTLP.
	TelemetryEnabled bool
}

// Load builds a Config from environment variables read through getenv.
//
// Supported variables:
//
//	PORT                         HTTP port (default 8080)
//	LOG_LEVEL                    DEBUG, INFO, WARN or ERROR (default INFO)
//	SSE_EVENT_INTERVAL           Go duration between updates (default 2s)
//	SHUTDOWN_DELAY               Go duration (default 0s)
//	SHUTDOWN_TIMEOUT             Go duration (default 15s)
//	OTEL_SERVICE_NAME            service name (default go-sse)
//	OTEL_SDK_DISABLED            "true" disables OTLP export
//	OTEL_EXPORTER_OTLP_ENDPOINT  enables OTLP export when set (also the
//	                             signal-specific *_TRACES_ENDPOINT/*_LOGS_ENDPOINT)
func Load(getenv func(string) string, version string) (Config, error) {
	cfg := Config{
		Addr:           ":" + cmp.Or(strings.TrimSpace(getenv("PORT")), DefaultPort),
		ServiceName:    cmp.Or(strings.TrimSpace(getenv("OTEL_SERVICE_NAME")), DefaultServiceName),
		ServiceVersion: cmp.Or(version, "dev"),
	}

	var errs []error

	level, err := ParseLevel(getenv("LOG_LEVEL"))
	errs = append(errs, err)
	cfg.LogLevel = level

	cfg.EventInterval, err = parseDuration(getenv, "SSE_EVENT_INTERVAL", DefaultEventInterval)
	errs = append(errs, err)
	if err == nil && cfg.EventInterval <= 0 {
		errs = append(errs, fmt.Errorf("SSE_EVENT_INTERVAL must be positive, got %s", cfg.EventInterval))
	}

	cfg.ShutdownDelay, err = parseDuration(getenv, "SHUTDOWN_DELAY", 0)
	errs = append(errs, err)

	cfg.ShutdownTimeout, err = parseDuration(getenv, "SHUTDOWN_TIMEOUT", DefaultShutdownTimeout)
	errs = append(errs, err)

	port, err := strconv.Atoi(strings.TrimPrefix(cfg.Addr, ":"))
	if err != nil || port < 0 || port > 65535 {
		errs = append(errs, fmt.Errorf("invalid PORT %q", strings.TrimPrefix(cfg.Addr, ":")))
	}

	disabled, _ := strconv.ParseBool(getenv("OTEL_SDK_DISABLED"))
	hasEndpoint := cmp.Or(
		getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
		getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT"),
		getenv("OTEL_EXPORTER_OTLP_LOGS_ENDPOINT"),
	) != ""
	cfg.TelemetryEnabled = hasEndpoint && !disabled

	return cfg, errors.Join(errs...)
}

// ParseLevel converts a textual log level (case-insensitive) into a slog.Level.
// An empty string yields the default level (INFO).
func ParseLevel(s string) (slog.Level, error) {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "":
		return DefaultLogLevel, nil
	case "DEBUG":
		return slog.LevelDebug, nil
	case "INFO":
		return slog.LevelInfo, nil
	case "WARN", "WARNING":
		return slog.LevelWarn, nil
	case "ERROR":
		return slog.LevelError, nil
	default:
		return DefaultLogLevel, fmt.Errorf("invalid LOG_LEVEL %q (use DEBUG, INFO, WARN or ERROR)", s)
	}
}

func parseDuration(getenv func(string) string, key string, def time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(getenv(key))
	if raw == "" {
		return def, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return def, fmt.Errorf("invalid %s %q: %w", key, raw, err)
	}
	if d < 0 {
		return def, fmt.Errorf("%s must not be negative, got %s", key, d)
	}
	return d, nil
}
